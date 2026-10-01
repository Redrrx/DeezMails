package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/iamolegga/goqite"
	queuejobs "github.com/iamolegga/goqite/jobs"
	"gorm.io/gorm"

	"deezmails/internal/models"
)

type jobService struct {
	queue   *goqite.Queue
	workers sync.WaitGroup
	stopped chan struct{}
}

type jobPayload struct {
	JobID uint `json:"jobId"`
}

type syncResult struct {
	Fetched int
	Created int
	Updated int
}

func (a *App) startJobs(ctx context.Context) error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	options := goqite.NewOpts{DB: sqlDB, Name: "deezmails", MaxReceive: a.config.JobMaxAttempts, Timeout: a.config.JobTimeout, TablePrefix: "deezmails_"}
	if a.config.DatabaseURL != "" {
		options.SQLFlavor = goqite.SQLFlavorPostgreSQL
		options.TablePrefix = ""
	}
	queue := goqite.New(options)
	if err := queue.Setup(ctx); err != nil {
		return err
	}
	a.jobs = &jobService{queue: queue, stopped: make(chan struct{})}
	if err := a.recoverQueuedJobs(ctx); err != nil {
		return err
	}
	runner := queuejobs.NewRunner(queuejobs.NewRunnerOpts{
		Queue:        queue,
		Limit:        a.config.WorkerConcurrency,
		PollInterval: time.Second,
		Extend:       time.Minute,
		Log:          slog.Default(),
	})
	runner.Register(string(models.JobTypeSyncAccount), a.runJob)
	runner.Register(string(models.JobTypeVerifyConnection), a.runJob)
	a.jobs.workers.Add(2)
	go a.runQueueWorkers(ctx, runner)
	go a.runJobScheduler(ctx, a.config.SyncInterval)
	go a.jobs.signalWhenStopped()
	return nil
}

func (a *App) runQueueWorkers(ctx context.Context, runner *queuejobs.Runner) {
	defer a.jobs.workers.Done()
	runner.Start(ctx)
}

func (a *App) runJobScheduler(ctx context.Context, interval time.Duration) {
	defer a.jobs.workers.Done()
	a.scheduleJobs(ctx, interval)
}

func (service *jobService) signalWhenStopped() {
	service.workers.Wait()
	close(service.stopped)
}

func (service *jobService) wait(ctx context.Context) error {
	select {
	case <-service.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *App) scheduleJobs(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		a.scheduleDueAccounts(ctx, interval)
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) scheduleDueAccounts(ctx context.Context, interval time.Duration) {
	now := time.Now()
	db := a.db.WithContext(ctx)
	var accounts []models.Account
	if err := db.Where("enabled = ? AND sync_enabled = ? AND status = ? AND (next_sync_at IS NULL OR next_sync_at <= ?)", true, true, models.AccountStatusConnected, now).Order("next_sync_at ASC NULLS FIRST").Limit(a.config.SchedulerBatchSize).Find(&accounts).Error; err != nil {
		slog.Error("load scheduled accounts", "error", err)
		return
	}
	for _, account := range accounts {
		job, err := a.enqueueJob(ctx, models.JobTypeSyncAccount, account.ID, folderAll)
		if err != nil {
			slog.Error("queue scheduled sync", "accountID", account.ID, "error", err)
			continue
		}
		if job.Status != models.JobStatusQueued {
			continue
		}
		next := now.Add(interval + time.Duration(account.ID%30)*time.Second)
		if err := db.Model(&models.Account{}).Where("id = ?", account.ID).Update("next_sync_at", next).Error; err != nil {
			slog.Error("set next sync time", "accountID", account.ID, "error", err)
		}
	}
}

func (a *App) enqueueJob(ctx context.Context, kind models.JobType, accountID uint, folder string) (*models.JobRun, error) {
	db := a.db.WithContext(ctx)
	var existing models.JobRun
	activeStatuses := []models.JobStatus{models.JobStatusQueued, models.JobStatusRunning, models.JobStatusRetrying}
	lookup := db.Where("account_id = ? AND type = ? AND folder = ? AND status IN ?", accountID, kind, folder, activeStatuses).First(&existing)
	if lookup.Error == nil {
		return &existing, nil
	}
	if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
		return nil, lookup.Error
	}
	job := models.JobRun{AccountID: &accountID, Type: kind, Folder: folder, Status: models.JobStatusQueued, Logs: "Queued"}
	if err := db.Create(&job).Error; err != nil {
		lookup = db.Where("account_id = ? AND type = ? AND folder = ? AND status IN ?", accountID, kind, folder, activeStatuses).First(&existing)
		if lookup.Error == nil {
			return &existing, nil
		}
		return nil, err
	}
	if err := a.pushJob(ctx, &job, 0); err != nil {
		now := time.Now()
		job.Status, job.Error, job.ErrorCode, job.ErrorAction, job.EndedAt = models.JobStatusFailed, "Could not queue the job.", models.MailErrorInternal, models.MailErrorActionRetry, &now
		job.Logs = appendJobLog(job.Logs, "Queueing failed: "+job.Error)
		slog.Error("queue job", "jobID", job.ID, "error", err)
		if saveErr := db.Save(&job).Error; saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}
	return &job, nil
}

