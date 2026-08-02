package main

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
)

func imapMessages(account Account, selectedFolder string) ([]Email, error) {
	client, err := imapSession(account)
	if err != nil {
		return nil, err
	}
	defer client.Logout()
	folders := []string{selectedFolder}
	if selectedFolder == "all" {
		folders, err = imapFolderNames(client)
		if err != nil {
			return nil, err
		}
	}
	result := make([]Email, 0, 100)
	for _, folder := range folders {
		mailbox, err := client.Select(folder, true)
		if err != nil {
			return nil, fmt.Errorf("select IMAP folder %q: %w", folder, err)
		}
		if mailbox.Messages == 0 {
			continue
		}
		first := uint32(1)
		if mailbox.Messages > 100 {
			first = mailbox.Messages - 99
		}
		set := new(imap.SeqSet)
		set.AddRange(first, mailbox.Messages)
		section := &imap.BodySectionName{}
		items := []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchFlags, section.FetchItem()}
		messages := make(chan *imap.Message, 20)
		done := make(chan error, 1)
		go func() { done <- client.Fetch(set, items, messages) }()
		var readErr error
		for message := range messages {
			if readErr != nil {
				continue
			}
			bodyReader := message.GetBody(section)
			if bodyReader == nil {
				readErr = fmt.Errorf("IMAP message %d did not include a body", message.Uid)
				continue
			}
			body, err := readLimited(bodyReader, maxMessageBytes())
			if err != nil {
				readErr = fmt.Errorf("read IMAP message %d: %w", message.Uid, err)
				continue
			}
			envelope := message.Envelope
			if envelope == nil {
				readErr = fmt.Errorf("IMAP message %d did not include an envelope", message.Uid)
				continue
			}
			email := Email{AccountID: account.ID, RemoteID: folder + ":" + fmt.Sprint(message.Uid), Folder: folder, Subject: envelope.Subject, ReceivedAt: envelope.Date, Raw: base64.RawURLEncoding.EncodeToString(body), IsRead: true}
			if len(envelope.From) > 0 {
				email.From = envelope.From[0].Address()
			}
			to := make([]string, 0, len(envelope.To))
			for _, recipient := range envelope.To {
				to = append(to, recipient.Address())
			}
			email.To = strings.Join(to, ", ")
			for _, flag := range message.Flags {
				if flag == imap.SeenFlag {
					email.IsRead = true
					break
				}
				email.IsRead = false
			}
			result = append(result, email)
		}
		if err := <-done; err != nil {
			return nil, err
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return result, nil
}

func imapSession(account Account) (*imapclient.Client, error) {
	address := net.JoinHostPort(account.IncomingHost, strconv.Itoa(account.IncomingPort))
	dialer, err := accountDialer(account)
	if err != nil {
		return nil, err
	}
	connection, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	if err := connection.SetDeadline(time.Now().Add(mailTimeout)); err != nil {
		connection.Close()
		return nil, err
	}
	if account.TLSMode == "implicit_tls" {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: account.IncomingHost, MinVersion: tls.VersionTLS12})
		if err := tlsConnection.Handshake(); err != nil {
			connection.Close()
			return nil, err
		}
		connection = tlsConnection
	}
	client, err := imapclient.New(connection)
	if err != nil {
		connection.Close()
		return nil, err
	}
	if account.TLSMode == "starttls" {
		if err := client.StartTLS(&tls.Config{ServerName: account.IncomingHost, MinVersion: tls.VersionTLS12}); err != nil {
			client.Logout()
			return nil, err
		}
	}
	if err := client.Login(account.Email, account.MailboxPassword); err != nil {
		client.Logout()
		return nil, err
	}
	return client, nil
}

func imapFolderNames(client *imapclient.Client) ([]string, error) {
	mailboxes := make(chan *imap.MailboxInfo, 32)
	done := make(chan error, 1)
	go func() { done <- client.List("", "*", mailboxes) }()
	folders := []string{}
	for mailbox := range mailboxes {
		if mailbox.Name != "" {
			folders = append(folders, mailbox.Name)
		}
	}
	return folders, <-done
}
func imapFolders(account Account) ([]Folder, error) {
	client, err := imapSession(account)
	if err != nil {
		return nil, err
	}
	defer client.Logout()
	names, err := imapFolderNames(client)
	if err != nil {
		return nil, err
	}
	folders := make([]Folder, 0, len(names))
	for _, name := range names {
		folders = append(folders, Folder{ID: name, Name: name})
	}
	return folders, nil
}

