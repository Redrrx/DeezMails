import { useUI } from "./store";
import { FluentProvider, webDarkTheme } from "@fluentui/react-components";
import { Header } from "./components/header";
import { AccountsView } from "./views/accounts-view";
import { MailboxView } from "./views/mailbox-view";
import { ProxiesView } from "./views/proxies-view";
import { JobsView } from "./views/jobs-view";
import { SettingsView } from "./views/settings-view";
import { UnlockScreen } from "./components/unlock-screen";
import { LiveUpdates } from "./components/live-updates";

export default function App() {
  const view = useUI((state) => state.view);
  const accessToken = useUI((state) => state.accessToken);
  if (!accessToken)
    return (
      <FluentProvider theme={webDarkTheme}>
        <UnlockScreen />
      </FluentProvider>
    );
  return (
    <FluentProvider theme={webDarkTheme}>
      <main className="app-shell">
        <Header />
        <LiveUpdates />
        <div className="app-content">
          {view === "accounts" && <AccountsView />}
          {view === "proxies" && <ProxiesView />}
          {view === "mail" && <MailboxView />}
          {view === "jobs" && <JobsView />}
          {view === "settings" && <SettingsView />}
        </div>
      </main>
    </FluentProvider>
  );
}
