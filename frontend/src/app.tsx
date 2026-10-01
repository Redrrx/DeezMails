import { lazy, Suspense, useEffect, useRef } from "react";
import { useUI } from "./store";
import {
  FluentProvider,
  Spinner,
  webDarkTheme,
} from "@fluentui/react-components";
import { Header } from "./components/header";
import { UnlockScreen } from "./components/unlock-screen";
import { LiveUpdates } from "./components/live-updates";
import { ErrorState } from "./components/states";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";

const AccountsView = lazy(() =>
  import("./views/accounts-view").then(({ AccountsView }) => ({
    default: AccountsView,
  })),
);
const ProxiesView = lazy(() =>
  import("./views/proxies-view").then(({ ProxiesView }) => ({
    default: ProxiesView,
  })),
);
const MailboxView = lazy(() =>
  import("./views/mailbox-view").then(({ MailboxView }) => ({
    default: MailboxView,
  })),
);
const JobsView = lazy(() =>
  import("./views/jobs-view").then(({ JobsView }) => ({ default: JobsView })),
);
const SettingsView = lazy(() =>
  import("./views/settings-view").then(({ SettingsView }) => ({
    default: SettingsView,
  })),
);

export default function App() {
  const view = useUI((state) => state.view);
  const accessToken = useUI((state) => state.accessToken);
  const mainContent = useRef<HTMLElement>(null);
  const previousView = useRef(view);
  const runtime = useQuery({
    queryKey: ["runtime"],
    queryFn: ({ signal }) => api.runtime(signal),
    retry: 1,
    staleTime: Infinity,
  });
  useEffect(() => {
    if (previousView.current === view) return;
    previousView.current = view;
    mainContent.current?.focus();
  }, [view]);
  if (runtime.isLoading)
    return (
      <FluentProvider theme={webDarkTheme}>
        <main className="unlock-shell">
          <Spinner label="Starting DeezMails…" />
        </main>
      </FluentProvider>
    );
  if (runtime.isError)
    return (
      <FluentProvider theme={webDarkTheme}>
        <main className="unlock-shell">
          <div className="unlock-card">
            <ErrorState
              title="Could not reach the DeezMails backend"
              error={runtime.error}
              onRetry={() => runtime.refetch()}
            />
          </div>
        </main>
      </FluentProvider>
    );
  const runtimeInfo = runtime.data;
  if (!runtimeInfo) return null;
  if (runtimeInfo.authRequired && !accessToken)
    return (
      <FluentProvider theme={webDarkTheme}>
        <UnlockScreen />
      </FluentProvider>
    );
  return (
    <FluentProvider theme={webDarkTheme}>
      <div className="app-shell">
        <a className="skip-link" href="#main-content">
          Skip to content
        </a>
        <Header
          authRequired={runtimeInfo.authRequired}
          mode={runtimeInfo.mode}
        />
        {runtimeInfo.mode === "demo" && (
          <div className="demo-notice" role="status">
            Static showcase. Everything is mock data and resets when this tab
            reloads.
          </div>
        )}
        <LiveUpdates />
        <main
          ref={mainContent}
          className="app-content"
          id="main-content"
          tabIndex={-1}
        >
          <Suspense fallback={<Spinner label="Loading view…" />}>
            {view === "accounts" && <AccountsView />}
            {view === "proxies" && <ProxiesView />}
            {view === "mail" && <MailboxView />}
            {view === "jobs" && <JobsView />}
            {view === "settings" && <SettingsView />}
          </Suspense>
        </main>
      </div>
    </FluentProvider>
  );
}
