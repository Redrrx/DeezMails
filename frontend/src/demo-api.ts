import type {
  Account,
  AccountInput,
  AccountUpdate,
  Email,
  Folder,
  JobRun,
  JobSummary,
  Page,
  Proxy,
  ProxyInput,
  RuntimeInfo,
} from "./models";

const now = "2026-09-26T12:00:00Z";

let proxies: Proxy[] = [
  {
    id: 1,
    name: "EU tunnel",
    type: "socks5",
    host: "proxy.demo.invalid",
    port: 1080,
    username: "demo",
    createdAt: "2026-09-12T09:00:00Z",
  },
  {
    id: 2,
    name: "Office gateway",
    type: "https",
    host: "gateway.demo.invalid",
    port: 8443,
    username: "",
    createdAt: "2026-09-18T14:30:00Z",
  },
];

let accounts: Account[] = [
  {
    id: 1,
    email: "alex@demo.example",
    provider: "gmail",
    clientId: "demo-google-client",
    incomingHost: "",
    incomingPort: 0,
    tlsMode: "implicit_tls",
    folder: "all",
    enabled: true,
    syncEnabled: true,
    status: "connected",
    lastSyncedAt: "2026-09-26T11:42:00Z",
    lastCheckedAt: "2026-09-26T11:42:00Z",
    lastError: "",
    createdAt: "2026-09-02T08:00:00Z",
  },
  {
    id: 2,
    email: "finance@demo.example",
    provider: "microsoft",
    clientId: "demo-microsoft-client",
    incomingHost: "",
    incomingPort: 0,
    tlsMode: "implicit_tls",
    folder: "all",
    proxyId: 1,
    proxy: proxies[0],
    enabled: true,
    syncEnabled: true,
    status: "reconnect_required",
    lastCheckedAt: "2026-09-26T11:56:00Z",
    lastError: "Microsoft authorization expired. Reconnect the account.",
    lastErrorCode: "authentication_failed",
    lastErrorAction: "reconnect",
    createdAt: "2026-09-07T10:15:00Z",
  },
  {
    id: 3,
    email: "support@demo.example",
    provider: "imap",
    clientId: "",
    incomingHost: "imap.demo.invalid",
    incomingPort: 993,
    tlsMode: "implicit_tls",
    folder: "INBOX",
    enabled: true,
    syncEnabled: true,
    status: "connected",
    lastSyncedAt: "2026-09-26T11:18:00Z",
    lastCheckedAt: "2026-09-26T11:18:00Z",
    lastError: "",
    createdAt: "2026-09-14T16:20:00Z",
  },
  {
    id: 4,
    email: "archive@demo.example",
    provider: "pop3",
    clientId: "",
    incomingHost: "pop.demo.invalid",
    incomingPort: 995,
    tlsMode: "implicit_tls",
    folder: "INBOX",
    proxyId: 2,
    proxy: proxies[1],
    enabled: false,
    syncEnabled: false,
    status: "disconnected",
    lastError: "",
    createdAt: "2026-09-20T13:10:00Z",
  },
];

