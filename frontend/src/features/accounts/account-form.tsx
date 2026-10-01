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
import { useForm } from "react-hook-form";
import { z } from "zod";
import type { Account, AccountInput, Proxy } from "../../models";

const formSchema = z.object({
  email: z
    .string()
    .trim()
    .max(320, "Email address is too long")
    .email("Enter a valid email address"),
  password: z
    .string()
    .max(64 * 1024, "Password is too long")
    .optional(),
  provider: z.enum(["gmail", "microsoft", "imap", "pop3"]),
  clientId: z.string().trim().max(4096, "Client ID is too long").optional(),
  refreshToken: z
    .string()
    .max(128 * 1024, "Refresh token is too long")
    .optional(),
  incomingHost: z.string().trim().max(253, "Host is too long").optional(),
  incomingPort: z.string().trim().optional(),
  tlsMode: z.enum(["implicit_tls", "starttls", "none"]),
  proxyId: z.string().optional(),
});

type FormValues = z.infer<typeof formSchema>;

const accountSchema = (canKeepPassword: boolean) =>
  formSchema.superRefine((data, context) => {
    const oauth = data.provider === "gmail" || data.provider === "microsoft";
    if (oauth && !data.clientId)
      context.addIssue({
        code: "custom",
        path: ["clientId"],
        message: "Client ID is required for OAuth providers",
      });
    if (oauth) return;
    if (!data.incomingHost)
      context.addIssue({
        code: "custom",
        path: ["incomingHost"],
        message: "Incoming host is required",
      });
    const port = Number(data.incomingPort);
    if (!Number.isInteger(port) || port < 1 || port > 65535)
      context.addIssue({
        code: "custom",
        path: ["incomingPort"],
        message: "Port must be an integer from 1 to 65535",
      });
    if (!canKeepPassword && !data.password)
      context.addIssue({
        code: "custom",
        path: ["password"],
        message: "Mailbox password is required",
      });
  });

