import { Button } from "@fluentui/react-components";
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

export function Header() {
  const { view, select, lock } = useUI();
  return (
    <header className="app-header">
      <div className="app-header__inner">
        <h1>DeezMails</h1>
        <nav aria-label="Primary navigation">
          {navigation.map(({ key, label, icon: Icon }) => (
            <Button
              key={key}
              appearance={view === key ? "primary" : "subtle"}
              icon={<Icon />}
              onClick={() => select(key)}
            >
              {label}
            </Button>
          ))}
          <Button
            appearance="subtle"
            icon={<BookOpenRegular />}
            as="a"
            href="/swagger/index.html"
            target="_blank"
          >
            API docs
          </Button>
          <Button
            appearance={view === "settings" ? "primary" : "subtle"}
            icon={<SettingsRegular />}
            aria-label="Settings"
            title="Settings"
            onClick={() => select("settings")}
          />
          <Button appearance="subtle" icon={<SignOutRegular />} onClick={lock}>
            Logout
          </Button>
        </nav>
      </div>
    </header>
  );
}
