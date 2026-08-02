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
import { useDeferredValue, useState } from "react";
import { api } from "../api";
import { EmptyState, LoadingState } from "../components/states";
import { useUI } from "../store";

export function MailboxView() {
  const queryClient = useQueryClient();
  const { selectedAccountID, select } = useUI();
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [folder, setFolder] = useState("all");
  const [selectedMessageID, setSelectedMessageID] = useState<string>();
  const accounts = useQuery({ queryKey: ["accounts"], queryFn: api.accounts });
  const accountID = selectedAccountID ?? accounts.data?.[0]?.id;
  const account = accounts.data?.find((item) => item.id === accountID);
  const folders = useQuery({
    queryKey: ["folders", accountID],
    queryFn: () => api.folders(accountID!),
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
    ...(deferredQuery ? { q: deferredQuery } : {}),
  });
  const messages = useQuery({
    queryKey: ["emails", accountID, page, deferredQuery, folder],
    queryFn: () => api.emails(accountID!, params),
    enabled: !!accountID,
  });
  const detail = useQuery({
    queryKey: ["email", accountID, selectedMessageID],
    queryFn: () => api.email(accountID!, selectedMessageID!),
    enabled: !!accountID && !!selectedMessageID,
  });
  const refresh = useMutation({
    mutationFn: (id: number) => api.sync(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["jobs"] }),
  });
  if (!accounts.data?.length)
    return (
      <EmptyState
        icon={null}
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
            !accountID || account?.status !== "connected" || refresh.isPending
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
            value={query}
            onChange={(_, data) => {
              setQuery(data.value);
              setPage(1);
            }}
          />
        </Field>
      </div>
      <div className="mail-layout">
        <MessageList
          loading={messages.isLoading}
          data={messages.data}
          page={page}
          selectedMessageID={selectedMessageID}
          onPageChange={setPage}
          onSelect={setSelectedMessageID}
        />
        <MessagePreview
          loading={detail.isLoading}
          message={detail.data}
          error={detail.error?.message}
          accountID={accountID}
        />
      </div>
    </section>
  );
}

function MessageList({
  loading,
  data,
  page,
  selectedMessageID,
  onPageChange,
  onSelect,
}: {
  loading: boolean;
  data: Awaited<ReturnType<typeof api.emails>> | undefined;
  page: number;
  selectedMessageID?: string;
  onPageChange: (page: number) => void;
  onSelect: (id: string) => void;
}) {
  if (loading)
    return (
      <div className="mail-panel">
        <LoadingState />
      </div>
    );
  return (
    <div className="mail-panel message-list">
      {!data?.items.length ? (
        <p className="empty-copy">No synced messages found.</p>
      ) : (
        data.items.map((message) => (
          <button
            key={message.remoteId}
            className={`message-row ${selectedMessageID === message.remoteId ? "message-row--active" : ""}`}
            onClick={() => onSelect(message.remoteId)}
          >
            <div className="message-row__top">
              <strong>{message.from || "Unknown sender"}</strong>
              <span className="message-row__date">
                {new Date(message.receivedAt).toLocaleDateString()}
              </span>
            </div>
            <p className="message-row__subject">
              {message.subject || "(No subject)"}
            </p>
            <p className="message-row__preview">{message.preview}</p>
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
}: {
  loading: boolean;
  message: Awaited<ReturnType<typeof api.email>> | undefined;
  error?: string;
  accountID?: number;
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
        <p>{error ?? "Choose a message to read it."}</p>
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
            link.download = `${message.subject || "message"}.eml`;
            link.click();
            URL.revokeObjectURL(url);
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
