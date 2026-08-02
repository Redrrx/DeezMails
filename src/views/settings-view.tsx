import { Switch } from "@fluentui/react-components";
import { useUI } from "../store";

export function SettingsView() {
  const notificationsMuted = useUI((state) => state.notificationsMuted);
  const toggleNotifications = useUI((state) => state.toggleNotifications);
  return (
    <section>
      <div className="page-intro">
        <div>
          <h2>Settings</h2>
          <p>Choose how DeezMails keeps you informed while you work.</p>
        </div>
      </div>
      <div className="settings-panel">
        <div>
          <h3>Job alerts</h3>
          <p>
            Show an in-app alert when a mailbox sync or connection check
            completes or fails.
          </p>
        </div>
        <Switch
          checked={!notificationsMuted}
          label={notificationsMuted ? "Off" : "On"}
          onChange={toggleNotifications}
        />
      </div>
    </section>
  );
}