let emails: Email[] = [
  {
    id: 1,
    accountId: 1,
    remoteId: "demo-message-001",
    folder: "INBOX",
    threadId: "demo-thread-001",
    from: "Compiler <compiler@localhost.invalid>",
    to: "alex@demo.example",
    subject: "There are only two hard things",
    preview: "Cache invalidation, naming things, and off-by-one errors.",
    body: "Cache invalidation, naming things, and off-by-one errors. Yes, that is three things.",
    bodyTruncated: false,
    receivedAt: "2026-09-26T11:18:00Z",
    isRead: false,
  },
  {
    id: 2,
    accountId: 1,
    remoteId: "demo-message-002",
    folder: "INBOX",
    threadId: "demo-thread-002",
    from: "Ops <ops@localhost.invalid>",
    to: "alex@demo.example",
    subject: "It works on my machine",
    preview: "Then we will ship your machine.",
    body: "Then we will ship your machine. Please leave it plugged in until Friday.",
    bodyTruncated: false,
    receivedAt: "2026-09-26T09:00:00Z",
    isRead: true,
  },
  {
    id: 3,
    accountId: 1,
    remoteId: "demo-message-003",
    folder: "SENT",
    threadId: "demo-thread-003",
    from: "alex@demo.example",
    to: "world@localhost.invalid",
    subject: "Hello, world",
    preview:
      "Any sufficiently advanced bug is indistinguishable from a feature.",
    body: "Any sufficiently advanced bug is indistinguishable from a feature.",
    bodyTruncated: false,
    receivedAt: "2026-09-25T10:00:00Z",
    isRead: true,
  },
  {
    id: 4,
    accountId: 2,
    remoteId: "demo-message-004",
    folder: "archive",
    threadId: "demo-thread-004",
    from: "Bug Counter <bugs@localhost.invalid>",
    to: "finance@demo.example",
    subject: "99 little bugs in the code",
    preview: "Take one down, patch it around—127 little bugs in the code.",
    body: "Take one down, patch it around—127 little bugs in the code.",
    bodyTruncated: false,
    receivedAt: "2026-09-24T12:00:00Z",
    isRead: true,
  },
  {
    id: 5,
    accountId: 3,
    remoteId: "INBOX:42",
    folder: "INBOX",
    threadId: "",
    from: "Rubber Duck <duck@localhost.invalid>",
    to: "support@demo.example",
    subject: "Have you tried explaining it out loud?",
    preview: "I found the bug before you reached the second sentence.",
    body: "I found the bug before you reached the second sentence. Quack.",
    bodyTruncated: false,
    receivedAt: "2026-09-26T08:30:00Z",
    isRead: false,
  },
  {
    id: 6,
    accountId: 3,
    remoteId: "Archive:7",
    folder: "Archive",
    threadId: "",
    from: "Git <git@localhost.invalid>",
    to: "support@demo.example",
    subject: "Detached HEAD seeks meaningful relationship",
    preview: "Commitment issues detected.",
    body: "Detached HEAD seeks meaningful relationship. Commitment issues detected.",
    bodyTruncated: false,
    receivedAt: "2026-09-23T17:45:00Z",
    isRead: true,
  },
];

let jobs: JobRun[] = [
  {
    id: 3,
    accountId: 1,
    accountEmail: "alex@demo.example",
    type: "sync_account",
    status: "succeeded",
    attempts: 1,
    fetched: 3,
    created: 2,
    updated: 1,
    synced: 3,
    error: "",
    queueId: "demo-3",
    folder: "all",
    logs: "2026-09-26T11:41:00Z Queued\n2026-09-26T11:42:00Z Completed successfully",
    startedAt: "2026-09-26T11:41:02Z",
    endedAt: "2026-09-26T11:42:00Z",
    createdAt: "2026-09-26T11:41:00Z",
    updatedAt: "2026-09-26T11:42:00Z",
  },
  {
    id: 2,
    accountId: 2,
    accountEmail: "finance@demo.example",
    type: "verify_connection",
    status: "failed",
    attempts: 1,
    fetched: 0,
    created: 0,
    updated: 0,
    synced: 0,
    error: "Microsoft authorization expired. Reconnect the account.",
    errorCode: "authentication_failed",
    errorAction: "reconnect",
    queueId: "demo-2",
    folder: "",
    logs: "2026-09-26T11:56:00Z Authorization expired. Reconnect required.",
    startedAt: "2026-09-26T11:55:58Z",
    endedAt: "2026-09-26T11:56:00Z",
    createdAt: "2026-09-26T11:55:57Z",
    updatedAt: "2026-09-26T11:56:00Z",
  },
  {
    id: 1,
    accountId: 3,
    accountEmail: "support@demo.example",
    type: "sync_account",
    status: "succeeded",
    attempts: 1,
    fetched: 2,
    created: 2,
    updated: 0,
    synced: 2,
    error: "",
    queueId: "demo-1",
    folder: "all",
    logs: "2026-09-26T11:17:00Z Queued\n2026-09-26T11:18:00Z Completed successfully",
    startedAt: "2026-09-26T11:17:03Z",
    endedAt: "2026-09-26T11:18:00Z",
    createdAt: "2026-09-26T11:17:00Z",
    updatedAt: "2026-09-26T11:18:00Z",
  },
];

let nextAccountID = 5;
let nextProxyID = 3;
let nextJobID = 4;

function clone<T>(value: T): T {
  return structuredClone(value);
}

function findAccount(id: number): Account {
  const account = accounts.find((item) => item.id === id);
  if (!account) throw new Error("Account not found");
  return account;
}

