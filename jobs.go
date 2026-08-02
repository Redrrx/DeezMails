package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/iamolegga/goqite"
	queuejobs "github.com/iamolegga/goqite/jobs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	jobSync   = "sync_account"
	jobVerify = "verify_connection"
)

type JobService struct {
	queue *goqite.Queue
}

type jobPayload struct {
	JobID uint `json:"jobId"`
}

type syncResult struct {
	Fetched int
	Created int
	Updated int
}

func jobMaxAttempts() int {
	return positive(env("JOB_MAX_ATTEMPTS", "3"), 3)
}

func jobRetryDelay(attempt int) time.Duration {
	base := time.Duration(positive(env("JOB_RETRY_BASE_SECONDS", "15"), 15)) * time.Second
	delay := base << max(attempt-1, 0)
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	return delay + time.Duration(time.Now().UnixNano()%1000)*time.Millisecond
}

func jobTimeout() time.Duration {
	return time.Duration(positive(env("JOB_TIMEOUT_MINUTES", "10"), 10)) * time.Minute
}

func (a *App) startJobs() error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	options := goqite.NewOpts{DB: sqlDB, Name: "deezmails", MaxReceive: jobMaxAttempts(), Timeout: jobTimeout(), TablePrefix: "deezmails_"}
	workers := "1"
	if os.Getenv("DATABASE_URL") != "" {
		options.SQLFlavor = goqite.SQLFlavorPostgreSQL
		options.TablePrefix = ""
		workers = "4"
	}
	queue := goqite.New(options)
	if err := queue.Setup(context.Background()); err != nil {
		return err
	}
	a.jobs = &JobService{queue: queue}
	if err := a.recoverQueuedJobs(); err != nil {
		return err
	}
	if err := a.recoverLegacyRetryingJobs(); err != nil {
		return err
	}
	runner := queuejobs.NewRunner(queuejobs.NewRunnerOpts{Queue: queue, Limit: positive(env("WORKER_CONCURRENCY", workers), positive(workers, 1)), PollInterval: time.Second, Extend: time.Minute, Log: slog.Default()})
	runner.Register(jobSync, a.runJob)
	runner.Register(jobVerify, a.runJob)
	go runner.Start(context.Background())
	interval := time.Duration(positive(env("SYNC_INTERVAL_MINUTES", "15"), 15)) * time.Minute
	go a.scheduleJobs(interval)
	return nil
}

func (a *App) scheduleJobs(interval time.Duration) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		a.scheduleDueAccounts(interval)
		<-ticker.C
	}
}

func (a *App) scheduleDueAccounts(interval time.Duration) {
	now := time.Now()
	var accounts []Account
	if err := a.db.Where("enabled = ? AND sync_enabled = ? AND status = ? AND (next_sync_at IS NULL OR next_sync_at <= ?)", true, true, "connected", now).Order("next_sync_at ASC NULLS FIRST").Limit(positive(env("SCHEDULER_BATCH_SIZE", "50"), 50)).Find(&accounts).Error; err != nil {
		slog.Error("load scheduled accounts", "error", err)
		return
	}
	for _, account := range accounts {
		job, err := a.enqueueJob(jobSync, account.ID, "all")
		if err != nil {
			slog.Error("queue scheduled sync", "accountID", account.ID, "error", err)
			continue
		}
		if job.Status != "queued" {
			continue
		}
		next := now.Add(interval + time.Duration(account.ID%30)*time.Second)
		if err := a.db.Model(&Account{}).Where("id = ?", account.ID).Update("next_sync_at", next).Error; err != nil {
			slog.Error("set next sync time", "accountID", account.ID, "error", err)
		}
	}
}

func (a *App) enqueueJob(kind string, accountID uint, folder string) (*JobRun, error) {
	var existing JobRun
	if a.db.Where("account_id = ? AND type = ? AND folder = ? AND status IN ?", accountID, kind, folder, []string{"queued", "running", "retrying"}).First(&existing).Error == nil {
		return &existing, nil
	}
	job := JobRun{AccountID: &accountID, Type: kind, Folder: folder, Status: "queued", Logs: "Queued"}
	if err := a.db.Create(&job).Error; err != nil {
		if a.db.Where("account_id = ? AND type = ? AND folder = ? AND status IN ?", accountID, kind, folder, []string{"queued", "running", "retrying"}).First(&existing).Error == nil {
			return &existing, nil
		}
		return nil, err
	}
	if err := a.pushJob(&job, 0); err != nil {
		now := time.Now()
		job.Status, job.Error, job.EndedAt = "failed", safeError(err), &now
		job.Logs = appendLog(job.Logs, "Queueing failed: "+job.Error)
		if saveErr := a.db.Save(&job).Error; saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}
	return &job, nil
}

