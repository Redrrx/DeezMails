import {
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
import { useEffect, useState } from "react";
import { api } from "../api";
import { EmptyState, ErrorState, LoadingState } from "../components/states";
import { useUI } from "../store";

export function MailboxView() {
  const queryClient = useQueryClient();
  const { selectedAccountID, select } = useUI();
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [folder, setFolder] = useState("all");
  const [selectedMessageID, setSelectedMessageID] = useState<string>();
  const accounts = useQuery({
    queryKey: ["accounts"],
    queryFn: ({ signal }) => api.accounts(signal),
  });
  const selectedAccount = accounts.data?.find(
    (item) => item.id === selectedAccountID,
  );
  const defaultAccount =
    accounts.data?.find(
      (item) => item.status === "connected" && item.lastSyncedAt,
    ) ?? accounts.data?.[0];
  const account = selectedAccount ?? defaultAccount;
  const accountID = account?.id;

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 300);
    return () => window.clearTimeout(timer);
  }, [query]);
  const folders = useQuery({
    queryKey: ["folders", accountID],
    queryFn: ({ signal }) => api.folders(accountID!, signal),
    enabled: !!accountID,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  });
  const params = new URLSearchParams({
    page: String(page),
    pageSize: "50",
    sort: "receivedAt",
    direction: "desc",
    folder,
    ...(debouncedQuery ? { q: debouncedQuery } : {}),
  });
  const messages = useQuery({
    queryKey: ["emails", accountID, page, debouncedQuery, folder],
    queryFn: ({ signal }) => api.emails(accountID!, params, signal),
    enabled: !!accountID,
  });
  const detail = useQuery({
    queryKey: ["email", accountID, selectedMessageID],
    queryFn: ({ signal }) => api.email(accountID!, selectedMessageID!, signal),
    enabled: !!accountID && !!selectedMessageID,
  });
  const refresh = useMutation({
    mutationFn: (id: number) => api.sync(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["jobs"] }),
  });
  if (accounts.isLoading) return <LoadingState />;
  if (accounts.isError)
    return (
      <ErrorState
        title="Could not load accounts"
        error={accounts.error}
        onRetry={() => accounts.refetch()}
      />
    );
  if (!accounts.data?.length)
    return (
      <EmptyState
        title="Connect an account first"
        text="Your synced inboxes will appear here."
      />
    );
  return (
    <section>
      <div className="page-intro">
        <div>
          <h2>Mailbox</h2>
          <p>
            All folders are shown by default, including Spam or Junk where the
            provider exposes them.
          </p>
        </div>
        <Button
          appearance="primary"
          disabled={
            !accountID ||
            !account?.enabled ||
            !account.syncEnabled ||
            account.status !== "connected" ||
            refresh.isPending
          }
          icon={refresh.isPending ? <Spinner size="tiny" /> : undefined}
          onClick={() => accountID && refresh.mutate(accountID)}
        >
          {refresh.isPending ? "Queueing…" : "Refresh mailbox"}
        </Button>
      </div>
      {refresh.isSuccess && (
        <MessageBar intent="info">
          <MessageBarBody>
            <MessageBarTitle>Mailbox sync queued</MessageBarTitle>
            This view refreshes as soon as the queued sync finishes.
          </MessageBarBody>
        </MessageBar>
      )}
      {refresh.error && (
        <MessageBar intent="error">
          <MessageBarBody>
            <MessageBarTitle>Could not queue mailbox refresh</MessageBarTitle>
            {refresh.error.message}
          </MessageBarBody>
        </MessageBar>
      )}
      {folders.isError && (
        <ErrorState
          title="Could not load mailbox folders"
          error={folders.error}
          onRetry={() => folders.refetch()}
        />
      )}
      <div className="mail-toolbar">
        <Field label="Mailbox">
          <Dropdown
            value={
              accounts.data.find((account) => account.id === accountID)?.email
            }
            selectedOptions={[String(accountID)]}
            onOptionSelect={(_, data) => {
              select("mail", Number(data.optionValue));
              setFolder("all");
              setPage(1);
              setSelectedMessageID(undefined);
            }}
          >
            {accounts.data.map((account) => (
              <Option key={account.id} value={String(account.id)}>
                {account.email}
              </Option>
            ))}
          </Dropdown>
        </Field>
        <Field label="Folder">
          <Dropdown
            value={
              folder === "all"
                ? "All folders"
                : folders.data?.find((item) => item.id === folder)?.name
            }
            selectedOptions={[folder]}
            onOptionSelect={(_, data) => {
              setFolder(data.optionValue ?? "all");
              setPage(1);
              setSelectedMessageID(undefined);
            }}
          >
            <Option value="all">All folders</Option>
            {folders.data?.map((item) => (
              <Option key={item.id} value={item.id}>
                {item.name}
              </Option>
            ))}
          </Dropdown>
        </Field>
        <Field label="Search">
          <Input
            placeholder="Sender, subject, or preview"
            maxLength={200}
            value={query}
            onChange={(_, data) => {
              setQuery(data.value);
              setPage(1);
              setSelectedMessageID(undefined);
            }}
          />
        </Field>
      </div>
      <div className="mail-layout">
        <MessageList
          loading={messages.isLoading}
          error={messages.error}
          data={messages.data}
          page={page}
          selectedMessageID={selectedMessageID}
          onPageChange={(nextPage) => {
            setPage(nextPage);
            setSelectedMessageID(undefined);
          }}
          onSelect={setSelectedMessageID}
          onRetry={() => messages.refetch()}
        />
        <MessagePreview
          key={`${accountID ?? "none"}:${selectedMessageID ?? "none"}`}
          loading={detail.isLoading}
          message={detail.data}
          error={detail.error}
          accountID={accountID}
          onRetry={() => detail.refetch()}
        />
      </div>
    </section>
  );
}