func pop3Messages(account Account) ([]Email, error) {
	connection, reader, err := pop3Session(account)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	line, err := pop3Request(connection, reader, "STAT")
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid POP3 STAT response")
	}
	count, _ := strconv.Atoi(parts[1])
	first := 1
	if count > 100 {
		first = count - 99
	}
	result := make([]Email, 0, count-first+1)
	for number := first; number <= count; number++ {
		uid, err := pop3Request(connection, reader, fmt.Sprintf("UIDL %d", number))
		if err != nil {
			return nil, err
		}
		uidParts := strings.Fields(uid)
		if len(uidParts) < 3 {
			continue
		}
		raw, err := pop3Multiline(connection, reader, fmt.Sprintf("RETR %d", number))
		if err != nil {
			return nil, err
		}
		parsed, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			continue
		}
		received, _ := mail.ParseDate(parsed.Header.Get("Date"))
		body, err := readLimited(parsed.Body, maxMessageBytes())
		if err != nil {
			return nil, err
		}
		result = append(result, Email{AccountID: account.ID, RemoteID: uidParts[2], Folder: "INBOX", From: parsed.Header.Get("From"), To: parsed.Header.Get("To"), Subject: parsed.Header.Get("Subject"), Body: string(body), ReceivedAt: received, Raw: base64.RawURLEncoding.EncodeToString([]byte(raw)), IsRead: true})
	}
	_ = pop3Command(connection, reader, "QUIT")
	return result, nil
}

func pop3Verify(account Account) error {
	connection, reader, err := pop3Session(account)
	if err != nil {
		return err
	}
	defer connection.Close()
	defer pop3Command(connection, reader, "QUIT")
	return pop3Command(connection, reader, "NOOP")
}

func pop3Session(account Account) (net.Conn, *bufio.Reader, error) {
	address := net.JoinHostPort(account.IncomingHost, strconv.Itoa(account.IncomingPort))
	dialer, err := accountDialer(account)
	if err != nil {
		return nil, nil, err
	}
	connection, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, nil, err
	}
	if err := connection.SetDeadline(time.Now().Add(mailTimeout)); err != nil {
		connection.Close()
		return nil, nil, err
	}
	if account.TLSMode == "implicit_tls" {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: account.IncomingHost, MinVersion: tls.VersionTLS12})
		if err := tlsConnection.Handshake(); err != nil {
			connection.Close()
			return nil, nil, err
		}
		connection = tlsConnection
	}
	reader := bufio.NewReader(connection)
	if _, err := pop3Line(reader); err != nil {
		connection.Close()
		return nil, nil, err
	}
	if account.TLSMode == "starttls" {
		if _, err := fmt.Fprint(connection, "STLS\r\n"); err != nil {
			connection.Close()
			return nil, nil, err
		}
		if _, err := pop3Line(reader); err != nil {
			connection.Close()
			return nil, nil, err
		}
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: account.IncomingHost, MinVersion: tls.VersionTLS12})
		if err := tlsConnection.Handshake(); err != nil {
			connection.Close()
			return nil, nil, err
		}
		connection = tlsConnection
		reader = bufio.NewReader(connection)
	}
	if err := pop3Command(connection, reader, "USER "+account.Email); err != nil {
		connection.Close()
		return nil, nil, err
	}
	if err := pop3Command(connection, reader, "PASS "+account.MailboxPassword); err != nil {
		connection.Close()
		return nil, nil, err
	}
	return connection, reader, nil
}

func pop3Request(connection net.Conn, reader *bufio.Reader, command string) (string, error) {
	if _, err := fmt.Fprint(connection, command+"\r\n"); err != nil {
		return "", err
	}
	return pop3Line(reader)
}
func pop3Command(connection net.Conn, reader *bufio.Reader, command string) error {
	_, err := pop3Request(connection, reader, command)
	return err
}
func pop3Line(reader *bufio.Reader) (string, error) {
	line, err := pop3ReadLine(reader)
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "+OK") {
		return "", fmt.Errorf("POP3: %s", line)
	}
	return line, nil
}
func pop3Multiline(connection net.Conn, reader *bufio.Reader, command string) (string, error) {
	if _, err := pop3Request(connection, reader, command); err != nil {
		return "", err
	}
	var output strings.Builder
	for {
		line, err := pop3ReadLine(reader)
		if err != nil {
			return "", err
		}
		if line == "." {
			break
		}
		if strings.HasPrefix(line, "..") {
			line = line[1:]
		}
		output.WriteString(line)
		output.WriteString("\r\n")
		if int64(output.Len()) > maxMessageBytes() {
			return "", fmt.Errorf("POP3 message exceeds the %d byte limit", maxMessageBytes())
		}
	}
	return output.String(), nil
}

func pop3ReadLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) > 16<<10 {
		return "", fmt.Errorf("POP3 response line exceeds the 16384 byte limit")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