func (a *App) pushJob(job *JobRun, delay time.Duration) error {
	payload, err := json.Marshal(jobPayload{JobID: job.ID})
	if err != nil {
		return err
	}
	queueID, err := queuejobs.Create(context.Background(), a.jobs.queue, job.Type, goqite.Message{Body: payload, Delay: delay, Priority: 10})
	if err != nil {
		return err
	}
	job.QueueID = string(queueID)
	if err := a.db.Save(job).Error; err != nil {
		return err
	}
	return nil
}

func (a *App) recoverQueuedJobs() error {
	var jobs []JobRun
	if err := a.db.Where("status = ? AND queue_id = ?", "queued", "").Find(&jobs).Error; err != nil {
		return err
	}
	for i := range jobs {
		if err := a.pushJob(&jobs[i], 0); err != nil {
			slog.Error("recover queued job", "jobID", jobs[i].ID, "error", err)
		}
	}
	return nil
}

func (a *App) recoverLegacyRetryingJobs() error {
	var jobs []JobRun
	if err := a.db.Where("status = ? AND next_attempt_at IS NULL", "retrying").Find(&jobs).Error; err != nil {
		return err
	}
	for i := range jobs {
		if err := a.pushJob(&jobs[i], 0); err != nil {
			slog.Error("recover legacy retrying job", "jobID", jobs[i].ID, "error", err)
			continue
		}
		now := time.Now()
		if err := a.db.Model(&jobs[i]).Update("next_attempt_at", &now).Error; err != nil {
			slog.Error("mark legacy retrying job recovered", "jobID", jobs[i].ID, "error", err)
		}
	}
	return nil
}

func (a *App) removeQueuedJobs(accountID uint) error {
	var jobs []JobRun
	if err := a.db.Where("account_id = ? AND status IN ?", accountID, []string{"queued", "retrying"}).Find(&jobs).Error; err != nil {
		return err
	}
	for _, job := range jobs {
		if job.QueueID != "" {
			_ = a.jobs.queue.Delete(context.Background(), goqite.ID(job.QueueID))
		}
	}
	return a.db.Where("account_id = ? AND status IN ?", accountID, []string{"queued", "retrying"}).Delete(&JobRun{}).Error
}

