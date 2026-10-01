![DeezMails](logo.png)

DeezMails is an advanced email manager for people who use multiple accounts, including marketers and users managing throwaway accounts for privacy or research. It supports Gmail, Microsoft, IMAP, and POP3 mailboxes, with a separate proxy configuration for each account.

[Try the demo](https://redrrx.github.io/DeezMails/)

## How to launch

1. Install dependencies: Linux runs `./install-dependencies-linux.sh`; Windows runs `install-dependencies-windows.cmd`.
2. Copy [backend/.env.example](backend/.env.example) to `.env` and fill in the key and token described below.
3. Start: Linux runs `./start-production-linux.sh`; Windows runs `start-production-windows.cmd`.

Open [http://localhost:8080](http://localhost:8080) and log in with your access token.

## How to use

1. **Log in** with the access token from your environment or `.env` file.
2. **Add a proxy, if needed.** Open **Proxies**, choose **Add proxy**, and enter its type, host, port, and optional credentials. HTTP, HTTPS, and SOCKS5 proxies are supported.
3. **Add a mailbox.** Open **Accounts** and choose **Add account**. For Gmail or Microsoft, enter your OAuth client ID and an optional existing refresh token. For IMAP or POP3, enter the mailbox password, incoming host, port, and connection security.
4. **Assign its proxy.** Select a saved proxy in the mailbox form, or use a direct connection. Save the account and complete the provider authorization when prompted. IMAP and POP3 accounts start a connection check automatically.
5. **Read and sync mail.** Once connected, choose **Mailbox** to browse messages and folders. Use **Sync all** or **Refresh mailbox** to request a sync.
6. **Track activity.** Open **Jobs** to inspect connection checks, sync progress, and errors. Account controls let you reconnect, retry a sync, pause syncing, or disable a mailbox.

### Gmail and Microsoft setup

Accounts purchased or obtained through third-party providers may include an OAuth client ID and refresh token, with a callback URL already configured in their OAuth app. Enter the supplied client ID and refresh token when adding the account.

If you need browser authorization or reconnection, DeezMails' callback must match a URL registered in that app. The local default is `http://localhost:8080/oauth/callback`; use `OAUTH_REDIRECT_URL` to match another registered callback. A preconfigured third-party callback may point to their service rather than your DeezMails instance.

If you use your own OAuth app, register your DeezMails callback with Gmail or Microsoft. Set `GMAIL_CLIENT_SECRET` or `MICROSOFT_CLIENT_SECRET` if the app requires a client secret.

## API

DeezMails includes an HTTP API for accounts, proxies, mail, folders, and sync jobs. Open **API docs** in the app header, or use [local Swagger docs](http://localhost:8080/swagger/index.html). Authenticate API requests with `Authorization: Bearer <DEEZMAILS_ACCESS_TOKEN>`.

The [demo API docs](https://redrrx.github.io/DeezMails/swagger/index.html) show the same specification; API requests require a running DeezMails backend.

## Configuration

Generate an encryption key and an access token:

```sh
bun -e "console.log(Buffer.from(crypto.getRandomValues(new Uint8Array(32))).toString('base64'))"
bun -e "console.log(crypto.randomUUID() + crypto.randomUUID())"
```

Put the first value in `CREDENTIALS_ENCRYPTION_KEY` and the second in `DEEZMAILS_ACCESS_TOKEN` in `.env`. Keep the encryption key when reusing your database. Environment variables override `.env`; `backend/.env` is also supported.

## Docker

Prepare the same `.env` file described above, then build and run:

```sh
docker build -t deezmails .
docker run --rm -p 127.0.0.1:8080:8080 --env-file .env -v deezmails-data:/data deezmails
```

Open [http://localhost:8080](http://localhost:8080) and log in with your access token.

The image builds the frontend with Bun and runs a static Go binary that serves the compiled interface. SQLite data is stored in the `deezmails-data` volume. Leave `SQLITE_PATH` unset, or set it to a path inside `/data`.

To restart, run the same `docker run` command with the same volume and encryption key. Stop the running container with **Ctrl+C**.

## Run the demo locally

The demo uses fake mailbox data and needs only Bun.

Linux:

```sh
./start-demo-linux.sh
```

Windows: double-click `start-demo-windows.cmd`.

Open [http://127.0.0.1:8080](http://127.0.0.1:8080). Changes in the demo are temporary and reset when you reload.
