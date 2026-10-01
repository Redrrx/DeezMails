package models

import "time"

type Email struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	AccountID     uint      `json:"accountId" gorm:"index;uniqueIndex:account_folder_remote;not null"`
	RemoteID      string    `json:"remoteId" gorm:"uniqueIndex:account_folder_remote;not null"`
	Folder        string    `json:"folder" gorm:"uniqueIndex:account_folder_remote;index;not null;default:INBOX"`
	ThreadID      string    `json:"threadId"`
	From          string    `json:"from"`
	To            string    `json:"to"`
	Subject       string    `json:"subject"`
	Preview       string    `json:"preview"`
	Body          string    `json:"body"`
	BodyTruncated bool      `json:"bodyTruncated" gorm:"-"`
	BodyFetched   bool      `json:"-" gorm:"not null;default:false"`
	Raw           string    `json:"-"`
	ReceivedAt    time.Time `json:"receivedAt" gorm:"index"`
	IsRead        bool      `json:"isRead"`
	CreatedAt     time.Time `json:"createdAt"`
	Account       *Account  `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
