import {
  Badge,
  Button,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
  Spinner,
} from "@fluentui/react-components";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, type JobRun } from "../api";
import { EmptyState, LoadingState } from "../components/states";

export function JobsView() {
  const queryClient = useQueryClient();
  const [selectedID, setSelectedID] = useState<number>();
  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: api.jobs,
  });
  const retry = useMutation({
    mutationFn: api.retryJob,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["jobs"] }),
  });
  const clear = useMutation({
    mutationFn: api.clearJobs,
    onSuccess: () => {
      setSelectedID(undefined);
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
    },
  });
  if (jobs.isLoading) return <LoadingState />;
  const selected =
    jobs.data?.find((job) => job.id === selectedID) ?? jobs.data?.[0];
  const hasHistory = jobs.data?.some(
    (job) => job.status === "succeeded" || job.status === "failed",
  );
  return (
    <section>
      <div className="page-intro">
        <div>
          <h2>Jobs</h2>
          <p>
            Durable database-backed work. Active jobs refresh automatically;
            completed history is checked less frequently.
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
              if (confirm("Clear completed and failed job history?")) {
                clear.mutate();
              }
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
      {!jobs.data?.length ? (
        <EmptyState
          icon={null}
          title="No jobs yet"
          text="Connection checks and mailbox syncs will appear here."
        />
      ) : (
        <div className="jobs-layout">
          <div className="job-list">
            {jobs.data.map((job) => (
              <button
                key={job.id}
                className={`job-row ${selected?.id === job.id ? "job-row--active" : ""}`}
                onClick={() => setSelectedID(job.id)}
              >
                <div>
                  <strong>{job.account?.email ?? "Unknown account"}</strong>
                  <p>
                    {job.type.replaceAll("_", " ")} ·{" "}
                    {new Date(job.createdAt).toLocaleString()}
                  </p>
                </div>
                <Badge
                  appearance="filled"
                  color={
                    job.status === "succeeded"
                      ? "success"
                      : job.status === "failed"
                        ? "danger"
                        : "informative"
                  }
                >
                  {job.status}
                </Badge>
              </button>
            ))}
          </div>
          <JobDetails
            job={selected}
            retrying={retry.isPending}
            onRetry={() => selected && retry.mutate(selected.id)}
          />
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
  const active =
    job.status === "queued" ||
    job.status === "running" ||
    job.status === "retrying";
  return (
    <article className="job-details">
      <div className="job-details__heading">
        <div>
          <p className="overline">{job.type.replaceAll("_", " ")}</p>
          <h3>{job.account?.email ?? "Unknown account"}</h3>
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
      <pre className="job-log">{job.logs || "No job events recorded."}</pre>
      {job.status === "failed" && (
        <Button appearance="primary" onClick={onRetry} disabled={retrying}>
          {retrying ? "Queueing…" : "Retry job"}
        </Button>
      )}
    </article>
  );
}
