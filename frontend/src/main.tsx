import React from "react";
import ReactDOM from "react-dom/client";
import {
  MutationCache,
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import App from "./app";
import { ApiError } from "./api";
import { AppErrorBoundary } from "./components/app-error-boundary";
import { useUI } from "./store";
import "./styles.css";

let queryClient: QueryClient;
const handleApiError = (error: unknown) => {
  if (
    !(error instanceof ApiError) ||
    error.status !== 401 ||
    error.code !== "access_token_required"
  )
    return;
  queryClient.clear();
  useUI.getState().lock();
};

queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: handleApiError }),
  mutationCache: new MutationCache({ onError: handleApiError }),
  defaultOptions: {
    queries: {
      retry: (failureCount, error) =>
        error instanceof ApiError && error.code === "access_token_required"
          ? false
          : failureCount < 1,
      refetchOnWindowFocus: true,
      refetchIntervalInBackground: false,
    },
  },
});
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <AppErrorBoundary>
        <App />
      </AppErrorBoundary>
    </QueryClientProvider>
  </React.StrictMode>,
);
