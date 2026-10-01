import {
  Button,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
  Spinner,
} from "@fluentui/react-components";
import { ApiError } from "../api";

export function LoadingState() {
  return (
    <div className="loading-state">
      <Spinner label="Loading" labelPosition="below" />
    </div>
  );
}

export function EmptyState({ title, text }: { title: string; text: string }) {
  return (
    <div className="empty-state">
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

export function ErrorState({
  title,
  error,
  onRetry,
}: {
  title: string;
  error: unknown;
  onRetry: () => void;
}) {
  const apiError = error instanceof ApiError ? error : undefined;
  const guidance = apiError?.action
    ? {
        reconnect: "Reconnect the account.",
        edit_account: "Edit the account settings.",
        contact_admin: "Contact the administrator.",
        retry: apiError.retryAfterSeconds
          ? `Retry after about ${apiError.retryAfterSeconds} seconds.`
          : "Try again.",
        none: "",
      }[apiError.action]
    : "";
  const canRetry = !apiError?.action || apiError.action === "retry";
  return (
    <MessageBar intent="error">
      <MessageBarBody>
        <MessageBarTitle>{title}</MessageBarTitle>
        <span>{error instanceof Error ? error.message : "Request failed"}</span>
        {guidance && <span>{guidance}</span>}
        {canRetry && (
          <Button size="small" appearance="secondary" onClick={onRetry}>
            Retry
          </Button>
        )}
      </MessageBarBody>
    </MessageBar>
  );
}
