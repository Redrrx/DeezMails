package app

import (
	"context"
	"net/url"
	"strings"

	"deezmails/internal/models"

	"golang.org/x/oauth2"
)

func (a *App) microsoftMessages(ctx context.Context, account models.Account, token *oauth2.Token, folder string) ([]models.Email, error) {
	client, err := providerClient(account, token, a.config.MaxMessageBytes)
	if err != nil {
		return nil, err
	}
	endpoint := "https://graph.microsoft.com/v1.0/me/messages"
	if folder != folderAll {
		endpoint = "https://graph.microsoft.com/v1.0/me/mailFolders/" + url.PathEscape(folder) + "/messages"
	}
	params := url.Values{"$top": {"100"}, "$orderby": {"receivedDateTime desc"}, "$select": {"id,conversationId,subject,bodyPreview,isRead,receivedDateTime,sentDateTime,parentFolderId,from,toRecipients"}}
	emails := []models.Email{}
	for page := 0; page < a.config.SyncMaxPages && len(emails) < a.config.SyncMaxMessages; page++ {
		var list microsoftMessageListResponse
		requestURL := endpoint + "?" + params.Encode()
		if page > 0 {
			requestURL = endpoint
		}
		if err := a.microsoftJSON(ctx, client, requestURL, &list); err != nil {
			return nil, err
		}
		for _, m := range list.Value {
			if len(emails) >= a.config.SyncMaxMessages {
				break
			}
			to := make([]string, 0, len(m.ToRecipients))
			for _, r := range m.ToRecipients {
				to = append(to, r.EmailAddress.Address)
			}
			messageFolder := m.ParentFolderID
			if folder != folderAll {
				messageFolder = folder
			}
			receivedAt := m.ReceivedDateTime
			if receivedAt.IsZero() {
				receivedAt = m.SentDateTime
			}
			emails = append(emails, models.Email{AccountID: account.ID, RemoteID: m.ID, Folder: messageFolder, ThreadID: m.ConversationID, Subject: m.Subject, Preview: m.BodyPreview, From: m.From.EmailAddress.Address, To: strings.Join(to, ", "), ReceivedAt: receivedAt, IsRead: m.IsRead})
		}
		if list.NextLink == "" {
			break
		}
		if err := validateProviderAPIURL(models.ProviderMicrosoft, list.NextLink); err != nil {
			return nil, err
		}
		endpoint = list.NextLink
		params = url.Values{}
	}
	return emails, nil
}

func (a *App) microsoftFolders(ctx context.Context, account models.Account, token *oauth2.Token) ([]models.Folder, error) {
	var result microsoftFoldersResponse
	client, err := providerClient(account, token, a.config.MaxMessageBytes)
	if err != nil {
		return nil, err
	}
	err = a.microsoftJSON(ctx, client, "https://graph.microsoft.com/v1.0/me/mailFolders?$top=999&includeHiddenFolders=true", &result)
	if err != nil {
		return nil, err
	}
	folders := make([]models.Folder, 0, len(result.Value))
	for _, folder := range result.Value {
		folders = append(folders, models.Folder{ID: folder.ID, Name: folder.DisplayName})
	}
	return folders, nil
}