func (a *App) runJob(ctx context.Context, body []byte) error {
	var payload jobPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	var job JobRun
	if err := a.db.First(&job, payload.JobID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	now := time.Now()
	job.Attempts++
	job.Status, job.StartedAt, job.Error = "running", &now, ""
	job.NextAttemptAt = nil
	job.EndedAt = nil
	job.Logs = appendLog(job.Logs, fmt.Sprintf("Attempt %d started", job.Attempts))
	claimed := a.db.Model(&JobRun{}).Where("id = ? AND status IN ?", job.ID, []string{"queued", "retrying"}).Updates(map[string]any{
		"attempts":        job.Attempts,
		"status":          job.Status,
		"started_at":      job.StartedAt,
		"next_attempt_at": nil,
		"ended_at":        nil,
		"error":           "",
		"logs":            job.Logs,
	})
	if claimed.Error != nil {
		return claimed.Error
	}
	if claimed.RowsAffected == 0 {
		return nil
	}
	var result syncResult
	var err error
	workCtx, cancel := context.WithTimeout(ctx, jobTimeout())
	defer cancel()
	if job.AccountID == nil {
		err = fmt.Errorf("job has no account")
	} else if job.Type == jobSync {
		result, err = a.syncAccount(workCtx, *job.AccountID, job.Folder)
	} else if job.Type == jobVerify {
		err = a.verifyAccount(workCtx, *job.AccountID)
	} else {
		err = fmt.Errorf("unknown job type %q", job.Type)
	}
	ended := time.Now()
	job.Fetched, job.Created, job.Updated = result.Fetched, result.Created, result.Updated
	job.Synced = result.Created + result.Updated
	if err == nil {
		job.EndedAt = &ended
		job.Status = "succeeded"
		job.NextAttemptAt = nil
		job.Logs = appendLog(job.Logs, fmt.Sprintf("Completed successfully; %d fetched, %d created, %d updated", result.Fetched, result.Created, result.Updated))
		if err := a.db.Save(&job).Error; err != nil {
			return err
		}
		return nil
	}
	job.Error = safeError(err)
	job.Logs = appendLog(job.Logs, "Failed: "+job.Error)
	if !retryableJobError(err) || job.Attempts >= jobMaxAttempts() {
		job.EndedAt = &ended
		job.Status = "failed"
		job.NextAttemptAt = nil
		if err := a.db.Save(&job).Error; err != nil {
			return err
		}
		if err := a.recordAccountJobFailure(job); err != nil {
			slog.Error("record account job failure", "jobID", job.ID, "error", err)
		}
		return nil
	}
	delay := jobRetryDelay(job.Attempts)
	next := ended.Add(delay)
	job.EndedAt, job.NextAttemptAt = nil, &next
	job.Status = "retrying"
	job.Logs = appendLog(job.Logs, fmt.Sprintf("Retrying in %s", delay.Round(time.Second)))
	if saveErr := a.db.Save(&job).Error; saveErr != nil {
		return saveErr
	}
	if err := a.pushJob(&job, delay); err != nil {
		job.EndedAt, job.NextAttemptAt = &ended, nil
		job.Status, job.Error = "failed", safeError(err)
		job.Logs = appendLog(job.Logs, "Could not queue retry: "+job.Error)
		if saveErr := a.db.Save(&job).Error; saveErr != nil {
			return saveErr
		}
		if saveErr := a.recordAccountJobFailure(job); saveErr != nil {
			slog.Error("record account retry failure", "jobID", job.ID, "error", saveErr)
		}
	}
	return nil
}

func (a *App) syncAccount(ctx context.Context, id uint, folder string) (syncResult, error) {
	result := syncResult{}
	var account Account
	if err := a.db.Preload("Proxy").First(&account, id).Error; err != nil {
		return result, err
	}
	if err := a.openAccount(&account); err != nil {
		return result, err
	}
	if !account.Enabled || !account.SyncEnabled || account.Status == "disconnected" || account.Status == "reconnect_required" {
		return result, fmt.Errorf("account is not ready to sync")
	}
	var emails []Email
	var err error
	if account.Provider == "imap" {
		emails, err = retryMailboxMessages(ctx, func() ([]Email, error) {
			return imapMessages(account, folder)
		})
	} else if account.Provider == "pop3" {
		if folder != "all" && !strings.EqualFold(folder, "INBOX") {
			return result, fmt.Errorf("POP3 exposes only INBOX")
		}
		emails, err = retryMailboxMessages(ctx, func() ([]Email, error) {
			return pop3Messages(account)
		})
	} else {
		token, tokenErr := a.token(ctx, &account)
		if tokenErr != nil {
			account.LastError = safeError(tokenErr)
			if retryableJobError(tokenErr) {
				account.Status = "error"
			} else {
				account.Status = "reconnect_required"
			}
			if err := a.saveAccount(&account); err != nil {
				return result, err
			}
			return result, tokenErr
		}
		if account.Provider == "gmail" {
			emails, err = gmailMessages(ctx, account, token, folder)
		} else {
			emails, err = microsoftMessages(ctx, account, token, folder)
		}
	}
	checked := time.Now()
	account.LastCheckedAt = &checked
	if err != nil {
		account.Status, account.LastError = "error", safeError(err)
		if saveErr := a.saveAccount(&account); saveErr != nil {
			return result, saveErr
		}
		return result, err
	}
	result.Fetched = len(emails)
	result.Created, result.Updated, err = a.persistEmails(emails)
	if err != nil {
		account.Status, account.LastError = "error", safeError(err)
		if saveErr := a.saveAccount(&account); saveErr != nil {
			return result, saveErr
		}
		return result, err
	}
	next := checked.Add(time.Duration(positive(env("SYNC_INTERVAL_MINUTES", "15"), 15)) * time.Minute)
	account.LastSyncedAt, account.NextSyncAt, account.LastError, account.Status = &checked, &next, "", "connected"
	if err := a.saveAccount(&account); err != nil {
		return result, err
	}
	return result, nil
}

func (a *App) persistEmails(emails []Email) (int, int, error) {
	byFolder := map[string][]Email{}
	for _, email := range emails {
		byFolder[email.Folder] = append(byFolder[email.Folder], email)
	}
	created, updated := 0, 0
	for folder, items := range byFolder {
		for start := 0; start < len(items); start += 100 {
			end := start + 100
			if end > len(items) {
				end = len(items)
			}
			batch := items[start:end]
			remoteIDs := make([]string, 0, len(batch))
			for _, email := range batch {
				remoteIDs = append(remoteIDs, email.RemoteID)
			}
			var existing []Email
			if err := a.db.Select("remote_id").Where("account_id = ? AND folder = ? AND remote_id IN ?", batch[0].AccountID, folder, remoteIDs).Find(&existing).Error; err != nil {
				return created, updated, err
			}
			existingIDs := map[string]bool{}
			for _, email := range existing {
				existingIDs[email.RemoteID] = true
			}
			for _, email := range batch {
				if existingIDs[email.RemoteID] {
					updated++
				} else {
					created++
				}
			}
			if err := a.db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "account_id"}, {Name: "folder"}, {Name: "remote_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"thread_id", "from", "to", "subject", "preview", "body", "raw", "received_at", "is_read"}),
			}).Create(&batch).Error; err != nil {
				return created, updated, err
			}
		}
	}
	return created, updated, nil
}

