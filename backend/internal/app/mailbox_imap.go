package app

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"deezmails/internal/models"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func (a *App) imapMessages(ctx context.Context, account models.Account, selectedFolder string) ([]models.Email, error) {
	client, err := imapSession(ctx, account)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	folders := []string{selectedFolder}
	if selectedFolder == folderAll {
		folders, err = a.imapFolderNames(client)
		if err != nil {
			return nil, err
		}
	}
	result := make([]models.Email, 0, 100)
	for _, folder := range folders {
		if len(result) >= a.config.SyncMaxMessages {
			break
		}
		mailbox, err := client.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait()
		if err != nil {
			return nil, fmt.Errorf("select IMAP folder %q: %w", folder, err)
		}
		if mailbox.NumMessages == 0 {
			continue
		}
		remaining := uint32(a.config.SyncMaxMessages - len(result))
		if remaining > 100 {
			remaining = 100
		}
		first := uint32(1)
		if mailbox.NumMessages > remaining {
			first = mailbox.NumMessages - remaining + 1
		}
		set := imap.SeqSet{}
		set.AddRange(first, mailbox.NumMessages)
		section := &imap.FetchItemBodySection{Peek: true, Partial: &imap.SectionPartial{Size: a.config.MaxMessageBytes + 1}}
		command := client.Fetch(set, &imap.FetchOptions{UID: true, Envelope: true, Flags: true, BodySection: []*imap.FetchItemBodySection{section}})
		var readErr error
		for message := command.Next(); message != nil; message = command.Next() {
			var uid imap.UID
			var envelope *imap.Envelope
			var flags []imap.Flag
			var body []byte
			bodyFound := false
			messageFailed := false
			for item := message.Next(); item != nil; item = message.Next() {
				switch item := item.(type) {
				case imapclient.FetchItemDataUID:
					uid = item.UID
				case imapclient.FetchItemDataEnvelope:
					envelope = item.Envelope
				case imapclient.FetchItemDataFlags:
					flags = item.Flags
				case imapclient.FetchItemDataBodySection:
					if item.Literal == nil || !item.MatchCommand(section) {
						continue
					}
					body, err = readLimited(item.Literal, a.config.MaxMessageBytes)
					if err != nil {
						readErr, messageFailed = err, true
						continue
					}
					bodyFound = true
				}
			}
			if messageFailed {
				continue
			}
			if !bodyFound || envelope == nil {
				readErr = fmt.Errorf("IMAP message %d omitted required data", uid)
				continue
			}
			email := models.Email{AccountID: account.ID, RemoteID: folder + ":" + fmt.Sprint(uid), Folder: folder, Subject: envelope.Subject, ReceivedAt: envelope.Date, Raw: base64.RawURLEncoding.EncodeToString(body)}
			if len(envelope.From) > 0 {
				email.From = envelope.From[0].Addr()
			}
			to := make([]string, 0, len(envelope.To))
			for _, recipient := range envelope.To {
				if address := recipient.Addr(); address != "" {
					to = append(to, address)
				}
			}
			email.To = strings.Join(to, ", ")
			for _, flag := range flags {
				if flag == imap.FlagSeen {
					email.IsRead = true
					break
				}
			}
			result = append(result, email)
		}
		if err := command.Close(); err != nil {
			return nil, err
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return result, nil
}

func imapSession(ctx context.Context, account models.Account) (*imapclient.Client, error) {
	address := net.JoinHostPort(account.IncomingHost, strconv.Itoa(account.IncomingPort))
	dialer, err := accountDialer(account)
	if err != nil {
		return nil, err
	}
	connection, err := (contextDialer{dialer: dialer}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	if err := connection.SetDeadline(mailDeadline(ctx)); err != nil {
		connection.Close()
		return nil, err
	}
	tlsConfig := &tls.Config{ServerName: account.IncomingHost, MinVersion: tls.VersionTLS12}
	options := &imapclient.Options{TLSConfig: tlsConfig}
	var client *imapclient.Client
	if account.TLSMode == models.TLSModeStartTLS {
		client, err = imapclient.NewStartTLS(connection, options)
	} else {
		if account.TLSMode == models.TLSModeImplicit {
			tlsConnection := tls.Client(connection, tlsConfig)
			if err = tlsConnection.HandshakeContext(ctx); err != nil {
				connection.Close()
				return nil, err
			}
			connection = tlsConnection
		}
		client = imapclient.New(connection, options)
		err = client.WaitGreeting()
	}
	if err != nil {
		connection.Close()
		if account.TLSMode == models.TLSModeStartTLS {
			if protocolError, ok := errors.AsType[*imap.Error](err); ok && protocolError.Code == "" && protocolError.Type != imap.StatusResponseTypeBye {
				return nil, &mailFailure{Code: models.MailErrorTLS, Action: models.MailErrorActionEditAccount, Message: "IMAP server does not accept STARTTLS. Edit the account security mode.", Cause: err}
			}
		}
		return nil, err
	}
	if err := client.Login(account.Email, account.MailboxPassword).Wait(); err != nil {
		client.Close()
		if protocolError, ok := errors.AsType[*imap.Error](err); ok && protocolError.Code == "" {
			return nil, &mailFailure{Code: models.MailErrorAuthentication, Action: models.MailErrorActionEditAccount, Message: "IMAP rejected the mailbox credentials. Edit the account credentials.", Cause: err}
		}
		return nil, err
	}
	return client, nil
}

func (a *App) imapFolderNames(client *imapclient.Client) ([]string, error) {
	command := client.List("", "*", nil)
	folders := make([]string, 0)
	for mailbox := command.Next(); mailbox != nil; mailbox = command.Next() {
		if mailbox.Mailbox != "" && len(folders) < a.config.SyncMaxFolders {
			folders = append(folders, mailbox.Mailbox)
		}
	}
	return folders, command.Close()
}

func (a *App) imapFolders(ctx context.Context, account models.Account) ([]models.Folder, error) {
	client, err := imapSession(ctx, account)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	names, err := a.imapFolderNames(client)
	if err != nil {
		return nil, err
	}
	folders := make([]models.Folder, 0, len(names))
	for _, name := range names {
		folders = append(folders, models.Folder{ID: name, Name: name})
	}
	return folders, nil
}
