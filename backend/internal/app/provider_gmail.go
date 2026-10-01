package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	"deezmails/internal/models"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	folderAll         = "all"
	gmailLabelAll     = "ALL"
	gmailLabelDraft   = "DRAFT"
	gmailLabelInbox   = "INBOX"
	gmailLabelSent    = "SENT"
	gmailLabelSpam    = "SPAM"
	gmailLabelTrash   = "TRASH"
	gmailLabelUnread  = "UNREAD"
	mailHeaderFrom    = "from"
	mailHeaderSubject = "subject"
	mailHeaderTo      = "to"
)

func (a *App) gmailService(ctx context.Context, account models.Account, token *oauth2.Token) (*gmail.Service, error) {
	client, err := providerClient(account, token, a.config.MaxMessageBytes)
	if err != nil {
		return nil, err
	}
	return gmail.NewService(ctx, option.WithHTTPClient(client), option.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
}

func (a *App) gmailMessages(ctx context.Context, account models.Account, token *oauth2.Token, folder string) ([]models.Email, error) {
	service, err := a.gmailService(ctx, account, token)
	if err != nil {
		return nil, err
	}
	emails := []models.Email{}
	pageToken := ""
	for page := 0; page < a.config.SyncMaxPages && len(emails) < a.config.SyncMaxMessages; page++ {
		call := service.Users.Messages.List("me").MaxResults(100).IncludeSpamTrash(true).Fields("messages(id,threadId),nextPageToken").Context(ctx)
		if folder != folderAll {
			call.LabelIds(folder)
		}
		if pageToken != "" {
			call.PageToken(pageToken)
		}
		list, err := call.Do()
		if err != nil {
			return nil, err
		}
		for _, item := range list.Messages {
			if len(emails) >= a.config.SyncMaxMessages {
				break
			}
			message, err := service.Users.Messages.Get("me", item.Id).Format("full").Fields("id,threadId,labelIds,snippet,internalDate,payload/headers").Context(ctx).Do()
			if err != nil {
				return nil, err
			}
			receivedAt := time.UnixMilli(message.InternalDate)
			email := models.Email{AccountID: account.ID, RemoteID: message.Id, Folder: folder, ThreadID: message.ThreadId, Preview: message.Snippet, ReceivedAt: receivedAt, IsRead: true}
			if folder == folderAll {
				email.Folder = gmailFolder(message.LabelIds)
			}
			if message.Payload != nil {
				for _, header := range message.Payload.Headers {
					switch strings.ToLower(header.Name) {
					case mailHeaderFrom:
						email.From = header.Value
					case mailHeaderTo:
						email.To = header.Value
					case mailHeaderSubject:
						email.Subject = header.Value
					}
				}
			}
			for _, label := range message.LabelIds {
				if label == gmailLabelUnread {
					email.IsRead = false
					break
				}
			}
			emails = append(emails, email)
		}
		if list.NextPageToken == "" {
			break
		}
		pageToken = list.NextPageToken
	}
	return emails, nil
}

func gmailFolder(labels []string) string {
	for _, label := range labels {
		switch label {
		case gmailLabelSpam, gmailLabelInbox, gmailLabelSent, gmailLabelDraft, gmailLabelTrash:
			return label
		}
	}
	return gmailLabelAll
}

func (a *App) gmailFolders(ctx context.Context, account models.Account, token *oauth2.Token) ([]models.Folder, error) {
	service, err := a.gmailService(ctx, account, token)
	if err != nil {
		return nil, err
	}
	result, err := service.Users.Labels.List("me").Fields("labels(id,name)").Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	folders := make([]models.Folder, 0, len(result.Labels))
	for _, label := range result.Labels {
		folders = append(folders, models.Folder{ID: label.Id, Name: label.Name})
	}
	return folders, nil
}
