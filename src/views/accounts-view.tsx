import { zodResolver } from "@hookform/resolvers/zod";
import {
  Badge,
  Button,
  Dropdown,
  Field,
  Input,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
  Option,
  Spinner,
} from "@fluentui/react-components";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import {
  api,
  type Account,
  type AccountInput,
  type JobRun,
  type Proxy,
} from "../api";
import { EmptyState, LoadingState } from "../components/states";
import { useUI } from "../store";

const formSchema = z.object({
  email: z.email("Enter a valid email address"),
  password: z.string().optional(),
  provider: z.enum(["gmail", "microsoft", "imap", "pop3"]),
  clientId: z.string().optional(),
  refreshToken: z.string().optional(),
  incomingHost: z.string().optional(),
  incomingPort: z.string().optional(),
  tlsMode: z.enum(["implicit_tls", "starttls", "none"]),
  proxyId: z.string().optional(),
});
type FormValues = z.infer<typeof formSchema>;

const accountSchema = (canKeepPassword: boolean) =>
  formSchema.superRefine((data, context) => {
    if (
      (data.provider === "gmail" || data.provider === "microsoft") &&
      !data.clientId
    )
      context.addIssue({
        code: "custom",
        path: ["clientId"],
        message: "Client ID is required for OAuth providers",
      });
    if (
      (data.provider === "imap" || data.provider === "pop3") &&
      (!data.incomingHost ||
        !Number(data.incomingPort) ||
        (!canKeepPassword && !data.password))
    )
      context.addIssue({
        code: "custom",
        path: ["incomingHost"],
        message: "Host, port, and password are required for IMAP/POP3",
      });
  });