function completedJob(account: Account, type: string, folder = ""): JobRun {
  const id = nextJobID++;
  const job: JobRun = {
    id,
    accountId: account.id,
    accountEmail: account.email,
    type,
    status: "succeeded",
    attempts: 1,
    fetched:
      type === "sync_account"
        ? emails.filter((email) => email.accountId === account.id).length
        : 0,
    created: 0,
    updated:
      type === "sync_account"
        ? emails.filter((email) => email.accountId === account.id).length
        : 0,
    synced:
      type === "sync_account"
        ? emails.filter((email) => email.accountId === account.id).length
        : 0,
    error: "",
    queueId: `demo-${id}`,
    folder,
    logs: `${now} Mock job completed in this browser tab.`,
    startedAt: now,
    endedAt: now,
    createdAt: now,
    updatedAt: now,
  };
  jobs.unshift(job);
  return job;
}

function summary(job: JobRun): JobSummary {
  return {
    id: job.id,
    accountId: job.accountId,
    accountEmail: job.accountEmail ?? "",
    type: job.type,
    status: job.status,
    attempts: job.attempts,
    fetched: job.fetched,
    created: job.created,
    updated: job.updated,
    synced: job.synced,
    error: job.error,
    errorCode: job.errorCode,
    errorAction: job.errorAction,
    startedAt: job.startedAt,
    nextAttemptAt: job.nextAttemptAt,
    endedAt: job.endedAt,
    createdAt: job.createdAt,
    updatedAt: job.updatedAt,
  };
}

