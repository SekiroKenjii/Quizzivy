import { useId, useState, type RefObject, type SubmitEvent } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { UserPlus } from "lucide-react";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/sonner";
import type { OneTimeCredential } from "@/features/students/credentials";
import { ResetSheet } from "@/features/students/components/ResetSheet";
import {
  USERS_KEY,
  assignableRolesQuery,
  createUser,
  emailTakenQuery,
  type AssignableRole,
  type UserDelivery,
} from "@/features/users/api";
import { DeliveryChoice } from "@/features/users/components/DeliveryChoice";
import { RolePicker } from "@/features/users/components/RolePicker";
import { refusalText } from "@/features/users/userText";
import { ApiError } from "@/lib/api/errors";
import { useDebounced } from "@/lib/useDebounced";

function isEmail(value: string): boolean {
  const at = value.indexOf("@");
  if (at < 1 || at !== value.lastIndexOf("@") || /\s/.test(value)) return false;
  const domain = value.slice(at + 1);
  const dot = domain.lastIndexOf(".");
  return dot > 0 && dot < domain.length - 1;
}

function defaultRole(roles: readonly AssignableRole[]): string {
  return (roles.find((role) => role.builtinKey === "student") ?? roles[0])?.id ?? "";
}

/**
 * AddUserDialog is the deck's "Add user", 480px: Full name, Email, which says
 * "Nhập đầy đủ địa chỉ email" for a partial address and, after a 300 ms
 * pause, "Email này đã có người dùng" when an account already uses it, the
 * role among those the caller may give, and how the password reaches the
 * person. Create answers `aria-disabled` until the form is valid. A
 * temporary password is shown once in the ResetSheet; then, or at once for
 * an emailed link, `onCreated` opens the new account.
 */
export function AddUserDialog({
  open,
  onOpenChange,
  onCreated,
  returnFocus,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (userId: string) => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const id = useId();
  const client = useQueryClient();
  const roles = useQuery({ ...assignableRolesQuery(), enabled: open });
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [emailTouched, setEmailTouched] = useState(false);
  const [picked, setPicked] = useState("");
  const [delivery, setDelivery] = useState<UserDelivery>("email_link");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [takenOnSubmit, setTakenOnSubmit] = useState<string | null>(null);
  const [created, setCreated] = useState<
    (OneTimeCredential & { userId: string }) | null
  >(null);

  const typedEmail = email.trim();
  const emailOk = isEmail(typedEmail);
  const settled = useDebounced(typedEmail, 300);
  const taken = useQuery({
    ...emailTakenQuery(settled),
    enabled: open && isEmail(settled) && settled === typedEmail,
  });
  const emailTaken =
    (taken.data === true && settled === typedEmail) ||
    takenOnSubmit?.toLowerCase() === typedEmail.toLowerCase();
  const roleId = picked === "" ? defaultRole(roles.data ?? []) : picked;
  const valid = fullName.trim() !== "" && emailOk && !emailTaken && roleId !== "";
  let emailError: string | null = null;
  if (emailTouched && typedEmail !== "" && !emailOk)
    emailError = t("users.addUser.emailPartial");
  else if (emailTaken) emailError = t("users.addUser.emailTaken");

  function reset() {
    setFullName("");
    setEmail("");
    setEmailTouched(false);
    setPicked("");
    setDelivery("email_link");
    setError(null);
    setTakenOnSubmit(null);
  }

  function close() {
    if (pending) return;
    reset();
    onOpenChange(false);
  }

  async function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setEmailTouched(true);
    if (pending || !valid) return;
    setPending(true);
    setError(null);
    try {
      const answer = await createUser({
        fullName: fullName.trim(),
        email: typedEmail,
        roleId,
        delivery,
      });
      void client.invalidateQueries({ queryKey: USERS_KEY });
      reset();
      onOpenChange(false);
      if (answer.temporaryPassword === undefined) {
        toast.success(t("users.addUser.linkSent", { email: answer.user.email }));
        onCreated(answer.user.id);
      } else {
        setCreated({
          userId: answer.user.id,
          studentId: answer.user.id,
          fullName: answer.user.fullName,
          email: answer.user.email,
          temporaryPassword: answer.temporaryPassword,
        });
      }
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === "EMAIL_TAKEN")
        setTakenOnSubmit(typedEmail);
      else setError(refusalText(cause, t, t("users.addUser.failed")));
    } finally {
      setPending(false);
    }
  }

  return (
    <>
      <DialogShell
        open={open}
        width={480}
        returnFocus={returnFocus}
        onOpenChange={(next) => {
          if (!next) close();
        }}
      >
        <DialogShellHeader
          icon={UserPlus}
          title={t("users.addUser.title")}
          description={t("users.addUser.lead")}
        />
        <DialogShellBody>
          <form
            id={id}
            noValidate
            onSubmit={(event) => void submit(event)}
            className="flex flex-col gap-3.5"
          >
            <div className="flex flex-col gap-1.5">
              <label htmlFor={`${id}-name`} className="text-meta font-medium">
                {t("users.addUser.fullName")}
              </label>
              <Input
                id={`${id}-name`}
                value={fullName}
                maxLength={200}
                autoComplete="off"
                placeholder={t("users.addUser.namePlaceholder")}
                readOnly={pending}
                onChange={(event) => setFullName(event.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor={`${id}-email`} className="text-meta font-medium">
                {t("users.addUser.email")}
              </label>
              <Input
                id={`${id}-email`}
                type="email"
                value={email}
                maxLength={254}
                autoComplete="off"
                placeholder={t("users.addUser.emailPlaceholder")}
                readOnly={pending}
                aria-invalid={emailError !== null || undefined}
                aria-describedby={emailError === null ? undefined : `${id}-email-error`}
                onChange={(event) => setEmail(event.target.value)}
                onBlur={() => setEmailTouched(true)}
              />
              {emailError === null ? null : (
                <p id={`${id}-email-error`} className="text-danger-ink text-xs">
                  {emailError}
                </p>
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <span id={`${id}-role`} className="text-meta font-medium">
                {t("users.addUser.role")}
              </span>
              {roles.isError && roles.data === undefined ? (
                <p role="alert" className="text-danger-ink text-xs">
                  {t("users.addUser.rolesFailed")}
                </p>
              ) : null}
              {roles.data === undefined ? null : (
                <RolePicker
                  label={t("users.addUser.role")}
                  roles={roles.data}
                  value={roleId}
                  disabled={pending}
                  onChange={setPicked}
                />
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <span className="text-meta font-medium">{t("users.delivery.label")}</span>
              <DeliveryChoice value={delivery} onChange={setDelivery} />
            </div>
          </form>
          {error === null ? null : (
            <p role="alert" className="text-danger-ink text-xs">
              {error}
            </p>
          )}
        </DialogShellBody>
        <DialogShellFooter>
          <Button type="button" variant="outline" onClick={close}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            form={id}
            aria-disabled={pending || !valid || undefined}
            className="aria-disabled:bg-primary/50 aria-disabled:hover:bg-primary/50"
          >
            {t("users.addUser.submit")}
          </Button>
        </DialogShellFooter>
      </DialogShell>
      <ResetSheet
        open={created !== null}
        title={t("users.addUser.done", { name: created?.fullName ?? "" })}
        credentials={created === null ? [] : [created]}
        onDone={() => {
          const next = created?.userId;
          setCreated(null);
          if (next !== undefined) onCreated(next);
        }}
      />
    </>
  );
}
