export type Provider = "gmail" | "microsoft" | "imap" | "pop3";
export type Proxy = {
  id: number;
  name: string;
  type: "http" | "https" | "socks5";
  host: string;
  port: number;
  username: string;
  createdAt: string;
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
  createdAt: string;
};
export type Email = {
  id: number;
  accountId: number;
  remoteId: string;
  threadId: string;
  from: string;
  to: string;
  subject: string;
  preview: string;
  body: string;
  receivedAt: string;
  isRead: boolean;
};
export type Folder = { id: string; name: string };
export type JobRun = {
  id: number;
  queueId: string;
  accountId?: number;
  account?: Account;
  type: string;
  folder: string;
  status: "queued" | "running" | "retrying" | "succeeded" | "failed";
  attempts: number;
  fetched: number;
  created: number;
  updated: number;
  synced: number;
  error: string;
  logs: string;
  startedAt?: string;
  nextAttemptAt?: string;
  endedAt?: string;
  createdAt: string;
};
export type Page<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
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
export type AccountUpdate = Partial<AccountInput> & { enabled?: boolean };

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const accessToken = sessionStorage.getItem("deezmails-access-token");
  const response = await fetch(path, {
    headers: {
      ...(options?.body ? { "Content-Type": "application/json" } : {}),
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      ...options?.headers,
    },
    ...options,
  });
  if (response.status === 204) return undefined as T;
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || "Request failed");
  return body;
}

async function requestBlob(path: string): Promise<Blob> {
  const accessToken = sessionStorage.getItem("deezmails-access-token");
  const response = await fetch(path, {
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || "Request failed");
  }
  return response.blob();
}

export const api = {
  accounts: () => request<Account[]>("/api/accounts"),
  account: (id: number) => request<Account>(`/api/accounts/${id}`),
  createAccount: (data: AccountInput) =>
    request<Account>("/api/accounts", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateAccount: (id: number, data: AccountUpdate) =>
    request<Account>(`/api/accounts/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  deleteAccount: (id: number) =>
    request<void>(`/api/accounts/${id}`, { method: "DELETE" }),
  reconnect: async (id: number) => {
    const result = await request<{ url?: string } | JobRun>(
      `/api/accounts/${id}/reconnect`,
      { method: "POST" },
    );
    if ("url" in result && result.url) window.location.assign(result.url);
    return result;
  },
  sync: (id: number) =>
    request<JobRun>(`/api/accounts/${id}/sync`, { method: "POST" }),
  folders: (accountID: number) =>
    request<Folder[]>(`/api/accounts/${accountID}/folders`),
  proxies: () => request<Proxy[]>("/api/proxies"),
  createProxy: (data: {
    name: string;
    type: "http" | "https" | "socks5";
    host: string;
    port: number;
    username?: string;
    password?: string;
  }) =>
    request<Proxy>("/api/proxies", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateProxy: (
    id: number,
    data: {
      name: string;
      type: "http" | "https" | "socks5";
      host: string;
      port: number;
      username?: string;
      password?: string;
    },
  ) =>
    request<Proxy>(`/api/proxies/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  deleteProxy: (id: number) =>
    request<void>(`/api/proxies/${id}`, { method: "DELETE" }),
  emails: (accountID: number, params: URLSearchParams) =>
    request<Page<Email>>(`/api/accounts/${accountID}/emails?${params}`),
  email: (accountID: number, id: string) =>
    request<Email>(
      `/api/accounts/${accountID}/emails/${encodeURIComponent(id)}`,
    ),
  rawEmail: (accountID: number, id: string) =>
    requestBlob(
      `/api/accounts/${accountID}/emails/${encodeURIComponent(id)}?format=raw`,
    ),
  jobs: () => request<JobRun[]>("/api/jobs"),
  clearJobs: () =>
    request<{ deleted: number }>("/api/jobs", { method: "DELETE" }),
  job: (id: number) => request<JobRun>(`/api/jobs/${id}`),
  retryJob: (id: number) =>
    request<JobRun>(`/api/jobs/${id}/retry`, { method: "POST" }),
};