export const demoApi = {
  runtime: async (_signal?: AbortSignal): Promise<RuntimeInfo> => ({
    mode: "demo",
    authRequired: false,
  }),
  accounts: async (_signal?: AbortSignal): Promise<Account[]> =>
    clone(accounts),
  verifyAccessToken: async (_accessToken: string): Promise<Account[]> =>
    clone(accounts),
  createAccount: async (data: AccountInput): Promise<Account> => {
    const account: Account = {
      id: nextAccountID++,
      email: data.email,
      provider: data.provider,
      clientId: data.clientId ?? "",
      incomingHost: data.incomingHost ?? "",
      incomingPort: data.incomingPort ?? 0,
      tlsMode: data.tlsMode ?? "implicit_tls",
      folder: data.folder ?? "INBOX",
      proxyId: data.proxyId,
      proxy: proxies.find((proxy) => proxy.id === data.proxyId),
      enabled: true,
      syncEnabled: true,
      status: "connected",
      lastCheckedAt: now,
      lastError: "",
      createdAt: now,
    };
    accounts.unshift(account);
    completedJob(account, "verify_connection");
    return clone(account);
  },
  updateAccount: async (id: number, data: AccountUpdate): Promise<Account> => {
    const account = findAccount(id);
    if (data.email !== undefined) account.email = data.email;
    if (data.provider !== undefined) account.provider = data.provider;
    if (data.clientId !== undefined) account.clientId = data.clientId;
    if (data.incomingHost !== undefined)
      account.incomingHost = data.incomingHost;
    if (data.incomingPort !== undefined)
      account.incomingPort = data.incomingPort;
    if (data.tlsMode !== undefined) account.tlsMode = data.tlsMode;
    if (data.folder !== undefined) account.folder = data.folder;
    if (data.enabled !== undefined) account.enabled = data.enabled;
    if (data.syncEnabled !== undefined) account.syncEnabled = data.syncEnabled;
    if (data.proxyId !== undefined) {
      account.proxyId = data.proxyId || undefined;
      account.proxy = proxies.find((proxy) => proxy.id === data.proxyId);
    }
    account.lastCheckedAt = now;
    return clone(account);
  },
  deleteAccount: async (id: number): Promise<void> => {
    accounts = accounts.filter((account) => account.id !== id);
    emails = emails.filter((email) => email.accountId !== id);
    jobs = jobs.filter((job) => job.accountId !== id);
  },
  reconnect: async (id: number): Promise<JobRun> => {
    const account = findAccount(id);
    account.status = "connected";
    account.lastError = "";
    account.lastErrorCode = undefined;
    account.lastErrorAction = undefined;
    account.lastCheckedAt = now;
    return clone(completedJob(account, "verify_connection"));
  },
  sync: async (id: number): Promise<JobRun> => {
    const account = findAccount(id);
    account.status = "connected";
    account.lastSyncedAt = now;
    account.lastCheckedAt = now;
    return clone(completedJob(account, "sync_account", "all"));
  },
  folders: async (
    accountID: number,
    _signal?: AbortSignal,
  ): Promise<Folder[]> => {
    const names = new Set(
      emails
        .filter((email) => email.accountId === accountID)
        .map((email) => email.folder),
    );
    if (names.size === 0) names.add("INBOX");
    return [...names].map((name) => ({ id: name, name }));
  },
  proxies: async (_signal?: AbortSignal): Promise<Proxy[]> => clone(proxies),
  createProxy: async (data: ProxyInput): Promise<Proxy> => {
    const proxy: Proxy = {
      id: nextProxyID++,
      name: data.name,
      type: data.type,
      host: data.host,
      port: data.port,
      username: data.username ?? "",
      createdAt: now,
    };
    proxies.unshift(proxy);
    return clone(proxy);
  },
  updateProxy: async (id: number, data: ProxyInput): Promise<Proxy> => {
    const proxy = proxies.find((item) => item.id === id);
    if (!proxy) throw new Error("Proxy not found");
    Object.assign(proxy, {
      name: data.name,
      type: data.type,
      host: data.host,
      port: data.port,
      username: data.username ?? "",
    });
    for (const account of accounts.filter((item) => item.proxyId === id)) {
      account.proxy = proxy;
    }
    return clone(proxy);
  },
  deleteProxy: async (id: number): Promise<void> => {
    proxies = proxies.filter((proxy) => proxy.id !== id);
    for (const account of accounts.filter((item) => item.proxyId === id)) {
      account.proxyId = undefined;
      account.proxy = undefined;
    }
  },
  emails: async (
    accountID: number,
    params: URLSearchParams,
    _signal?: AbortSignal,
  ): Promise<Page<Email>> => {
    const page = Math.max(Number(params.get("page")) || 1, 1);
    const pageSize = Math.max(Number(params.get("pageSize")) || 50, 1);
    const folder = params.get("folder") ?? "all";
    const query = (params.get("q") ?? "").toLowerCase();
    const filtered = emails
      .filter((email) => email.accountId === accountID)
      .filter((email) => folder === "all" || email.folder === folder)
      .filter(
        (email) =>
          !query ||
          `${email.from} ${email.subject} ${email.preview}`
            .toLowerCase()
            .includes(query),
      )
      .sort((left, right) => right.receivedAt.localeCompare(left.receivedAt));
    const offset = (page - 1) * pageSize;
    return clone({
      items: filtered.slice(offset, offset + pageSize),
      page,
      pageSize,
      total: filtered.length,
    });
  },
  email: async (
    accountID: number,
    id: string,
    _signal?: AbortSignal,
  ): Promise<Email> => {
    const email = emails.find(
      (item) => item.accountId === accountID && item.remoteId === id,
    );
    if (!email) throw new Error("Message not found");
    return clone(email);
  },
  rawEmail: async (accountID: number, id: string): Promise<Blob> => {
    const email = emails.find(
      (item) => item.accountId === accountID && item.remoteId === id,
    );
    if (!email) throw new Error("Message not found");
    return new Blob(
      [
        `From: ${email.from}\r\nTo: ${email.to}\r\nSubject: ${email.subject}\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n${email.body}\r\n`,
      ],
      { type: "message/rfc822" },
    );
  },
  jobs: async (_signal?: AbortSignal): Promise<JobSummary[]> =>
    clone(jobs.map(summary)),
  clearJobs: async (): Promise<{ deleted: number }> => {
    const deleted = jobs.filter(
      (job) => job.status === "succeeded" || job.status === "failed",
    ).length;
    jobs = jobs.filter(
      (job) => job.status !== "succeeded" && job.status !== "failed",
    );
    return { deleted };
  },
  job: async (id: number, _signal?: AbortSignal): Promise<JobRun> => {
    const job = jobs.find((item) => item.id === id);
    if (!job) throw new Error("Job not found");
    return clone({
      ...job,
      account: accounts.find((account) => account.id === job.accountId),
    });
  },
  retryJob: async (id: number): Promise<JobRun> => {
    const previous = jobs.find((job) => job.id === id);
    if (!previous?.accountId) throw new Error("Job not found");
    return clone(
      completedJob(
        findAccount(previous.accountId),
        previous.type,
        previous.folder,
      ),
    );
  },
};
