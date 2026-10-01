import { Button } from "@fluentui/react-components";
import { useQueryClient } from "@tanstack/react-query";
import {
  BookOpenRegular,
  ClipboardTaskListLtrRegular,
  MailRegular,
  PlugConnectedRegular,
  PeopleRegular,
  SignOutRegular,
  SettingsRegular,
} from "@fluentui/react-icons";
import { useUI } from "../store";

const navigation = [
  { key: "accounts" as const, label: "Accounts", icon: PeopleRegular },
  { key: "proxies" as const, label: "Proxies", icon: PlugConnectedRegular },
  { key: "mail" as const, label: "Mailbox", icon: MailRegular },
  { key: "jobs" as const, label: "Jobs", icon: ClipboardTaskListLtrRegular },
];

export function Header({
  authRequired,
  mode,
}: {
  authRequired: boolean;
  mode: "demo" | "production";
}) {
  const { view, select, lock } = useUI();
  const queryClient = useQueryClient();
  return (
    <header className="app-header">
      <div className="app-header__inner">
        <h1>DeezMails{mode === "demo" ? " Demo" : ""}</h1>
        <nav aria-label="Primary navigation">
          {navigation.map(({ key, label, icon: Icon }) => (
            <Button
              key={key}
              appearance={view === key ? "primary" : "subtle"}
              icon={<Icon />}
              aria-current={view === key ? "page" : undefined}
              onClick={() => select(key)}
            >
              {label}
            </Button>
          ))}
          <Button
            appearance="subtle"
            icon={<BookOpenRegular />}
            as="a"
            href={
              mode === "demo"
                ? `${import.meta.env.BASE_URL}swagger/index.html`
                : "/swagger/index.html"
            }
            target="_blank"
            rel="noopener noreferrer"
          >
            API docs
          </Button>
          <Button
            appearance={view === "settings" ? "primary" : "subtle"}
            icon={<SettingsRegular />}
            aria-label="Settings"
            aria-current={view === "settings" ? "page" : undefined}
            title="Settings"
            onClick={() => select("settings")}
          />
          {authRequired && (
            <Button
              appearance="subtle"
              icon={<SignOutRegular />}
              onClick={() => {
                queryClient.clear();
                lock();
              }}
            >
              Logout
            </Button>
          )}
        </nav>
      </div>
    </header>
  );
}
