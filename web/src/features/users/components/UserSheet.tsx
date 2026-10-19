import { useRef, useState, type ReactNode, type RefObject } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowRightLeft,
  KeyRound,
  LogOut,
  UserCheck,
  UserX,
  type LucideIcon,
} from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { EventList, type EventListItem } from "@/components/shared/EventList";
import { LoadError } from "@/components/shared/ListState";
import { Sheet } from "@/components/shared/Sheet";
import { Avatar } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { sentenceOf } from "@/features/audit/actions";
import { useCan } from "@/features/auth/permissions";
import {
  USERS_KEY,
  assignableRolesQuery,
  disableUser,
  enableUser,
  signOutUser,
  userQuery,
  type AdminUserActivity,
  type AdminUserDetail,
  type AssignableRole,
  type UserPasswordReset,
} from "@/features/users/api";
import { CredentialResult } from "@/features/users/components/CredentialResult";
import { ChangeRoleConfirm } from "@/features/users/components/ChangeRoleConfirm";
import { ResetUserPassword } from "@/features/users/components/ResetUserPassword";
import { RolePicker } from "@/features/users/components/RolePicker";
import { TransferContentDialog } from "@/features/users/components/TransferContentDialog";
import { lastActiveText, refusalText, statusKeyOf } from "@/features/users/userText";
import { ApiError } from "@/lib/api/errors";
import i18n, { type Locale } from "@/lib/i18n";
import { dayMonthYear, formatRelativeShort } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { roleName } from "@/lib/roleName";
import { cn } from "@/lib/utils";

type Dialog = "reset" | "disable" | "transfer" | null;

const TONES: Record<AdminUserActivity["kind"], EventListItem["tone"]> = {
  sign_in: "success",
  submission: "info",
  class_joined: "neutral",
  audit: "neutral",
};

function activityText(item: AdminUserActivity, t: TFunction, locale: Locale): string {
  const detail = item.detail;
  switch (item.kind) {
    case "sign_in":
      return detail === "google" ||
        detail === "password" ||
        detail === "set_password_link"
        ? t(`users.activity.signIn.${detail}`)
        : t("users.activity.signInPlain");
    case "submission":
      return detail === null
        ? t("users.activity.submittedRemoved")
        : t("users.activity.submitted", { title: detail });
    case "class_joined":
      return detail === null
        ? t("users.activity.joinedRemoved")
        : t("users.activity.joined", { name: detail });
    default: {
      const sentences = (i18n.getResource(locale, "translation", "audit.action") ??
        {}) as Record<string, string>;
      return detail === null
        ? t("users.activity.audit")
        : sentenceOf(sentences, detail);
    }
  }
}

function owned(detail: AdminUserDetail): number {
  const content = detail.ownedContent;
  return (
    content.classes +
    content.tests +
    content.questions +
    content.media +
    content.assignments
  );
}

function Fact({ label, children }: Readonly<{ label: string; children: ReactNode }>) {
  return (
    <div className="flex justify-between gap-3 px-3 py-2.5 text-[13px]">
      <dt className="text-muted-fg">{label}</dt>
      <dd className="min-w-0 text-right font-medium break-words">{children}</dd>
    </div>
  );
}

function Action({
  icon: Icon,
  label,
  hint,
  danger = false,
  pending = false,
  onClick,
}: Readonly<{
  icon: LucideIcon;
  label: string;
  hint: string;
  danger?: boolean;
  pending?: boolean;
  onClick: () => void;
}>) {
  return (
    <button
      type="button"
      aria-disabled={pending || undefined}
      onClick={() => {
        if (!pending) onClick();
      }}
      className={cn(
        "bg-card hover:bg-muted flex items-center gap-2.5 rounded-[9px] border px-3 py-2.5 text-left",
        danger ? "text-danger-ink" : "text-fg",
      )}
    >
      <Icon aria-hidden="true" className="size-4 flex-none" />
      <span className="min-w-0 flex-1">
        <span className="text-ui block font-medium">{label}</span>
        <span className="text-muted-fg block text-xs">{hint}</span>
      </span>
    </button>
  );
}

