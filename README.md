# DeezMails

OAuth-connected Gmail and Microsoft mailbox manager. One Google OAuth app and one Microsoft Entra app are shared by every mailbox; each mailbox stores its own OAuth tokens and optional proxy assignment.

```powershell
Copy-Item .env.example .env
$env:GMAIL_CLIENT_SECRET = "..."
$env:MICROSOFT_CLIENT_SECRET = "..."
$env:CREDENTIALS_ENCRYPTION_KEY = "..."
$env:DEEZMAILS_ACCESS_TOKEN = "..."
go mod tidy
go run .
```

SQLite is the default (`deezmails.db`). Set `DATABASE_URL` to a PostgreSQL URL for Postgres.

Generate a persistent encryption key once and keep it in your deployment secret manager:

```powershell
[Convert]::ToBase64String([byte[]](1..32 | ForEach-Object { Get-Random -Maximum 256 }))
```

Every account supplies its own OAuth client ID. The server still needs the matching provider client secret for confidential OAuth clients.

## Access and durable jobs

Set `DEEZMAILS_ACCESS_TOKEN` to a random string of at least 32 characters. The dashboard asks for it once per browser session, and direct API calls must send it as `Authorization: Bearer <token>`.

Mailbox sync and connection checks are durable database jobs powered by goqite. SQLite is the default for a single-process deployment; set `DATABASE_URL` to use the same job system on PostgreSQL. No Redis is required. `WORKER_CONCURRENCY` defaults to `1` on SQLite and `4` on PostgreSQL; `SYNC_INTERVAL_MINUTES` defaults to `15`. The scheduler takes due accounts in batches (`SCHEDULER_BATCH_SIZE=50`) rather than starting every account at once. Transient network failures retry with a short exponential delay; `JOB_MAX_ATTEMPTS` defaults to `3` and `JOB_RETRY_BASE_SECONDS` defaults to `15`.

Each proxy works for OAuth provider calls, IMAP, and POP3. HTTP/HTTPS proxies use CONNECT for mail protocols and SOCKS5 proxies support optional username/password authentication. Provider calls retry transient failures with backoff. Message/raw responses are limited to 64 MiB by default; raise `MAX_MESSAGE_BYTES` when a deployment needs larger raw messages. `SYNC_MAX_PAGES` controls how many 100-message provider pages a sync examines (default `5`).

## Dashboard

In a second terminal:

```powershell
npm install
npm run dev
```

Open `http://localhost:5173`. Vite proxies API and OAuth requests to the Go API on port 8080. For a single production server, run `npm run build`; the Go server will serve `dist/` at `http://localhost:8080`.

## API documentation

With the Go server running, open `http://localhost:8080/swagger/index.html` for interactive Swagger UI. The OpenAPI document is generated from the Go route annotations in `main.go`. After changing an annotated route or model, run:

```powershell
go generate ./...
```

Register `http://localhost:8080/oauth/callback` with your Google and Microsoft OAuth apps for local development. Use your production API URL for deployment.

Important routes:

- `POST /api/accounts` – create account metadata
- `GET /api/accounts` – list accounts
- `POST /api/accounts/:id/reconnect` – reconnect through OAuth or queue a direct-mail connection check
- `POST /api/accounts/:id/sync` – queue a mailbox sync
- `GET /api/jobs` – inspect durable job history
- `POST /api/jobs/:id/retry` – retry a failed job
- `GET /api/accounts/:id/emails` – paginated, sortable messages
- `GET /api/accounts/:id/emails/:messageID?format=json|raw` – message detail
