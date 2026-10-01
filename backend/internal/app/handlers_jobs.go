package app

import (
	"net/http"
	"strconv"
	"strings"

	"deezmails/internal/models"
	"deezmails/internal/validation"

	"github.com/gin-gonic/gin"
)

// sync godoc
// @Summary Queue a mailbox sync job
// @Tags accounts
// @Produce json
// @Param id path int true "Account ID"
// @Param folder query string false "Folder ID or all" default(all)
// @Success 202 {object} models.JobRun
// @Router /api/accounts/{id}/sync [post]
func (a *App) sync(c *gin.Context) {
	account, ok := a.account(c)
	if !ok {
		return
	}
	if !account.Enabled || !account.SyncEnabled {
		c.JSON(http.StatusConflict, gin.H{"error": "account syncing is disabled"})
		return
	}
	if account.Status != models.AccountStatusConnected && !(account.Status == models.AccountStatusError && account.LastErrorAction == models.MailErrorActionRetry) {
		c.JSON(http.StatusConflict, gin.H{"error": "account is not connected; reconnect it first"})
		return
	}
	folder := strings.TrimSpace(c.DefaultQuery("folder", folderAll))
	if folder == "" || len(folder) > 1024 || validation.HasLineBreakOrNUL(folder) {
		badRequest(c, "invalid folder")
		return
	}
	job, err := a.enqueueJob(c.Request.Context(), models.JobTypeSyncAccount, account.ID, folder)
	if err != nil {
		internalError(c, "could not queue mailbox sync", err)
		return
	}
	c.JSON(http.StatusAccepted, job)
}

// listJobs godoc
// @Summary List durable job runs
// @Tags jobs
// @Produce json
// @Param status query string false "Filter status"
// @Param accountId query int false "Filter account"
// @Success 200 {array} models.JobRun
// @Router /api/jobs [get]
func (a *App) listJobs(c *gin.Context) {
	query := a.requestDB(c).Model(&models.JobRun{})
	if status := models.JobStatus(c.Query("status")); status != "" {
		switch status {
		case models.JobStatusQueued, models.JobStatusRunning, models.JobStatusRetrying, models.JobStatusSucceeded, models.JobStatusFailed:
		default:
			badRequest(c, "invalid job status")
			return
		}
		query = query.Where("job_runs.status = ?", status)
	}
	if accountID := c.Query("accountId"); accountID != "" {
		parsed, err := strconv.ParseUint(accountID, 10, 64)
		if err != nil || parsed == 0 {
			badRequest(c, "invalid accountId")
			return
		}
		query = query.Where("job_runs.account_id = ?", parsed)
	}
	var jobs []models.JobRun
	if err := query.Preload("Account").Order("job_runs.id DESC").Limit(200).Find(&jobs).Error; err != nil {
		internalError(c, "could not load jobs", err)
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// listJobSummaries godoc
// @Summary List lightweight job status rows
// @Tags jobs
// @Produce json
// @Success 200 {array} models.JobSummary
// @Router /api/jobs/summary [get]
func (a *App) listJobSummaries(c *gin.Context) {
	var jobs []models.JobSummary
	err := a.requestDB(c).Model(&models.JobRun{}).
		Select(`job_runs.id, job_runs.account_id, accounts.email AS account_email,
			job_runs.type, job_runs.status, job_runs.attempts, job_runs.fetched,
			job_runs.created, job_runs.updated, job_runs.synced, job_runs.error,
			job_runs.error_code, job_runs.error_action,
			job_runs.started_at, job_runs.next_attempt_at, job_runs.ended_at,
			job_runs.created_at, job_runs.updated_at`).
		Joins("LEFT JOIN accounts ON accounts.id = job_runs.account_id").
		Order("job_runs.id DESC").Limit(200).Scan(&jobs).Error
	if err != nil {
		internalError(c, "could not load job summaries", err)
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// clearJobs godoc
// @Summary Clear completed and failed job history
// @Tags jobs
// @Produce json
// @Success 200 {object} map[string]int
// @Router /api/jobs [delete]
func (a *App) clearJobs(c *gin.Context) {
	result := a.requestDB(c).Where("status IN ?", []models.JobStatus{models.JobStatusSucceeded, models.JobStatusFailed}).Delete(&models.JobRun{})
	if result.Error != nil {
		internalError(c, "could not clear job history", result.Error)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": result.RowsAffected})
}

// getJob godoc
// @Summary Get a durable job run
// @Tags jobs
// @Produce json
// @Param id path int true "Job ID"
// @Success 200 {object} models.JobRun
// @Router /api/jobs/{id} [get]
func (a *App) getJob(c *gin.Context) {
	var job models.JobRun
	if err := a.requestDB(c).Preload("Account").First(&job, c.Param("id")).Error; err != nil {
		handleLookupError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

// retryJob godoc
// @Summary Retry a failed job
// @Tags jobs
// @Produce json
// @Param id path int true "Job ID"
// @Success 202 {object} models.JobRun
// @Router /api/jobs/{id}/retry [post]
func (a *App) retryJob(c *gin.Context) {
	var previous models.JobRun
	if err := a.requestDB(c).First(&previous, c.Param("id")).Error; err != nil {
		handleLookupError(c, err)
		return
	}
	if previous.Status != models.JobStatusFailed {
		c.JSON(http.StatusConflict, gin.H{"error": "only failed jobs can be retried"})
		return
	}
	if previous.ErrorAction != "" && previous.ErrorAction != models.MailErrorActionRetry {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "This failure needs another action and cannot be retried.", Code: previous.ErrorCode, Action: previous.ErrorAction})
		return
	}
	if previous.AccountID == nil {
		badRequest(c, "job has no account")
		return
	}
	var account models.Account
	if err := a.requestDB(c).Select("id", "enabled").First(&account, *previous.AccountID).Error; err != nil {
		handleLookupError(c, err)
		return
	}
	if !account.Enabled {
		c.JSON(http.StatusConflict, gin.H{"error": "the job account is disabled"})
		return
	}
	if previous.Type != models.JobTypeSyncAccount && previous.Type != models.JobTypeVerifyConnection {
		badRequest(c, "job type cannot be retried")
		return
	}
	job, err := a.enqueueJob(c.Request.Context(), previous.Type, *previous.AccountID, previous.Folder)
	if err != nil {
		internalError(c, "could not retry job", err)
		return
	}
	c.JSON(http.StatusAccepted, job)
}