function Facts({ data }: Readonly<{ data: AdminUserDetail }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const person = data.user;
  const methods = [
    person.signIn.password ? t("users.signIn.password") : null,
    person.signIn.google ? t("users.signIn.google") : null,
  ].filter((part) => part !== null);
  return (
    <dl className="divide-y rounded-[10px] border">
      <Fact label={t("users.sheet.status")}>
        {t(`users.status.${statusKeyOf(person)}`)}
      </Fact>
      <Fact label={t("users.sheet.signIn")}>
        {methods.length === 0 ? t("users.signIn.none") : methods.join(" + ")}
      </Fact>
      <Fact label={t("users.sheet.classes")}>
        {data.classes.length === 0
          ? "—"
          : data.classes.map((klass) => klass.name).join(", ")}
      </Fact>
      <Fact label={t("users.sheet.lastActive")}>
        {lastActiveText(person, locale, t)}
      </Fact>
      <Fact label={t("users.sheet.created")}>
        {dayMonthYear(person.createdAt, locale)}
      </Fact>
    </dl>
  );
}

function Activity({ items }: Readonly<{ items: readonly AdminUserActivity[] }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  if (items.length === 0) return null;
  return (
    <section aria-labelledby="user-sheet-activity" className="flex flex-col gap-2">
      <h3 id="user-sheet-activity" className="text-meta font-semibold">
        {t("users.sheet.activity")}
      </h3>
      <EventList
        label={t("users.sheet.activity")}
        items={items.map((item, index) => ({
          key: `${item.at}-${index}`,
          tone: TONES[item.kind],
          text: activityText(item, t, locale),
          time: formatRelativeShort(item.at, locale, t("common.agoNow")),
        }))}
      />
    </section>
  );
}

function SheetSkeleton() {
  const { t } = useTranslation();
  return (
    <div role="status" aria-label={t("common.loading")} className="flex flex-col gap-3">
      <Skeleton className="h-9 w-full" />
      <Skeleton className="h-40 w-full" />
      <Skeleton className="h-28 w-full" />
    </div>
  );
}

function RoleSection({
  data,
  roles,
  origin,
  onRemember,
  onOpen,
  onChanged,
}: Readonly<{
  data: AdminUserDetail;
  roles: readonly AssignableRole[];
  origin: RefObject<HTMLElement | null>;
  onRemember: () => void;
  onOpen: (dialog: Dialog, keepOrigin: boolean) => void;
  onChanged: () => void;
}>) {
  const { t } = useTranslation();
  const [target, setTarget] = useState<string | null>(null);
  const person = data.user;
  const offered = roles.some((role) => role.id === person.role.id);
  return (
    <section aria-labelledby="user-sheet-role" className="flex flex-col gap-2">
      <h3 id="user-sheet-role" className="text-meta font-semibold">
        {t("users.sheet.role")}
      </h3>
      {offered ? (
        <RolePicker
          label={t("users.sheet.role")}
          roles={roles}
          value={person.role.id}
          disabled={!data.can.changeRole}
          onChange={(roleId) => {
            if (roleId === person.role.id) return;
            onRemember();
            setTarget(roleId);
          }}
        />
      ) : (
        <p className="text-ui">
          {roleName(person.role, t)}
          <span className="text-muted-fg text-meta block">
            {t("users.sheet.roleFixed")}
          </span>
        </p>
      )}
      <ChangeRoleConfirm
        user={person}
        role={roles.find((role) => role.id === target) ?? null}
        canDisable={data.can.disable}
        canTransfer={data.can.transfer && owned(data) > 0}
        returnFocus={origin}
        onDisable={() => onOpen("disable", true)}
        onTransfer={() => onOpen("transfer", true)}
        onClose={() => setTarget(null)}
        onChanged={onChanged}
      />
    </section>
  );
}