export function AccountsView() {
  const queryClient = useQueryClient();
  const selectView = useUI((state) => state.select);
  const [showForm, setShowForm] = useState(false);
  const [editingAccount, setEditingAccount] = useState<Account>();
  const accounts = useQuery({ queryKey: ["accounts"], queryFn: api.accounts });
  const proxies = useQuery({ queryKey: ["proxies"], queryFn: api.proxies });
  const jobs = useQuery({ queryKey: ["jobs"], queryFn: api.jobs });
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["accounts"] });
  const refreshAfterJob = () => {
    refresh();
    queryClient.invalidateQueries({ queryKey: ["jobs"] });
  };
  const reconnect = useMutation({
    mutationFn: (account: Account) => api.reconnect(account.id),
    onSuccess: refreshAfterJob,
  });
  const create = useMutation({
    mutationFn: api.createAccount,
    onSuccess: (account) => {
      refresh();
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
      setShowForm(false);
      setEditingAccount(undefined);
      if (
        (account.provider === "gmail" || account.provider === "microsoft") &&
        account.status !== "connected"
      ) {
        reconnect.mutate(account);
      }
    },
  });
  const update = useMutation({
    mutationFn: ({ id, data }: { id: number; data: AccountInput }) =>
      api.updateAccount(id, data),
    onSuccess: () => {
      refresh();
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
      setShowForm(false);
      setEditingAccount(undefined);
    },
  });
  const sync = useMutation({
    mutationFn: api.sync,
    onSuccess: refreshAfterJob,
  });
  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      api.updateAccount(id, { enabled }),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: api.deleteAccount,
    onSuccess: refresh,
  });
  return (
    <section>
      <div className="page-intro">
        <div>
          <h2>Accounts</h2>
          <p>
            Use Gmail or Microsoft OAuth, or configure any domain with IMAP or
            POP3. Sync covers all available folders by default.
          </p>
        </div>
        <Button
          appearance="primary"
          onClick={() => {
            if (showForm) {
              setShowForm(false);
              setEditingAccount(undefined);
              return;
            }
            setEditingAccount(undefined);
            setShowForm(true);
          }}
        >
          {showForm ? "Close" : "Add account"}
        </Button>
      </div>
      {(reconnect.error || sync.error || remove.error || toggle.error) && (
        <MessageBar intent="error">
          <MessageBarBody>
            <MessageBarTitle>Operation failed</MessageBarTitle>
            {reconnect.error?.message ||
              sync.error?.message ||
              remove.error?.message ||
              toggle.error?.message}
          </MessageBarBody>
        </MessageBar>
      )}
      {showForm && (
        <AccountForm
          key={editingAccount?.id ?? "new"}
          account={editingAccount}
          proxies={proxies.data ?? []}
          pending={create.isPending || update.isPending}
          error={create.error?.message || update.error?.message}
          onCancel={() => {
            setShowForm(false);
            setEditingAccount(undefined);
          }}
          onSubmit={(data) => {
            if (editingAccount) {
              update.mutate({ id: editingAccount.id, data });
              return;
            }
            create.mutate(data);
          }}
        />
      )}
      {accounts.isLoading ? (
        <LoadingState />
      ) : !accounts.data?.length ? (
        <EmptyState
          icon={null}
          title="No accounts connected"
          text="Add Gmail, Microsoft, IMAP, or POP3 account settings to begin."
        />
      ) : (
        <div className="account-list">
          {accounts.data.map((account) => (
            <AccountCard
              key={account.id}
              account={account}
              formOpen={showForm}
              activeJob={jobs.data?.find(
                (job) =>
                  job.accountId === account.id &&
                  (job.status === "queued" ||
                    job.status === "running" ||
                    job.status === "retrying"),
              )}
              syncPending={sync.isPending && sync.variables === account.id}
              reconnectPending={
                reconnect.isPending && reconnect.variables?.id === account.id
              }
              onReconnect={() => reconnect.mutate(account)}
              onSync={() => sync.mutate(account.id)}
              onOpen={() => selectView("mail", account.id)}
              onEdit={() => {
                setEditingAccount(account);
                setShowForm(true);
              }}
              onToggle={() =>
                toggle.mutate({ id: account.id, enabled: !account.enabled })
              }
              onDelete={() => {
                if (confirm(`Remove ${account.email}?`))
                  remove.mutate(account.id);
              }}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function AccountForm({
  account,
  proxies,
  pending,
  error,
  onCancel,
  onSubmit,
}: {
  account?: Account;
  proxies: Proxy[];
  pending: boolean;
  error?: string;
  onCancel: () => void;
  onSubmit: (data: AccountInput) => void;
}) {
  const form = useForm<FormValues>({
    resolver: zodResolver(
      accountSchema(
        account?.provider === "imap" || account?.provider === "pop3",
      ),
    ),
    defaultValues: {
      email: account?.email ?? "",
      provider: account?.provider ?? "gmail",
      clientId: account?.clientId ?? "",
      incomingHost: account?.incomingHost ?? "",
      incomingPort: account?.incomingPort ? String(account.incomingPort) : "",
      tlsMode: (account?.tlsMode as FormValues["tlsMode"]) ?? "implicit_tls",
      proxyId: account?.proxyId ? String(account.proxyId) : "",
    },
  });
  const provider = form.watch("provider");
  const oauth = provider === "gmail" || provider === "microsoft";
  return (
    <div className="form-panel">
      <h3>{account ? "Edit mailbox" : "Add mailbox"}</h3>
      <p>
        {account
          ? "Leave a password or refresh token blank to keep the saved credential."
          : "Fields adapt to the protocol. Saving validates the mailbox and starts its first sync automatically."}
      </p>
      <form
        className="form-grid form-grid--account"
        onSubmit={form.handleSubmit((data) =>
          onSubmit({
            email: data.email,
            provider: data.provider,
            password: oauth ? undefined : data.password || undefined,
            clientId: oauth ? data.clientId || undefined : undefined,
            refreshToken: oauth ? data.refreshToken || undefined : undefined,
            incomingHost: oauth ? undefined : data.incomingHost || undefined,
            incomingPort:
              oauth || !data.incomingPort
                ? undefined
                : Number(data.incomingPort),
            tlsMode: oauth ? undefined : data.tlsMode,
            proxyId: data.proxyId
              ? Number(data.proxyId)
              : account
                ? 0
                : undefined,
          }),
        )}
      >
        <Field
          label="Mailbox login"
          validationMessage={form.formState.errors.email?.message}
          validationState={form.formState.errors.email ? "error" : "none"}
        >
          <Input placeholder="team@example.com" {...form.register("email")} />
        </Field>
        <Field label="Provider">
          <Dropdown
            value={
              {
                gmail: "Gmail OAuth",
                microsoft: "Microsoft OAuth",
                imap: "Custom IMAP",
                pop3: "Custom POP3",
              }[provider]
            }
            selectedOptions={[provider]}
            onOptionSelect={(_, data) =>
              form.setValue(
                "provider",
                data.optionValue as FormValues["provider"],
              )
            }
          >
            <Option value="gmail">Gmail OAuth</Option>
            <Option value="microsoft">Microsoft OAuth</Option>
            <Option value="imap">Custom IMAP</Option>
            <Option value="pop3">Custom POP3</Option>
          </Dropdown>
        </Field>
        {oauth ? (
          <>
            <Field
              label="OAuth client ID"
              validationMessage={form.formState.errors.clientId?.message}
              validationState={
                form.formState.errors.clientId ? "error" : "none"
              }
            >
              <Input {...form.register("clientId")} />
            </Field>
            <Field
              label={
                account
                  ? "Refresh token (leave blank to keep saved)"
                  : "Refresh token (optional)"
              }
            >
              <Input type="password" {...form.register("refreshToken")} />
            </Field>
          </>
        ) : (
          <>
            <Field
              label={
                account
                  ? "Mailbox password (leave blank to keep saved)"
                  : "Mailbox password"
              }
            >
              <Input type="password" {...form.register("password")} />
            </Field>
            <Field
              label="Incoming host"
              validationMessage={form.formState.errors.incomingHost?.message}
              validationState={
                form.formState.errors.incomingHost ? "error" : "none"
              }
            >
              <Input
                placeholder="mail.example.com"
                {...form.register("incomingHost")}
              />
            </Field>
            <Field label="Incoming port">
              <Input
                placeholder={provider === "imap" ? "993" : "995"}
                {...form.register("incomingPort")}
              />
            </Field>
            <Field label="Connection security">
              <Dropdown
                value={
                  {
                    implicit_tls: "SSL/TLS",
                    starttls: "STARTTLS",
                    none: "None",
                  }[form.watch("tlsMode")]
                }
                selectedOptions={[form.watch("tlsMode")]}
                onOptionSelect={(_, data) =>
                  form.setValue(
                    "tlsMode",
                    data.optionValue as FormValues["tlsMode"],
                  )
                }
              >
                <Option value="implicit_tls">SSL/TLS</Option>
                <Option value="starttls">STARTTLS</Option>
                <Option value="none">None</Option>
              </Dropdown>
            </Field>
          </>
        )}
        <Field label="Proxy">
          <Dropdown
            value={
              form.watch("proxyId")
                ? proxies.find(
                    (proxy) => String(proxy.id) === form.watch("proxyId"),
                  )?.name
                : "Direct connection"
            }
            selectedOptions={[form.watch("proxyId") ?? ""]}
            onOptionSelect={(_, data) =>
              form.setValue("proxyId", data.optionValue ?? "")
            }
          >
            <Option value="">Direct connection</Option>
            {proxies.map((proxy) => (
              <Option key={proxy.id} value={String(proxy.id)}>
                {proxy.name}
              </Option>
            ))}
          </Dropdown>
        </Field>
        <div className="actions">
          <Button appearance="primary" type="submit" disabled={pending}>
            {pending ? "Saving…" : account ? "Save changes" : "Save account"}
          </Button>
          <Button appearance="secondary" type="button" onClick={onCancel}>
            Cancel
          </Button>
        </div>
      </form>
      {error && (
        <MessageBar intent="error">
          <MessageBarBody>
            <MessageBarTitle>Could not save account</MessageBarTitle>
            {error}
          </MessageBarBody>
        </MessageBar>
      )}
    </div>
  );
}

function AccountCard({
  account,
  formOpen,
  activeJob,
  reconnectPending,
  syncPending,
  onReconnect,
  onSync,
  onOpen,
  onEdit,
  onToggle,
  onDelete,
}: {
  account: Account;
  formOpen: boolean;
  activeJob?: JobRun;
  reconnectPending: boolean;
  syncPending: boolean;
  onReconnect: () => void;
  onSync: () => void;
  onOpen: () => void;
  onEdit: () => void;
  onToggle: () => void;
  onDelete: () => void;
}) {
  const connected = account.status === "connected";
  const working = Boolean(activeJob) || reconnectPending || syncPending;
  const controlsLocked = formOpen || working;
  const actionLabel =
    activeJob?.status === "retrying"
      ? `Retrying (attempt ${activeJob.attempts})`
      : activeJob?.type === "verify_connection"
        ? "Checking connection..."
        : activeJob?.type === "sync_account"
          ? "Syncing..."
          : "";
  const label =
    account.provider === "imap"
      ? `IMAP ${account.incomingHost}:${account.incomingPort}`
      : account.provider === "pop3"
        ? `POP3 ${account.incomingHost}:${account.incomingPort}`
        : account.provider === "gmail"
          ? "Gmail"
          : "Microsoft";
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
        </div>
        <p className="account-meta">
          {label} · {account.proxy?.name ?? "Direct connection"} ·{" "}
          {account.lastSyncedAt
            ? `Synced ${new Date(account.lastSyncedAt).toLocaleString()}`
            : "Not synced"}
        </p>
        {account.lastError && (
          <p className="account-error">{account.lastError}</p>
        )}
      </div>
      <div className="actions">
        {!connected && (
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
        {connected && (
          <Button
            size="small"
            appearance="primary"
            disabled={!account.enabled || controlsLocked}
            icon={working ? <Spinner size="tiny" /> : undefined}
            onClick={onSync}
          >
            {actionLabel || (syncPending ? "Syncing..." : "Sync all")}
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
          onClick={onToggle}
        >
          {account.enabled ? "Disable" : "Enable"}
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
          Remove
        </Button>
      </div>
    </article>
  );
}
