import { Spinner } from "@fluentui/react-components";
import type { ReactNode } from "react";

export function LoadingState() {
  return (
    <div className="loading-state">
      <Spinner label="Loading" labelPosition="below" />
    </div>
  );
}

export function EmptyState({
  icon,
  title,
  text,
}: {
  icon: ReactNode;
  title: string;
  text: string;
}) {
  return (
    <div className="empty-state">
      <div>{icon}</div>
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}
