import { zodResolver } from "@hookform/resolvers/zod";
import {
  Button,
  Dropdown,
  Field,
  Input,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
  Option,
} from "@fluentui/react-components";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { api } from "../api";
import { EmptyState, ErrorState, LoadingState } from "../components/states";
import type { Proxy } from "../models";

const schema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Give the proxy a name")
    .max(200, "Name is too long"),
  type: z.enum(["http", "https", "socks5"]),
  host: z
    .string()
    .trim()
    .min(1, "Host is required")
    .max(253, "Host is too long"),
  port: z.number().int().min(1).max(65535),
  username: z.string().trim().max(1024, "Username is too long").optional(),
  password: z
    .string()
    .max(64 * 1024, "Password is too long")
    .optional(),
});
type FormValues = z.infer<typeof schema>;

export function ProxiesView() {
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Proxy>();
  const proxies = useQuery({
    queryKey: ["proxies"],
    queryFn: ({ signal }) => api.proxies(signal),
  });
  const save = useMutation({
    mutationFn: ({ id, data }: { id?: number; data: FormValues }) =>
      id ? api.updateProxy(id, data) : api.createProxy(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["proxies"] });
      setShowForm(false);
      setEditing(undefined);
    },
  });
  const remove = useMutation({
    mutationFn: api.deleteProxy,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["proxies"] }),
  });
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { type: "http", port: 8080 },
  });
  useEffect(() => {
    form.reset({
      name: editing?.name ?? "",
      type: editing?.type ?? "http",
      host: editing?.host ?? "",
      port: editing?.port ?? 8080,
      username: editing?.username ?? "",
      password: "",
    });
  }, [editing, form]);
  const closeForm = () => {
    setShowForm(false);
    setEditing(undefined);
    save.reset();
  };
  return (
    <section>
      <div className="page-intro">
        <div>
          <h2>Proxies</h2>
          <p>
            Store each proxy as discrete connection fields. Saved passwords
            never return to the dashboard.
          </p>
        </div>
        <Button
          appearance="primary"
          onClick={() => {
            if (showForm) {
              closeForm();
              return;
            }
            setEditing(undefined);
            save.reset();
            setShowForm(true);
          }}
        >
          {showForm ? "Close" : "Add proxy"}
        </Button>
      </div>
      {showForm && (
        <div
          className="form-panel"
          role="region"
          aria-labelledby="proxy-form-title"
        >
          <h3 id="proxy-form-title">{editing ? "Edit proxy" : "Add proxy"}</h3>
          {editing && <p>Leave password blank to keep the saved credential.</p>}
          <form
            className="form-grid"
            onSubmit={form.handleSubmit((data) =>
              save.mutate({ id: editing?.id, data }),
            )}
          >
            <Field
              label="Name"
              validationMessage={form.formState.errors.name?.message}
              validationState={form.formState.errors.name ? "error" : "none"}
            >
              <Input
                placeholder="EU gateway"
                autoComplete="off"
                autoFocus
                {...form.register("name")}
              />
            </Field>
            <Field label="Type">
              <Dropdown
                value={form.watch("type").toUpperCase()}
                selectedOptions={[form.watch("type")]}
                onOptionSelect={(_, data) =>
                  form.setValue("type", data.optionValue as FormValues["type"])
                }
              >
                <Option value="http">HTTP</Option>
                <Option value="https">HTTPS</Option>
                <Option value="socks5">SOCKS5</Option>
              </Dropdown>
            </Field>
            <Field
              label="Host"
              validationMessage={form.formState.errors.host?.message}
              validationState={form.formState.errors.host ? "error" : "none"}
            >
              <Input
                placeholder="proxy.example.com"
                autoComplete="off"
                {...form.register("host")}
              />
            </Field>
            <Field
              label="Port"
              validationMessage={form.formState.errors.port?.message}
              validationState={form.formState.errors.port ? "error" : "none"}
            >
              <Input
                type="number"
                min={1}
                max={65535}
                step={1}
                inputMode="numeric"
                {...form.register("port", { valueAsNumber: true })}
              />
            </Field>
            <Field
              label="Username"
              validationMessage={form.formState.errors.username?.message}
              validationState={
                form.formState.errors.username ? "error" : "none"
              }
            >
              <Input autoComplete="username" {...form.register("username")} />
            </Field>
            <Field
              label="Password"
              validationMessage={form.formState.errors.password?.message}
              validationState={
                form.formState.errors.password ? "error" : "none"
              }
            >
              <Input
                type="password"
                autoComplete="current-password"
                {...form.register("password")}
              />
            </Field>
            <div className="actions">
              <Button
                appearance="primary"
                type="submit"
                disabled={save.isPending}
              >
                {save.isPending
                  ? "Saving..."
                  : editing
                    ? "Save changes"
                    : "Save proxy"}
              </Button>
              <Button appearance="secondary" type="button" onClick={closeForm}>
                Cancel
              </Button>
            </div>
          </form>
          {save.error && (
            <MessageBar intent="error">
              <MessageBarBody>
                <MessageBarTitle>Could not save proxy</MessageBarTitle>
                {save.error.message}
              </MessageBarBody>
            </MessageBar>
          )}
        </div>
      )}
      {proxies.isLoading ? (
        <LoadingState />
      ) : proxies.isError ? (
        <ErrorState
          title="Could not load proxies"
          error={proxies.error}
          onRetry={() => proxies.refetch()}
        />
      ) : !proxies.data?.length ? (
        <EmptyState
          title="No proxies saved"
          text="Add a proxy before assigning one to an account."
        />
      ) : (
        <div className="proxy-list">
          {proxies.data.map((proxy) => (
            <article className="proxy-card" key={proxy.id}>
              <div>
                <h3>{proxy.name}</h3>
                <p>
                  {proxy.type.toUpperCase()} ·{" "}
                  {proxy.username ? `${proxy.username}@` : ""}
                  {proxy.host}:{proxy.port}
                </p>
              </div>
              <Button
                size="small"
                appearance="subtle"
                disabled={showForm || remove.isPending}
                onClick={() => {
                  save.reset();
                  setEditing(proxy);
                  setShowForm(true);
                }}
              >
                Edit
              </Button>
              <Button
                size="small"
                appearance="subtle"
                className="danger-button"
                disabled={showForm || remove.isPending}
                onClick={() => {
                  if (
                    confirm(
                      `Delete ${proxy.name}? Assigned accounts will use a direct connection.`,
                    )
                  )
                    remove.mutate(proxy.id);
                }}
              >
                {remove.isPending && remove.variables === proxy.id
                  ? "Removing…"
                  : "Remove"}
              </Button>
            </article>
          ))}
        </div>
      )}
      {remove.error && (
        <MessageBar intent="error">
          <MessageBarBody>
            <MessageBarTitle>Could not remove proxy</MessageBarTitle>
            {remove.error.message}
          </MessageBarBody>
        </MessageBar>
      )}
    </section>
  );
}
