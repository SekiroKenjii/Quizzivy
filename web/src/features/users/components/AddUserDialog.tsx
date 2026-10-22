import { useId, useRef, useState, type RefObject, type SubmitEvent } from "react";
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
import { useCan } from "@/features/auth/permissions";
import {
  USERS_KEY,
  assignableRolesQuery,
  createUser,
  emailTakenQuery,
  type AssignableRole,
  type ClassChoice,
  type UserDelivery,
  type UserPasswordReset,
} from "@/features/users/api";
import { ClassPicker } from "@/features/users/components/ClassPicker";
import { CredentialResult } from "@/features/users/components/CredentialResult";
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

function refusalOf(cause: unknown, classSent: boolean): "email" | "class" | null {
  if (!(cause instanceof ApiError)) return null;
  if (cause.code === "EMAIL_TAKEN") return "email";
  return classSent && cause.status === 404 ? "class" : null;
}

function ClassField({
  offered,
  value,
  onChange,
}: Readonly<{
  offered: boolean;
  value: ClassChoice | null;
  onChange: (choice: ClassChoice | null) => void;
}>) {
  const { t } = useTranslation();
  if (!offered) return null;
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-meta font-medium">
        {t("users.addUser.class")}{" "}
        <span className="text-muted-fg font-normal">
          {t("users.addUser.classOptional")}
        </span>
      </span>
      <ClassPicker
        label={`${t("users.addUser.class")} ${t("users.addUser.classOptional")}`}
        value={value}
        none={t("users.classPicker.none")}
        onChange={onChange}
      />
    </div>
  );
}

/**
 * AddUserDialog is the deck's "Add user", 480px: Full name, Email, which says
 * "Nhập đầy đủ địa chỉ email" for a partial address and, after a 300 ms
 * pause, "Email này đã có người dùng" when an account already uses it, the
 * role among those the caller may give, for a student role and a caller with
 * `scope.all` the class to add them to (none by default), and how the
 * password reaches the person. Create answers `aria-disabled` until the form is valid. A
 * temporary password is shown once in CredentialResult; then, or at once for
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
  const [created, setCreated] = useState<UserPasswordReset | null>(null);
  const [classChoice, setClassChoice] = useState<ClassChoice | null>(null);
  const seesAll = useCan("scope.all");
  const emailField = useRef<HTMLInputElement>(null);

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
  const classOffered =
    seesAll && roles.data?.find((role) => role.id === roleId)?.studentTarget === true;
  const classId = classOffered ? classChoice?.id : undefined;
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
    setClassChoice(null);
  }

  function close() {
    if (pending) return;
    reset();
    onOpenChange(false);
  }

  function refused(cause: unknown) {
    const refusal = refusalOf(cause, classId !== undefined);
    if (refusal === "email") {
      setTakenOnSubmit(typedEmail);
      emailField.current?.focus();
      return;
    }
    if (refusal === "class") {
      setClassChoice(null);
      setError(t("users.addUser.classGone"));
      return;
    }
    setError(refusalText(cause, t, t("users.addUser.failed")));
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
        ...(classId === undefined ? {} : { classId }),
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
          fullName: answer.user.fullName,
          email: answer.user.email,
          temporaryPassword: answer.temporaryPassword,
        });
      }
    } catch (cause) {
      refused(cause);
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
                ref={emailField}
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
              <p
                id={`${id}-email-error`}
                aria-live="polite"
                className="text-danger-ink text-xs empty:hidden"
              >
                {emailError}
              </p>
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
            <ClassField
              offered={classOffered}
              value={classChoice}
              onChange={setClassChoice}
            />
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
            className="aria-disabled:bg-primary/55 aria-disabled:hover:bg-primary/55"
          >
            {t("users.addUser.submit")}
          </Button>
        </DialogShellFooter>
      </DialogShell>
      <CredentialResult
        open={created !== null}
        title={t("users.addUser.done", { name: created?.fullName ?? "" })}
        credentials={created === null ? [] : [created]}
        failures={[]}
        returnFocus={returnFocus}
        onDone={() => {
          const next = created?.userId;
          setCreated(null);
          if (next !== undefined) onCreated(next);
        }}
      />
    </>
  );
}
