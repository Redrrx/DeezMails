import {
  Toast,
  ToastBody,
  ToastTitle,
  Toaster,
  useToastController,
} from "@fluentui/react-components";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useId, useRef } from "react";
import { api } from "../api";
import { useUI } from "../store";

export function LiveUpdates() {
  const toasterId = useId();
  const queryClient = useQueryClient();
  const { dispatchToast } = useToastController(toasterId);
  const notificationsMuted = useUI((state) => state.notificationsMuted);
  const seen = useRef(new Map<number, string>());
  const initialized = useRef(false);
  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: api.jobs,
    refetchInterval: (query) =>
      query.state.data?.some(
        (job) =>
          job.status === "queued" ||
          job.status === "running" ||
          job.status === "retrying",
      )
        ? 3000
        : 15000,
  });

  useEffect(() => {
    if (!jobs.data) return;
    const previous = seen.current;
    if (initialized.current) {
      for (const job of jobs.data) {
        const changed = previous.get(job.id) !== job.status;
        if (
          !changed ||
          (job.status !== "succeeded" && job.status !== "failed")
        ) {
          continue;
        }
        if (job.type === "sync_account" && job.accountId) {
          queryClient.invalidateQueries({
            queryKey: ["emails", job.accountId],
          });
          queryClient.invalidateQueries({ queryKey: ["accounts"] });
        }
        if (notificationsMuted) continue;
        const email = job.account?.email ?? "Mailbox";
        const title =
          job.status === "succeeded" ? "Job completed" : "Job failed";
        const detail =
          job.status === "succeeded"
            ? `${email}: ${job.synced} messages synchronized.`
            : `${email}: ${job.error || "Open Jobs for details."}`;
        dispatchToast(
          <Toast>
            <ToastTitle>{title}</ToastTitle>
            <ToastBody>{detail}</ToastBody>
          </Toast>,
          { intent: job.status === "succeeded" ? "success" : "error" },
        );
      }
    }
    seen.current = new Map(jobs.data.map((job) => [job.id, job.status]));
    initialized.current = true;
  }, [dispatchToast, jobs.data, notificationsMuted, queryClient]);

  return <Toaster toasterId={toasterId} position="top-end" />;
}
