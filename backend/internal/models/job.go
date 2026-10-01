package models

import "time"

type JobType string

const (
	JobTypeSyncAccount      JobType = "sync_account"
	JobTypeVerifyConnection JobType = "verify_connection"
)

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusRetrying  JobStatus = "retrying"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
)

type JobRun struct {
	ID            uint            `json:"id" gorm:"primaryKey"`
	QueueID       string          `json:"queueId" gorm:"index"`
	AccountID     *uint           `json:"accountId" gorm:"index"`
	Account       *Account        `json:"account,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Type          JobType         `json:"type" gorm:"index;not null"`
	Folder        string          `json:"folder"`
	Status        JobStatus       `json:"status" gorm:"index;not null"`
	Attempts      int             `json:"attempts"`
	Fetched       int             `json:"fetched"`
	Created       int             `json:"created"`
	Updated       int             `json:"updated"`
	Synced        int             `json:"synced"`
	Error         string          `json:"error"`
	ErrorCode     MailErrorCode   `json:"errorCode"`
	ErrorAction   MailErrorAction `json:"errorAction"`
	Logs          string          `json:"logs"`
	StartedAt     *time.Time      `json:"startedAt"`
	NextAttemptAt *time.Time      `json:"nextAttemptAt"`
	EndedAt       *time.Time      `json:"endedAt"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type JobSummary struct {
	ID            uint            `json:"id"`
	AccountID     *uint           `json:"accountId"`
	AccountEmail  string          `json:"accountEmail"`
	Type          JobType         `json:"type"`
	Status        JobStatus       `json:"status"`
	Attempts      int             `json:"attempts"`
	Fetched       int             `json:"fetched"`
	Created       int             `json:"created"`
	Updated       int             `json:"updated"`
	Synced        int             `json:"synced"`
	Error         string          `json:"error"`
	ErrorCode     MailErrorCode   `json:"errorCode"`
	ErrorAction   MailErrorAction `json:"errorAction"`
	StartedAt     *time.Time      `json:"startedAt"`
	NextAttemptAt *time.Time      `json:"nextAttemptAt"`
	EndedAt       *time.Time      `json:"endedAt"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}