function MessageList({
  loading,
  error,
  data,
  page,
  selectedMessageID,
  onPageChange,
  onSelect,
  onRetry,
}: {
  loading: boolean;
  error: Error | null;
  data: Awaited<ReturnType<typeof api.emails>> | undefined;
  page: number;
  selectedMessageID?: string;
  onPageChange: (page: number) => void;
  onSelect: (id: string) => void;
  onRetry: () => void;
}) {
  if (loading)
    return (
      <div className="mail-panel">
        <LoadingState />
      </div>
    );
  if (error)
    return (
      <div className="mail-panel">
        <ErrorState
          title="Could not load messages"
          error={error}
          onRetry={onRetry}
        />
      </div>
    );
  return (
    <div className="mail-panel message-list">
      {!data?.items.length ? (
        <p className="empty-copy">No synced messages found.</p>
      ) : (
        data.items.map((message) => (
          <button
            type="button"
            key={message.remoteId}
            className={`message-row ${selectedMessageID === message.remoteId ? "message-row--active" : ""}`}
            aria-pressed={selectedMessageID === message.remoteId}
            onClick={() => onSelect(message.remoteId)}
          >
            <span className="message-row__top">
              <strong>{message.from || "Unknown sender"}</strong>
              <span className="message-row__date">
                {new Date(message.receivedAt).toLocaleDateString()}
              </span>
            </span>
            <span className="message-row__subject">
              {message.subject || "(No subject)"}
              {!message.isRead && (
                <span className="message-unread">Unread</span>
              )}
            </span>
            <span className="message-row__preview">{message.preview}</span>
          </button>
        ))
      )}
      <div className="pager">
        <span>{data?.total ?? 0} messages</span>
        <div>
          <Button
            size="small"
            appearance="secondary"
            disabled={page === 1}
            onClick={() => onPageChange(page - 1)}
          >
            Previous
          </Button>
          <Button
            size="small"
            appearance="secondary"
            disabled={!data || page * data.pageSize >= data.total}
            onClick={() => onPageChange(page + 1)}
          >
            Next
          </Button>
        </div>
      </div>
    </div>
  );
}

function MessagePreview({
  loading,
  message,
  error,
  accountID,
  onRetry,
}: {
  loading: boolean;
  message: Awaited<ReturnType<typeof api.email>> | undefined;
  error: Error | null;
  accountID?: number;
  onRetry: () => void;
}) {
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState("");
  if (loading)
    return (
      <div className="message-preview">
        <LoadingState />
      </div>
    );
  if (!message)
    return (
      <div className="message-preview">
        {error ? (
          <ErrorState
            title="Could not load message"
            error={error}
            onRetry={onRetry}
          />
        ) : (
          <p>Choose a message to read it.</p>
        )}
      </div>
    );
  return (
    <article className="message-preview">
      <h3>{message.subject || "(No subject)"}</h3>
      <dl>
        <div>
          <dt>From</dt>
          <dd>{message.from}</dd>
        </div>
        <div>
          <dt>To</dt>
          <dd>{message.to}</dd>
        </div>
      </dl>
      <div className="message-body">
        {message.body ||
          message.preview ||
          "No readable body was returned by this provider."}
      </div>
      {message.bodyTruncated && (
        <MessageBar intent="info">
          <MessageBarBody>
            Display limited for browser safety. Download raw email for full
            content.
          </MessageBarBody>
        </MessageBar>
      )}
      <Button
        appearance="subtle"
        className="raw-link"
        disabled={downloading || !accountID}
        onClick={async () => {
          if (!accountID) return;
          setDownloading(true);
          setDownloadError("");
          try {
            const raw = await api.rawEmail(accountID, message.remoteId);
            const url = URL.createObjectURL(raw);
            const link = document.createElement("a");
            link.href = url;
            const filename = (message.subject || "message")
              .replace(/[\u0000-\u001f<>:"/\\|?*]+/g, "_")
              .trim()
              .slice(0, 120);
            link.download = `${filename || "message"}.eml`;
            document.body.append(link);
            link.click();
            link.remove();
            window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
          } catch (reason) {
            setDownloadError(
              reason instanceof Error
                ? reason.message
                : "Could not download raw email",
            );
          } finally {
            setDownloading(false);
          }
        }}
      >
        {downloading ? "Preparing raw email..." : "Download raw email (.eml)"}
      </Button>
      {downloadError && <p className="account-error">{downloadError}</p>}
    </article>
  );
}
