package app

import "time"

type microsoftErrorResponse struct {
	Error microsoftErrorDetail `json:"error"`
}

type microsoftErrorDetail struct {
	Code       string                `json:"code"`
	Message    string                `json:"message"`
	InnerError *microsoftErrorDetail `json:"innerError"`
}

type microsoftEmailAddress struct {
	Address string `json:"address"`
}

type microsoftRecipient struct {
	EmailAddress microsoftEmailAddress `json:"emailAddress"`
}

type microsoftMessage struct {
	ID               string               `json:"id"`
	ConversationID   string               `json:"conversationId"`
	Subject          string               `json:"subject"`
	BodyPreview      string               `json:"bodyPreview"`
	ParentFolderID   string               `json:"parentFolderId"`
	IsRead           bool                 `json:"isRead"`
	ReceivedDateTime time.Time            `json:"receivedDateTime"`
	SentDateTime     time.Time            `json:"sentDateTime"`
	From             microsoftRecipient   `json:"from"`
	ToRecipients     []microsoftRecipient `json:"toRecipients"`
}

type microsoftMessageListResponse struct {
	Value    []microsoftMessage `json:"value"`
	NextLink string             `json:"@odata.nextLink"`
}

type microsoftFolder struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type microsoftFoldersResponse struct {
	Value []microsoftFolder `json:"value"`
}