func (a *App) verifyAccount(ctx context.Context, id uint) error {
	var account Account
	if err := a.db.Preload("Proxy").First(&account, id).Error; err != nil {
		return err
	}
	if err := a.openAccount(&account); err != nil {
		return err
	}
	var err error
	if account.Provider == "imap" {
		err = retryMailbox(ctx, func() error {
			_, err := imapFolders(account)
			return err
		})
	} else if account.Provider == "pop3" {
		err = retryMailbox(ctx, func() error { return pop3Verify(account) })
	} else {
		_, err = a.token(ctx, &account)
	}
	checked := time.Now()
	account.LastCheckedAt = &checked
	if err != nil {
		account.Status, account.LastError = "error", safeError(err)
		if usesOAuth(account.Provider) && !retryableJobError(err) {
			account.Status = "reconnect_required"
		}
		if saveErr := a.saveAccount(&account); saveErr != nil {
			return saveErr
		}
		return err
	}
	account.Status, account.LastError = "connected", ""
	if err := a.saveAccount(&account); err != nil {
		return err
	}
	if account.Enabled && account.SyncEnabled {
		if _, err := a.enqueueJob(jobSync, account.ID, "all"); err != nil {
			return err
		}
	}
	return nil
}

func retryableJobError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) && (networkError.Timeout() || networkError.Temporary()) {
		return true
	}
	var providerError *providerStatusError
	if errors.As(err, &providerError) {
		return retryableStatus(providerError.Status)
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database is busy")
}

func (a *App) recordAccountJobFailure(job JobRun) error {
	if job.AccountID == nil {
		return nil
	}
	now := time.Now()
	return a.db.Model(&Account{}).
		Where("id = ? AND status <> ?", *job.AccountID, "reconnect_required").
		Updates(map[string]any{"status": "error", "last_error": job.Error, "last_checked_at": &now}).Error
}

func retryMailboxMessages(ctx context.Context, fetch func() ([]Email, error)) ([]Email, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		emails, err := fetch()
		if err == nil {
			return emails, nil
		}
		lastErr = err
		if attempt < 2 {
			if err := waitForRetry(ctx, retryDelay(attempt, "")); err != nil {
				return nil, err
			}
		}
	}
	return nil, lastErr
}

func retryMailbox(ctx context.Context, operation func() error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < 2 {
			if err := waitForRetry(ctx, retryDelay(attempt, "")); err != nil {
				return err
			}
		}
	}
	return lastErr
}

func appendLog(logs, line string) string {
	const maxLogBytes = 32 << 10
	entry := time.Now().Format(time.RFC3339) + " " + line
	if logs == "" {
		return entry
	}
	logs += "\n" + entry
	if len(logs) > maxLogBytes {
		return "...\n" + logs[len(logs)-maxLogBytes:]
	}
	return logs
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 600 {
		message = message[:600] + "..."
	}
	return message
}
