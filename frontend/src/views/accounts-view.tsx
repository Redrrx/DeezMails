import {
  Button,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
} from "@fluentui/react-components";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../api";
import { ErrorState, EmptyState, LoadingState } from "../components/states";
import { AccountCard } from "../features/accounts/account-card";
import { AccountForm } from "../features/accounts/account-form";
import {
  activeJobStatuses,
  type Account,
  type AccountInput,
  type AccountUpdate,
} from "../models";
import { useUI } from "../store";

export function AccountsView() {
  const queryClient = useQueryClient();
  const selectView = useUI((state) => state.select);
  const [showForm, setShowForm] = useState(false);
  const [editingAccount, setEditingAccount] = useState<Account>();
  const accounts = useQuery({
    queryKey: ["accounts"],
    queryFn: ({ signal }) => api.accounts(signal),
  });
  const proxies = useQuery({
    queryKey: ["proxies"],
    queryFn: ({ signal }) => api.proxies(signal),
  });
  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: ({ signal }) => api.jobs(signal),
  });
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["accounts"] });
  const refreshAfterJob = () => {
    refresh();
    queryClient.invalidateQueries({ queryKey: ["jobs"] });
  };
  const reconnect = useMutation({
    mutationFn: (account: Account) => api.reconnect(account.id),
    onSuccess: refreshAfterJob,
  });
  const create = useMutation({
    mutationFn: api.createAccount,
    onSuccess: (account) => {
      refresh();
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
      setShowForm(false);
      setEditingAccount(undefined);
      if (
        (account.provider === "gmail" || account.provider === "microsoft") &&
        account.status !== "connected"
      )
        reconnect.mutate(account);
    },
  });
  const update = useMutation({
    mutationFn: ({ id, data }: { id: number; data: AccountInput }) =>
      api.updateAccount(id, data),
    onSuccess: () => {
      refresh();
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
      setShowForm(false);
      setEditingAccount(undefined);
    },
  });
  const sync = useMutation({
    mutationFn: api.sync,
    onSuccess: refreshAfterJob,
  });
  const updateState = useMutation({
    mutationFn: ({ id, data }: { id: number; data: AccountUpdate }) =>
      api.updateAccount(id, data),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: api.deleteAccount,
    onSuccess: refresh,
  });

  const closeForm = () => {
    setShowForm(false);
    setEditingAccount(undefined);
    create.reset();
    update.reset();
  };

  return (
    <section>
      <div className="page-intro">
        <div>
          <h2>Accounts</h2>
          <p>
            Use Gmail or Microsoft OAuth, or configure any domain with IMAP or
            POP3. Sync covers all available folders by default.
          </p>
        </div>
        <Button
          appearance="primary"
          onClick={() => {
            if (showForm) {
              closeForm();
              return;
            }
            create.reset();
            update.reset();
            setEditingAccount(undefined);
            setShowForm(true);
          }}
        >
          {showForm ? "Close" : "Add account"}
        </Button>
      </div>
      {(reconnect.error || sync.error || remove.error || updateState.error) && (
        <MessageBar intent="error">
          <MessageBarBody>
            <MessageBarTitle>Operation failed</MessageBarTitle>
            {reconnect.error?.message ||
              sync.error?.message ||
              remove.error?.message ||
              updateState.error?.message}
          </MessageBarBody>
        </MessageBar>
      )}
      {showForm && proxies.isError && (
        <ErrorState
          title="Could not load proxies"
          error={proxies.error}
          onRetry={() => proxies.refetch()}
        />
      )}
      {jobs.isError && (
        <ErrorState
          title="Could not load active jobs"
          error={jobs.error}
          onRetry={() => jobs.refetch()}
        />
      )}
      {showForm && (
        <AccountForm
          key={editingAccount?.id ?? "new"}
          account={editingAccount}
          proxies={proxies.data ?? []}
          pending={create.isPending || update.isPending}
          error={create.error?.message || update.error?.message}
          onCancel={closeForm}
          onSubmit={(data) => {
            if (editingAccount) {
              update.mutate({ id: editingAccount.id, data });
              return;
            }
            create.mutate(data);
          }}
        />
      )}
      {accounts.isLoading ? (
        <LoadingState />
      ) : accounts.isError ? (
        <ErrorState
          title="Could not load accounts"
          error={accounts.error}
          onRetry={() => accounts.refetch()}
        />
      ) : !accounts.data?.length ? (
        <EmptyState
          title="No accounts connected"
          text="Add Gmail, Microsoft, IMAP, or POP3 account settings to begin."
        />
      ) : (
        <div className="account-list">
          {accounts.data.map((account) => (
            <AccountCard
              key={account.id}
              account={account}
              formOpen={showForm}
              activeJob={jobs.data?.find(
                (job) =>
                  job.accountId === account.id &&
                  activeJobStatuses.has(job.status),
              )}
              syncPending={sync.isPending && sync.variables === account.id}
              reconnectPending={
                reconnect.isPending && reconnect.variables?.id === account.id
              }
              statePending={
                updateState.isPending &&
                updateState.variables?.id === account.id
              }
              removePending={
                remove.isPending && remove.variables === account.id
              }
              mutationPending={
                reconnect.isPending ||
                sync.isPending ||
                updateState.isPending ||
                remove.isPending
              }
              onReconnect={() => reconnect.mutate(account)}
              onSync={() => sync.mutate(account.id)}
              onOpen={() => selectView("mail", account.id)}
              onEdit={() => {
                create.reset();
                update.reset();
                setEditingAccount(account);
                setShowForm(true);
              }}
              onToggleEnabled={() =>
                updateState.mutate({
                  id: account.id,
                  data: { enabled: !account.enabled },
                })
              }
              onToggleSync={() =>
                updateState.mutate({
                  id: account.id,
                  data: { syncEnabled: !account.syncEnabled },
                })
              }
              onDelete={() => {
                if (confirm(`Remove ${account.email}?`))
                  remove.mutate(account.id);
              }}
            />
          ))}
        </div>
      )}
    </section>
  );
}
