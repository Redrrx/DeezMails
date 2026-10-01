import { Component, type ErrorInfo, type ReactNode } from "react";

export class AppErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("DeezMails UI failed", error, info);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <main className="unlock-shell">
        <div className="unlock-card" role="alert">
          <h1>DeezMails stopped rendering</h1>
          <p>Reload the dashboard. No server data was changed.</p>
          <button
            className="recovery-button"
            type="button"
            onClick={() => window.location.reload()}
          >
            Reload dashboard
          </button>
        </div>
      </main>
    );
  }
}
