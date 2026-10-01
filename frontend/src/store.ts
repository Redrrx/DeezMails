import { create } from "zustand";

export type View = "accounts" | "proxies" | "mail" | "jobs" | "settings";

const views: ReadonlySet<string> = new Set([
  "accounts",
  "proxies",
  "mail",
  "jobs",
  "settings",
]);

function locationState(): { view: View; selectedAccountID?: number } {
  const params = new URLSearchParams(window.location.search);
  const requestedView = params.get("view") ?? "accounts";
  const view = views.has(requestedView) ? (requestedView as View) : "accounts";
  const accountID = Number(params.get("account"));
  return {
    view,
    selectedAccountID:
      view === "mail" && Number.isSafeInteger(accountID) && accountID > 0
        ? accountID
        : undefined,
  };
}

function updateLocation(
  view: View,
  selectedAccountID: number | undefined,
  replace = false,
) {
  const url = new URL(window.location.href);
  if (view === "accounts") url.searchParams.delete("view");
  else url.searchParams.set("view", view);
  if (view === "mail" && selectedAccountID)
    url.searchParams.set("account", String(selectedAccountID));
  else url.searchParams.delete("account");
  window.history[replace ? "replaceState" : "pushState"]({}, "", url);
}

type UIStore = {
  view: View;
  selectedAccountID?: number;
  accessToken: string;
  notificationsMuted: boolean;
  select: (view: View, accountID?: number) => void;
  unlock: (accessToken: string) => void;
  lock: () => void;
  toggleNotifications: () => void;
};

export const useUI = create<UIStore>((set) => ({
  ...locationState(),
  accessToken: sessionStorage.getItem("deezmails-access-token") ?? "",
  notificationsMuted:
    localStorage.getItem("deezmails-notifications-muted") === "true",
  select: (view, selectedAccountID) => {
    const current = useUI.getState();
    if (
      current.view === view &&
      current.selectedAccountID === selectedAccountID
    )
      return;
    updateLocation(view, selectedAccountID);
    set({ view, selectedAccountID });
  },
  unlock: (accessToken) => {
    sessionStorage.setItem("deezmails-access-token", accessToken);
    set({ accessToken });
  },
  lock: () => {
    sessionStorage.removeItem("deezmails-access-token");
    updateLocation("accounts", undefined, true);
    set({ accessToken: "", view: "accounts", selectedAccountID: undefined });
  },
  toggleNotifications: () =>
    set((state) => {
      const notificationsMuted = !state.notificationsMuted;
      localStorage.setItem(
        "deezmails-notifications-muted",
        String(notificationsMuted),
      );
      return { notificationsMuted };
    }),
}));

window.addEventListener("popstate", () => useUI.setState(locationState()));
