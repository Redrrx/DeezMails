export type Provider = "gmail" | "microsoft" | "imap" | "pop3";

export type MailErrorAction =
  "none" | "retry" | "reconnect" | "edit_account" | "contact_admin";

export type Proxy = {
  id: number;
  name: string;
  type: "http" | "https" | "socks5";
  host: string;
  port: number;
  username: string;
  createdAt: string;
};

export type ProxyInput = {
  name: string;
  type: Proxy["type"];
  host: string;
  port: number;
  username?: string;
  password?: string;
};

export type Account = {
  id: number;
  email: string;
  provider: Provider;
  clientId: string;
  incomingHost: string;
  incomingPort: number;
  tlsMode: string;
  folder: string;
  proxyId?: number;
  proxy?: Proxy;
  enabled: boolean;
  syncEnabled: boolean;
  status: string;
  lastSyncedAt?: string;
  lastCheckedAt?: string;
  lastError: string;
  lastErrorCode?: string;
  lastErrorAction?: MailErrorAction;
  createdAt: string;
};

export type Email = {
  id: number;
  accountId: number;
  remoteId: string;
  folder: string;
  threadId: string;
  from: string;
  to: string;
  subject: string;
  preview: string;
  body: string;
  bodyTruncated: boolean;
  receivedAt: string;
  isRead: boolean;
};

export type Folder = { id: string; name: string };

export type JobStatus =
  "queued" | "running" | "retrying" | "succeeded" | "failed";

export type JobSummary = {
  id: number;
  accountId?: number;
  accountEmail: string;
  type: string;
  status: JobStatus;
  attempts: number;
  fetched: number;
  created: number;
  updated: number;
  synced: number;
  error: string;
  errorCode?: string;
  errorAction?: MailErrorAction;
  startedAt?: string;
  nextAttemptAt?: string;
  endedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type JobRun = Omit<JobSummary, "accountEmail"> & {
  accountEmail?: string;
  queueId: string;
  folder: string;
  logs: string;
  account?: Account;
};

export const activeJobStatuses: ReadonlySet<JobStatus> = new Set([
  "queued",
  "running",
  "retrying",
]);

export type Page<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
};

export type RuntimeInfo = {
  mode: "demo" | "production";
  authRequired: boolean;
};

export type AccountInput = {
  email: string;
  password?: string;
  provider: Provider;
  clientId?: string;
  refreshToken?: string;
  incomingHost?: string;
  incomingPort?: number;
  tlsMode?: string;
  folder?: string;
  proxyId?: number;
};

export type AccountUpdate = Partial<AccountInput> & {
  enabled?: boolean;
  syncEnabled?: boolean;
};
