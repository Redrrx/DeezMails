import {
  Button,
  Field,
  Input,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
} from "@fluentui/react-components";
import { type FormEvent, useState } from "react";
import { api } from "../api";
import { useUI } from "../store";

export function UnlockScreen() {
  const unlock = useUI((state) => state.unlock);
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError("");
    setPending(true);
    try {
      await api.verifyAccessToken(token);
      unlock(token);
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : "Could not unlock DeezMails",
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <main className="unlock-shell">
      <form className="unlock-card" onSubmit={submit}>
        <h1>DeezMails Login</h1>
        <Field label="Access token">
          <Input
            type="password"
            value={token}
            onChange={(_, data) => setToken(data.value)}
            autoComplete="current-password"
            autoFocus
          />
        </Field>
        <Button appearance="primary" type="submit" disabled={!token || pending}>
          {pending ? "Logging in…" : "Login"}
        </Button>
        {error && (
          <MessageBar intent="error">
            <MessageBarBody>
              <MessageBarTitle>Access denied</MessageBarTitle>
              {error}
            </MessageBarBody>
          </MessageBar>
        )}
      </form>
    </main>
  );
}
