import { Badge, Button, Spinner } from "@fluentui/react-components";
import type { Account, JobSummary } from "../../models";

export function AccountCard({
  account,
  formOpen,
  activeJob,
  reconnectPending,
  syncPending,
  statePending,
  removePending,
  mutationPending,
  onReconnect,
  onSync,
  onOpen,
  onEdit,
  onToggleEnabled,
  onToggleSync,
  onDelete,
}: {
  account: Account;
  formOpen: boolean;
  activeJob?: JobSummary;
  reconnectPending: boolean;
  syncPending: boolean;
  statePending: boolean;
  removePending: boolean;
  mutationPending: boolean;
  onReconnect: () => void;
  onSync: () => void;
  onOpen: () => void;
  onEdit: () => void;
  onToggleEnabled: () => void;
  onToggleSync: () => void;
  onDelete: () => void;
}) {
  const connected = account.status === "connected";
  const retryable = account.lastErrorAction === "retry";
  const showReconnect =
    !connected &&
    (account.lastErrorAction === "reconnect" ||
      !account.lastErrorAction ||
      account.status === "disconnected");
  const guidance =
    account.lastErrorAction === "edit_account"
      ? "Edit account settings, then save to verify again."
      : account.lastErrorAction === "contact_admin"
        ? "Administrator action is required."
        : "";
  const working = Boolean(activeJob) || reconnectPending || syncPending;
  const controlsLocked = formOpen || working || mutationPending;
  const actionLabel =
    activeJob?.status === "retrying"
      ? `Retrying (attempt ${activeJob.attempts})`
      : activeJob?.type === "verify_connection"
        ? "Checking connection..."
        : activeJob?.type === "sync_account"
          ? "Syncing..."
          : "";
  const provider =
    account.provider === "imap"
      ? `IMAP ${account.incomingHost}:${account.incomingPort}`
      : account.provider === "pop3"
        ? `POP3 ${account.incomingHost}:${account.incomingPort}`
        : account.provider === "gmail"
          ? "Gmail"
          : "Microsoft";
  const activity = account.lastSyncedAt
    ? `Synced ${new Date(account.lastSyncedAt).toLocaleString()}`
    : account.lastCheckedAt
      ? `Checked ${new Date(account.lastCheckedAt).toLocaleString()}`
      : "Not synced";

  return (
    <article className="account-card">
      <div>
        <div className="account-name">
          <h3>{account.email}</h3>
          <Badge
            appearance="filled"
            color={connected ? "success" : "informative"}
          >
            {account.status.replaceAll("_", " ")}
          </Badge>
          {!account.enabled && <Badge appearance="outline">Disabled</Badge>}
          <Badge appearance="outline">
            {account.syncEnabled ? "Sync on" : "Sync paused"}
          </Badge>
        </div>
        <p className="account-meta">
          {provider} · {account.proxy?.name ?? "Direct connection"} · {activity}
        </p>
        {account.lastError && (
          <p className="account-error">{account.lastError}</p>
        )}
        {guidance && <p className="account-meta">{guidance}</p>}
      </div>
      <div className="actions">
        {showReconnect && (
          <Button
            size="small"
            appearance="primary"
            disabled={!account.enabled || controlsLocked}
            icon={working ? <Spinner size="tiny" /> : undefined}
            onClick={onReconnect}
          >
            {actionLabel ||
              (reconnectPending ? "Reconnecting..." : "Reconnect")}
          </Button>
        )}
        {(connected || retryable) && (
          <Button
            size="small"
            appearance="primary"
            disabled={
              !account.enabled || !account.syncEnabled || controlsLocked
            }
            icon={working ? <Spinner size="tiny" /> : undefined}
            onClick={onSync}
          >
            {actionLabel ||
              (syncPending
                ? "Syncing..."
                : retryable
                  ? "Retry sync"
                  : "Sync all")}
          </Button>
        )}
        <Button
          size="small"
          appearance="subtle"
          disabled={controlsLocked}
          onClick={onEdit}
        >
          Edit
        </Button>
        <Button
          size="small"
          appearance="secondary"
          disabled={!connected || controlsLocked}
          onClick={onOpen}
        >
          Mailbox
        </Button>
        <Button
          size="small"
          appearance="subtle"
          disabled={controlsLocked}
          onClick={onToggleEnabled}
        >
          {statePending ? "Updating…" : account.enabled ? "Disable" : "Enable"}
        </Button>
        <Button
          size="small"
          appearance="subtle"
          disabled={!account.enabled || controlsLocked}
          onClick={onToggleSync}
        >
          {account.syncEnabled ? "Pause sync" : "Resume sync"}
        </Button>
        <Button
          size="small"
          appearance="subtle"
          className="danger-button"
          disabled={controlsLocked}
          title={
            formOpen ? "Finish editing before removing an account" : undefined
          }
          onClick={onDelete}
        >
          {removePending ? "Removing…" : "Remove"}
        </Button>
      </div>
    </article>
  );
}