func (a *App) pushJob(ctx context.Context, job *models.JobRun, delay time.Duration) error {
	payload, err := json.Marshal(jobPayload{JobID: job.ID})
	if err != nil {
		return err
	}
	queueID, err := queuejobs.Create(ctx, a.jobs.queue, string(job.Type), goqite.Message{Body: payload, Delay: delay, Priority: 10})
	if err != nil {
		return err
	}
	job.QueueID = string(queueID)
	if err := a.db.WithContext(ctx).Save(job).Error; err != nil {
		return err
	}
	return nil
}

func (a *App) recoverQueuedJobs(ctx context.Context) error {
	var jobs []models.JobRun
	if err := a.db.WithContext(ctx).Where("status = ? AND queue_id = ?", models.JobStatusQueued, "").Find(&jobs).Error; err != nil {
		return err
	}
	for i := range jobs {
		if err := a.pushJob(ctx, &jobs[i], 0); err != nil {
			slog.Error("recover queued job", "jobID", jobs[i].ID, "error", err)
		}
	}
	return nil
}

func (a *App) removeQueuedJobs(ctx context.Context, accountID uint) error {
	db := a.db.WithContext(ctx)
	var jobs []models.JobRun
	removableStatuses := []models.JobStatus{models.JobStatusQueued, models.JobStatusRetrying}
	if err := db.Where("account_id = ? AND status IN ?", accountID, removableStatuses).Find(&jobs).Error; err != nil {
		return err
	}
	for _, job := range jobs {
		if job.QueueID != "" {
			_ = a.jobs.queue.Delete(ctx, goqite.ID(job.QueueID))
		}
	}
	return db.Where("account_id = ? AND status IN ?", accountID, removableStatuses).Delete(&models.JobRun{}).Error
}

