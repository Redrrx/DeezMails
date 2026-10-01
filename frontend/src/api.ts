import type {
  Account,
  AccountInput,
  AccountUpdate,
  Email,
  Folder,
  JobRun,
  JobSummary,
  MailErrorAction,
  Page,
  Proxy,
  ProxyInput,
  RuntimeInfo,
} from "./models";
import { demoApi } from "./demo-api";

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string,
    readonly action?: MailErrorAction,
    readonly retryAfterSeconds?: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type RequestOptions = RequestInit & {
  accessToken?: string;
};

async function authorizedFetch(
  path: string,
  options: RequestOptions = {},
): Promise<Response> {
  const { accessToken: suppliedToken, ...requestInit } = options;
  const accessToken =
    suppliedToken ?? sessionStorage.getItem("deezmails-access-token");
  const response = await fetch(path, {
    ...requestInit,
    headers: {
      ...(requestInit.body ? { "Content-Type": "application/json" } : {}),
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      ...requestInit.headers,
    },
  });
  if (response.ok) return response;
  const body: {
    error?: unknown;
    code?: unknown;
    action?: unknown;
    retryAfterSeconds?: unknown;
  } = await response.json().catch(() => ({}));
  throw new ApiError(
    typeof body.error === "string" ? body.error : "Request failed",
    response.status,
    typeof body.code === "string" ? body.code : undefined,
    typeof body.action === "string"
      ? (body.action as MailErrorAction)
      : undefined,
    typeof body.retryAfterSeconds === "number"
      ? body.retryAfterSeconds
      : undefined,
  );
}

async function request<T>(path: string, options?: RequestOptions): Promise<T> {
  const response = await authorizedFetch(path, options);
  if (response.status === 204) return undefined as T;
  return response.json();
}

async function requestBlob(path: string): Promise<Blob> {
  const response = await authorizedFetch(path);
  return response.blob();
}

const liveApi = {
  runtime: (signal?: AbortSignal) =>
    request<RuntimeInfo>("/api/runtime", { signal }),
  accounts: (signal?: AbortSignal) =>
    request<Account[]>("/api/accounts", { signal }),
  verifyAccessToken: (accessToken: string) =>
    request<Account[]>("/api/accounts", { accessToken }),
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
  folders: (accountID: number, signal?: AbortSignal) =>
    request<Folder[]>(`/api/accounts/${accountID}/folders`, { signal }),
  proxies: (signal?: AbortSignal) =>
    request<Proxy[]>("/api/proxies", { signal }),
  createProxy: (data: ProxyInput) =>
    request<Proxy>("/api/proxies", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateProxy: (id: number, data: ProxyInput) =>
    request<Proxy>(`/api/proxies/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  deleteProxy: (id: number) =>
    request<void>(`/api/proxies/${id}`, { method: "DELETE" }),
  emails: (accountID: number, params: URLSearchParams, signal?: AbortSignal) =>
    request<Page<Email>>(`/api/accounts/${accountID}/emails?${params}`, {
      signal,
    }),
  email: (accountID: number, id: string, signal?: AbortSignal) =>
    request<Email>(
      `/api/accounts/${accountID}/emails/${encodeURIComponent(id)}`,
      { signal },
    ),
  rawEmail: (accountID: number, id: string) =>
    requestBlob(
      `/api/accounts/${accountID}/emails/${encodeURIComponent(id)}?format=raw`,
    ),
  jobs: (signal?: AbortSignal) =>
    request<JobSummary[]>("/api/jobs/summary", { signal }),
  clearJobs: () =>
    request<{ deleted: number }>("/api/jobs", { method: "DELETE" }),
  job: (id: number, signal?: AbortSignal) =>
    request<JobRun>(`/api/jobs/${id}`, { signal }),
  retryJob: (id: number) =>
    request<JobRun>(`/api/jobs/${id}/retry`, { method: "POST" }),
};

export type Api = typeof liveApi;
export const isStaticDemo = import.meta.env.VITE_STATIC_DEMO === "true";
export const api: Api = isStaticDemo ? demoApi : liveApi;