function Actions({
  data,
  pending,
  onOpen,
  onRun,
}: Readonly<{
  data: AdminUserDetail;
  pending: boolean;
  onOpen: (dialog: Dialog) => void;
  onRun: (write: () => Promise<unknown>, done: string, fallback: string) => void;
}>) {
  const { t } = useTranslation();
  const person = data.user;
  const can = data.can;
  const transferable = can.transfer && person.status === "disabled" && owned(data) > 0;
  if (!(can.resetPassword || can.signOut || can.enable || can.disable || transferable))
    return null;
  return (
    <section aria-labelledby="user-sheet-actions" className="flex flex-col gap-2">
      <h3 id="user-sheet-actions" className="text-meta font-semibold">
        {t("users.sheet.actions")}
      </h3>
      {can.resetPassword ? (
        <Action
          icon={KeyRound}
          label={t("users.sheet.reset")}
          hint={t("users.sheet.resetHint")}
          pending={pending}
          onClick={() => onOpen("reset")}
        />
      ) : null}
      {can.signOut ? (
        <Action
          icon={LogOut}
          label={t("users.sheet.signOut")}
          hint={t("users.sheet.signOutHint")}
          pending={pending}
          onClick={() =>
            onRun(
              () => signOutUser(person.id),
              t("users.sheet.signedOut", { name: person.fullName }),
              t("users.sheet.signOutFailed"),
            )
          }
        />
      ) : null}
      {can.enable ? (
        <Action
          icon={UserCheck}
          label={t("users.sheet.enable")}
          hint={t("users.sheet.enableHint")}
          pending={pending}
          onClick={() =>
            onRun(
              () => enableUser(person.id),
              t("users.sheet.enabled"),
              t("users.sheet.enableFailed"),
            )
          }
        />
      ) : null}
      {can.disable ? (
        <Action
          icon={UserX}
          label={t("users.sheet.disable")}
          hint={t("users.sheet.disableHint")}
          danger
          pending={pending}
          onClick={() => onOpen("disable")}
        />
      ) : null}
      {transferable ? (
        <Action
          icon={ArrowRightLeft}
          label={t("users.transfer.open")}
          hint={t("users.transfer.openHint")}
          pending={pending}
          onClick={() => onOpen("transfer")}
        />
      ) : null}
    </section>
  );
}

function Body({
  data,
  roles,
  origin,
  onRemember,
  onChanged,
  onCredential,
}: Readonly<{
  data: AdminUserDetail;
  roles: readonly AssignableRole[] | undefined;
  origin: RefObject<HTMLElement | null>;
  onRemember: () => void;
  onChanged: () => Promise<void>;
  onCredential: (credential: UserPasswordReset) => void;
}>) {
  const { t } = useTranslation();
  const person = data.user;
  const [dialog, setDialog] = useState<Dialog>(null);
  const [pending, setPending] = useState(false);
  const busy = useRef(false);
  const [error, setError] = useState<string | null>(null);

  async function run(write: () => Promise<unknown>, done: string, fallback: string) {
    if (busy.current) return;
    busy.current = true;
    setPending(true);
    setError(null);
    try {
      await write();
      toast.success(done);
      setDialog(null);
      await onChanged();
    } catch (cause) {
      const message = refusalText(cause, t, fallback);
      if (dialog === null) toast.error(message);
      else setError(message);
    } finally {
      busy.current = false;
      setPending(false);
    }
  }

  function open(next: Dialog, keepOrigin = false) {
    if (busy.current) return;
    if (next !== null && !keepOrigin) onRemember();
    setError(null);
    setDialog(next);
  }

  return (
    <>
      {roles === undefined ? null : (
        <RoleSection
          data={data}
          roles={roles}
          origin={origin}
          onRemember={onRemember}
          onOpen={open}
          onChanged={() => void onChanged()}
        />
      )}
      <Facts data={data} />
      <Actions
        data={data}
        pending={pending}
        onOpen={open}
        onRun={(write, done, fallback) => void run(write, done, fallback)}
      />
      <Activity items={data.activity} />

      <ResetUserPassword
        open={dialog === "reset"}
        user={person}
        returnFocus={origin}
        onClose={() => open(null)}
        onCredential={onCredential}
      />
      <ConfirmDialog
        open={dialog === "disable"}
        onOpenChange={(next) => {
          if (!next) open(null);
        }}
        destructive
        returnFocus={origin}
        icon={UserX}
        title={t("users.disable.title", { name: person.fullName })}
        description={
          <span className="flex flex-col gap-2">
            <span>{t("users.disable.body")}</span>
            {data.ownedContent.classes > 0 ? (
              <span>
                {t("users.disable.teaches", { count: data.ownedContent.classes })}
              </span>
            ) : null}
            {data.inProgressAttempts > 0 ? (
              <span>{t("users.disable.midTest")}</span>
            ) : null}
          </span>
        }
        confirmLabel={t("users.disable.submit")}
        pending={pending}
        error={error}
        onConfirm={() =>
          void run(
            () => disableUser(person.id),
            t("users.sheet.disabled"),
            t("users.sheet.disableFailed"),
          )
        }
      />
      <TransferContentDialog
        open={dialog === "transfer"}
        user={person}
        returnFocus={origin}
        owned={data.ownedContent}
        onClose={() => open(null)}
        onMoved={() => void onChanged()}
      />
    </>
  );
}