func (a *App) runJob(ctx context.Context, body []byte) error {
	workCtx, cancel := context.WithTimeout(ctx, a.config.JobTimeout)
	defer cancel()
	db := a.db.WithContext(workCtx)
	var payload jobPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	var job models.JobRun
	if err := db.First(&job, payload.JobID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	now := time.Now()
	job.Attempts++
	job.Status, job.StartedAt, job.Error, job.ErrorCode, job.ErrorAction = models.JobStatusRunning, &now, "", "", ""
	job.NextAttemptAt = nil
	job.EndedAt = nil
	job.Logs = appendJobLog(job.Logs, fmt.Sprintf("Attempt %d started", job.Attempts))
	staleBefore := now.Add(-a.config.JobTimeout)
	claimed := db.Model(&models.JobRun{}).Where("id = ? AND (status IN ? OR (status = ? AND started_at <= ?))", job.ID, []models.JobStatus{models.JobStatusQueued, models.JobStatusRetrying}, models.JobStatusRunning, staleBefore).Updates(map[string]any{
		"attempts":        job.Attempts,
		"status":          job.Status,
		"started_at":      job.StartedAt,
		"next_attempt_at": nil,
		"ended_at":        nil,
		"error":           "",
		"error_code":      "",
		"error_action":    "",
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
	if job.AccountID == nil {
		err = fmt.Errorf("job has no account")
	} else if job.Type == models.JobTypeSyncAccount {
		result, err = a.syncAccount(workCtx, *job.AccountID, job.Folder)
	} else if job.Type == models.JobTypeVerifyConnection {
		err = a.verifyAccount(workCtx, *job.AccountID)
	} else {
		err = fmt.Errorf("unknown job type %q", job.Type)
	}
	ended := time.Now()
	job.Fetched, job.Created, job.Updated = result.Fetched, result.Created, result.Updated
	job.Synced = result.Created + result.Updated
	if err == nil {
		job.EndedAt = &ended
		job.Status = models.JobStatusSucceeded
		job.NextAttemptAt = nil
		job.Logs = appendJobLog(job.Logs, fmt.Sprintf("Completed successfully; %d fetched, %d created, %d updated", result.Fetched, result.Created, result.Updated))
		if err := db.Save(&job).Error; err != nil {
			return err
		}
		return nil
	}
	failure := normalizeMailError(err)
	slog.Error("job attempt failed", "jobID", job.ID, "code", failure.Code, "error", err)
	job.Error, job.ErrorCode, job.ErrorAction = failure.Message, failure.Code, failure.Action
	job.Logs = appendJobLog(job.Logs, "Failed: "+job.Error)
	if failure.Action != models.MailErrorActionRetry || job.Attempts >= a.config.JobMaxAttempts {
		job.EndedAt = &ended
		job.Status = models.JobStatusFailed
		job.NextAttemptAt = nil
		if err := db.Save(&job).Error; err != nil {
			return err
		}
		if err := a.recordAccountJobFailure(workCtx, job); err != nil {
			slog.Error("record account job failure", "jobID", job.ID, "error", err)
		}
		return nil
	}
	delay := failure.RetryAfter
	if delay == 0 {
		exponent := min(max(job.Attempts-1, 0), 6)
		delay = min(a.config.JobRetryBase<<exponent, 5*time.Minute) + time.Duration(rand.IntN(1000))*time.Millisecond
	}
	next := ended.Add(delay)
	job.EndedAt, job.NextAttemptAt = nil, &next
	job.Status = models.JobStatusRetrying
	job.Logs = appendJobLog(job.Logs, fmt.Sprintf("Retrying in %s", delay.Round(time.Second)))
	if saveErr := db.Save(&job).Error; saveErr != nil {
		return saveErr
	}
	if err := a.pushJob(workCtx, &job, delay); err != nil {
		job.EndedAt, job.NextAttemptAt = &ended, nil
		job.Status, job.Error, job.ErrorCode, job.ErrorAction = models.JobStatusFailed, "Could not schedule the retry.", models.MailErrorInternal, models.MailErrorActionRetry
		job.Logs = appendJobLog(job.Logs, "Could not queue retry: "+job.Error)
		slog.Error("queue job retry", "jobID", job.ID, "error", err)
		if saveErr := db.Save(&job).Error; saveErr != nil {
			return saveErr
		}
		if saveErr := a.recordAccountJobFailure(workCtx, job); saveErr != nil {
			slog.Error("record account retry failure", "jobID", job.ID, "error", saveErr)
		}
	}
	return nil
}

func (a *App) recordAccountJobFailure(ctx context.Context, job models.JobRun) error {
	if job.AccountID == nil {
		return nil
	}
	updates := map[string]any{"last_error": job.Error, "last_error_code": job.ErrorCode, "last_error_action": job.ErrorAction}
	if job.ErrorAction == models.MailErrorActionReconnect {
		updates["status"] = models.AccountStatusReconnectRequired
	} else if job.ErrorAction != models.MailErrorActionNone {
		updates["status"] = models.AccountStatusError
	}
	now := time.Now()
	updates["last_checked_at"] = &now
	return a.db.WithContext(ctx).Model(&models.Account{}).
		Where("id = ? AND status <> ?", *job.AccountID, models.AccountStatusReconnectRequired).
		Updates(updates).Error
}

func appendJobLog(logs, line string) string {
	const maxBytes = 32 * 1024
	entry := time.Now().Format(time.RFC3339) + " " + line
	if logs == "" {
		return entry
	}
	logs += "\n" + entry
	if len(logs) <= maxBytes {
		return logs
	}
	return "...\n" + strings.ToValidUTF8(logs[len(logs)-maxBytes:], "�")
}