export function AccountForm({
  account,
  proxies,
  pending,
  error,
  onCancel,
  onSubmit,
}: {
  account?: Account;
  proxies: Proxy[];
  pending: boolean;
  error?: string;
  onCancel: () => void;
  onSubmit: (data: AccountInput) => void;
}) {
  const form = useForm<FormValues>({
    resolver: zodResolver(
      accountSchema(
        account?.provider === "imap" || account?.provider === "pop3",
      ),
    ),
    defaultValues: {
      email: account?.email ?? "",
      provider: account?.provider ?? "gmail",
      clientId: account?.clientId ?? "",
      incomingHost: account?.incomingHost ?? "",
      incomingPort: account?.incomingPort ? String(account.incomingPort) : "",
      tlsMode: (account?.tlsMode as FormValues["tlsMode"]) ?? "implicit_tls",
      proxyId: account?.proxyId ? String(account.proxyId) : "",
    },
  });
  const provider = form.watch("provider");
  const oauth = provider === "gmail" || provider === "microsoft";

  return (
    <div
      className="form-panel"
      role="region"
      aria-labelledby="account-form-title"
    >
      <h3 id="account-form-title">
        {account ? "Edit mailbox" : "Add mailbox"}
      </h3>
      <p>
        {account
          ? "Leave a password or refresh token blank to keep the saved credential."
          : "Fields adapt to the protocol. Saving adds the mailbox and starts its first connection check."}
      </p>
      <form
        className="form-grid form-grid--account"
        onSubmit={form.handleSubmit((data) =>
          onSubmit({
            email: data.email,
            provider: data.provider,
            password: oauth ? undefined : data.password || undefined,
            clientId: oauth ? data.clientId || undefined : undefined,
            refreshToken: oauth ? data.refreshToken || undefined : undefined,
            incomingHost: oauth ? undefined : data.incomingHost || undefined,
            incomingPort:
              oauth || !data.incomingPort
                ? undefined
                : Number(data.incomingPort),
            tlsMode: oauth ? undefined : data.tlsMode,
            proxyId: data.proxyId
              ? Number(data.proxyId)
              : account
                ? 0
                : undefined,
          }),
        )}
      >
        <Field
          label="Mailbox login"
          validationMessage={form.formState.errors.email?.message}
          validationState={form.formState.errors.email ? "error" : "none"}
        >
          <Input
            type="email"
            autoComplete="username"
            autoCapitalize="none"
            placeholder="team@example.com"
            autoFocus
            {...form.register("email")}
          />
        </Field>
        <Field label="Provider">
          <Dropdown
            value={
              {
                gmail: "Gmail OAuth",
                microsoft: "Microsoft OAuth",
                imap: "Custom IMAP",
                pop3: "Custom POP3",
              }[provider]
            }
            selectedOptions={[provider]}
            onOptionSelect={(_, data) =>
              form.setValue(
                "provider",
                data.optionValue as FormValues["provider"],
                { shouldValidate: true },
              )
            }
          >
            <Option value="gmail">Gmail OAuth</Option>
            <Option value="microsoft">Microsoft OAuth</Option>
            <Option value="imap">Custom IMAP</Option>
            <Option value="pop3">Custom POP3</Option>
          </Dropdown>
        </Field>
        {oauth ? (
          <>
            <Field
              label="OAuth client ID"
              validationMessage={form.formState.errors.clientId?.message}
              validationState={
                form.formState.errors.clientId ? "error" : "none"
              }
            >
              <Input autoComplete="off" {...form.register("clientId")} />
            </Field>
            <Field
              label={
                account
                  ? "Refresh token (leave blank to keep saved)"
                  : "Refresh token (optional)"
              }
              validationMessage={form.formState.errors.refreshToken?.message}
              validationState={
                form.formState.errors.refreshToken ? "error" : "none"
              }
            >
              <Input
                type="password"
                autoComplete="off"
                {...form.register("refreshToken")}
              />
            </Field>
          </>
        ) : (
          <>
            <Field
              label={
                account
                  ? "Mailbox password (leave blank to keep saved)"
                  : "Mailbox password"
              }
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
            <Field
              label="Incoming host"
              validationMessage={form.formState.errors.incomingHost?.message}
              validationState={
                form.formState.errors.incomingHost ? "error" : "none"
              }
            >
              <Input
                autoComplete="off"
                placeholder="mail.example.com"
                {...form.register("incomingHost")}
              />
            </Field>
            <Field
              label="Incoming port"
              validationMessage={form.formState.errors.incomingPort?.message}
              validationState={
                form.formState.errors.incomingPort ? "error" : "none"
              }
            >
              <Input
                type="number"
                min={1}
                max={65535}
                step={1}
                inputMode="numeric"
                placeholder={provider === "imap" ? "993" : "995"}
                {...form.register("incomingPort")}
              />
            </Field>
            <Field label="Connection security">
              <Dropdown
                value={
                  {
                    implicit_tls: "SSL/TLS",
                    starttls: "STARTTLS",
                    none: "None",
                  }[form.watch("tlsMode")]
                }
                selectedOptions={[form.watch("tlsMode")]}
                onOptionSelect={(_, data) =>
                  form.setValue(
                    "tlsMode",
                    data.optionValue as FormValues["tlsMode"],
                  )
                }
              >
                <Option value="implicit_tls">SSL/TLS</Option>
                <Option value="starttls">STARTTLS</Option>
                <Option value="none">None</Option>
              </Dropdown>
            </Field>
          </>
        )}
        <Field label="Proxy">
          <Dropdown
            value={
              form.watch("proxyId")
                ? proxies.find(
                    (proxy) => String(proxy.id) === form.watch("proxyId"),
                  )?.name
                : "Direct connection"
            }
            selectedOptions={[form.watch("proxyId") ?? ""]}
            onOptionSelect={(_, data) =>
              form.setValue("proxyId", data.optionValue ?? "")
            }
          >
            <Option value="">Direct connection</Option>
            {proxies.map((proxy) => (
              <Option key={proxy.id} value={String(proxy.id)}>
                {proxy.name}
              </Option>
            ))}
          </Dropdown>
        </Field>
        <div className="actions">
          <Button appearance="primary" type="submit" disabled={pending}>
            {pending ? "Saving…" : account ? "Save changes" : "Save account"}
          </Button>
          <Button appearance="secondary" type="button" onClick={onCancel}>
            Cancel
          </Button>
        </div>
      </form>
      {error && (
        <MessageBar intent="error">
          <MessageBarBody>
            <MessageBarTitle>Could not save account</MessageBarTitle>
            {error}
          </MessageBarBody>
        </MessageBar>
      )}
    </div>
  );
}