function focused(): HTMLElement | null {
  const active = document.activeElement;
  return active instanceof HTMLElement && active !== document.body ? active : null;
}

function missing(error: unknown): boolean {
  return error instanceof ApiError && (error.status === 404 || error.status === 400);
}

/**
 * UserSheet is the deck's user sheet, 440px at the right edge and the whole
 * width on a phone, for the account `userId` names (`?user=` on the Users
 * screen): the role, shown only with `people.roles.manage` and changed
 * through a confirmation, or named when the caller cannot give it; the
 * facts; the actions the server says the caller may take (`can`), each
 * hidden when it is not allowed, and none at all with no heading; and the
 * recent activity. Transfer content is offered for a disabled account that
 * owns something (DG-56). A temporary password is shown outside the sheet,
 * so it outlives a Back that closes it. Each dialog gives focus back to the
 * control that opened it, the role control for one reached from a role change. An id that answers 404 or 400 says
 * the account was not found. `onClose` clears the address.
 */
export function UserSheet({
  userId,
  onClose,
}: Readonly<{ userId: string | null; onClose: () => void }>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const manageRoles = useCan("people.roles.manage");
  const detail = useQuery({ ...userQuery(userId ?? ""), enabled: userId !== null });
  const roles = useQuery({
    ...assignableRolesQuery(),
    enabled: userId !== null && manageRoles,
  });
  const [credential, setCredential] = useState<UserPasswordReset | null>(null);
  const origin = useRef<HTMLElement | null>(null);
  const data = detail.data;
  const person = data?.user;
  const refresh = () => client.invalidateQueries({ queryKey: USERS_KEY });

  return (
    <>
      <Sheet
        open={userId !== null}
        onOpenChange={(open) => {
          if (!open) onClose();
        }}
        width={440}
        title={person?.fullName ?? t("users.sheet.title")}
        subtitle={person?.email}
        leading={
          person === undefined ? undefined : (
            <Avatar name={person.fullName} src={person.avatarUrl} size="44" />
          )
        }
      >
        {detail.isError && missing(detail.error) ? (
          <p role="alert" className="text-muted-fg text-ui">
            {t("users.error.notFound")}
          </p>
        ) : null}
        {detail.isError && !missing(detail.error) ? (
          <LoadError error={detail.error} onRetry={() => void detail.refetch()}>
            {t("users.sheet.loadFailed")}
          </LoadError>
        ) : null}
        {data === undefined ? (
          detail.isPending && userId !== null && <SheetSkeleton />
        ) : (
          <Body
            key={data.user.id}
            data={data}
            roles={manageRoles ? roles.data : undefined}
            origin={origin}
            onRemember={() => {
              origin.current = focused();
            }}
            onChanged={refresh}
            onCredential={setCredential}
          />
        )}
      </Sheet>
      <CredentialResult
        open={credential !== null}
        title={t("users.reset.done", { name: credential?.fullName ?? "" })}
        credentials={credential === null ? [] : [credential]}
        failures={[]}
        returnFocus={origin}
        onDone={() => setCredential(null)}
      />
    </>
  );
}
