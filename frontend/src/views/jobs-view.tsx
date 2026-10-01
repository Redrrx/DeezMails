import {
  Button,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
  Spinner,
} from "@fluentui/react-components";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../api";
import { EmptyState, ErrorState, LoadingState } from "../components/states";
import { activeJobStatuses, type JobRun } from "../models";

export function JobsView() {
  const queryClient = useQueryClient();
  const [selectedID, setSelectedID] = useState<number>();
  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: ({ signal }) => api.jobs(signal),
  });
  const selectedSummary =
    jobs.data?.find((job) => job.id === selectedID) ?? jobs.data?.[0];
  const selectedJobID = selectedSummary?.id;
  const detail = useQuery({
    queryKey: ["job", selectedJobID, selectedSummary?.updatedAt],
    queryFn: ({ signal }) => api.job(selectedJobID!, signal),
    enabled: Boolean(selectedJobID),
  });
  const retry = useMutation({
    mutationFn: api.retryJob,
    onSuccess: (job) => {
      setSelectedID(job.id);
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
    },
  });
  const clear = useMutation({
    mutationFn: api.clearJobs,
    onSuccess: () => {
      setSelectedID(undefined);
      queryClient.removeQueries({ queryKey: ["job"] });
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
    },
  });

  if (jobs.isLoading) return <LoadingState />;
  if (jobs.isError)
    return (
      <ErrorState
        title="Could not load jobs"
        error={jobs.error}
        onRetry={() => jobs.refetch()}
      />
    );

  const hasHistory = jobs.data?.some(
    (job) => job.status === "succeeded" || job.status === "failed",
  );

  return (
    <section>
      <div className="page-intro">
        <div>
          <h2>Jobs</h2>
          <p>
            Mailbox sync and connection history. Active jobs refresh
            automatically; completed history is checked less frequently.
          </p>
        </div>
        <div className="actions">
          <Button
            appearance="secondary"
            onClick={() =>
              queryClient.invalidateQueries({ queryKey: ["jobs"] })
            }
          >
            Refresh
          </Button>
          <Button
            appearance="subtle"
            className="danger-button"
            disabled={!hasHistory || clear.isPending}
            onClick={() => {
              if (confirm("Clear completed and failed job history?"))
                clear.mutate();
            }}
          >
            {clear.isPending ? "Clearing…" : "Clear history"}
          </Button>
        </div>
      </div>
      {clear.error && (
        <MessageBar intent="error">
          <MessageBarBody>
            <MessageBarTitle>Could not clear job history</MessageBarTitle>
            {clear.error.message}
          </MessageBarBody>
        </MessageBar>
      )}
      {retry.error && (
        <MessageBar intent="error">
          <MessageBarBody>
            <MessageBarTitle>Could not retry job</MessageBarTitle>
            {retry.error.message}
          </MessageBarBody>
        </MessageBar>
      )}
      {!jobs.data?.length ? (
        <EmptyState
          title="No jobs yet"
          text="Connection checks and mailbox syncs will appear here."
        />
      ) : (
        <div className="jobs-layout">
          <div className="job-list">
            {jobs.data.map((job) => (
              <button
                type="button"
                key={job.id}
                className={`job-row ${selectedSummary?.id === job.id ? "job-row--active" : ""}`}
                aria-pressed={selectedSummary?.id === job.id}
                onClick={() => setSelectedID(job.id)}
              >
                <span className="job-row__copy">
                  <strong>{job.accountEmail || "Unknown account"}</strong>
                  <span>
                    {job.type.replaceAll("_", " ")} ·{" "}
                    {new Date(job.createdAt).toLocaleString()}
                  </span>
                </span>
                <span className={`job-status job-status--${job.status}`}>
                  {job.status}
                </span>
              </button>
            ))}
          </div>
          {detail.isLoading ? (
            <div className="job-details">
              <LoadingState />
            </div>
          ) : detail.isError ? (
            <div className="job-details">
              <ErrorState
                title="Could not load job details"
                error={detail.error}
                onRetry={() => detail.refetch()}
              />
            </div>
          ) : (
            <JobDetails
              job={detail.data}
              retrying={retry.isPending && retry.variables === selectedJobID}
              onRetry={() => selectedJobID && retry.mutate(selectedJobID)}
            />
          )}
        </div>
      )}
    </section>
  );
}

function JobDetails({
  job,
  retrying,
  onRetry,
}: {
  job?: JobRun;
  retrying: boolean;
  onRetry: () => void;
}) {
  if (!job) return null;
  const active = activeJobStatuses.has(job.status);
  const retryable = !job.errorAction || job.errorAction === "retry";
  const guidance =
    job.errorAction === "reconnect"
      ? "Reconnect the account before starting another job."
      : job.errorAction === "edit_account"
        ? "Edit the account settings before starting another job."
        : job.errorAction === "contact_admin"
          ? "Administrator action is required."
          : "";
  return (
    <article className="job-details">
      <div className="job-details__heading">
        <div>
          <p className="overline">{job.type.replaceAll("_", " ")}</p>
          <h3>{job.account?.email || job.accountEmail || "Unknown account"}</h3>
        </div>
        {active && <Spinner size="tiny" />}
      </div>
      <dl>
        <div>
          <dt>Status</dt>
          <dd>{job.status}</dd>
        </div>
        <div>
          <dt>Attempts</dt>
          <dd>{job.attempts}</dd>
        </div>
        {job.nextAttemptAt && (
          <div>
            <dt>Next retry</dt>
            <dd>{new Date(job.nextAttemptAt).toLocaleString()}</dd>
          </div>
        )}
        <div>
          <dt>Fetched</dt>
          <dd>{job.fetched}</dd>
        </div>
        <div>
          <dt>Created / updated</dt>
          <dd>
            {job.created} / {job.updated}
          </dd>
        </div>
        <div>
          <dt>Queued</dt>
          <dd>{new Date(job.createdAt).toLocaleString()}</dd>
        </div>
      </dl>
      {job.status === "succeeded" && (
        <div className="job-success">
          Completed successfully. {job.fetched} fetched, {job.created} new, and{" "}
          {job.updated} updated.
        </div>
      )}
      {job.error && <div className="job-error">{job.error}</div>}
      {guidance && <p>{guidance}</p>}
      <pre className="job-log">{job.logs || "No job events recorded."}</pre>
      {job.status === "failed" && retryable && (
        <Button appearance="primary" onClick={onRetry} disabled={retrying}>
          {retrying ? "Queueing…" : "Retry job"}
        </Button>
      )}
    </article>
  );
}
