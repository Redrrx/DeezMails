import { create } from "zustand";

type View = "accounts" | "proxies" | "mail" | "jobs" | "settings";
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
  view: "accounts",
  accessToken: sessionStorage.getItem("deezmails-access-token") ?? "",
  notificationsMuted:
    localStorage.getItem("deezmails-notifications-muted") === "true",
  select: (view, selectedAccountID) => set({ view, selectedAccountID }),
  unlock: (accessToken) => {
    sessionStorage.setItem("deezmails-access-token", accessToken);
    set({ accessToken });
  },
  lock: () => {
    sessionStorage.removeItem("deezmails-access-token");
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
