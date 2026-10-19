import { useId, useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { ArrowRightLeft } from "lucide-react";
import { RadioCards } from "@/components/shared/form/RadioCard";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { SearchInput } from "@/components/shared/SearchInput";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { useCan } from "@/features/auth/permissions";
import { rolesQuery, type RoleSummary } from "@/features/roles/api";
import {
  recipientsQuery,
  transferUserContent,
  type AdminUser,
  type AdminUserDetail,
  type UserContentTransferResult,
} from "@/features/users/api";
import { searchOf } from "@/features/users/userFilters";
import { refusalText } from "@/features/users/userText";
import type { components } from "@/lib/api/schema";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";

const COUNTED = ["classes", "tests", "questions", "media", "assignments"] as const;
const WRITE_KEYS: Record<
  (typeof COUNTED)[number],
  components["schemas"]["PermissionKey"]
> = {
  classes: "teaching.classes.write",
  tests: "content.tests.write",
  questions: "content.questions.write",
  media: "content.media.write",
  assignments: "teaching.assignments.write",
};
const MOVED = [
  "classes",
  "tests",
  "questions",
  "questionGroups",
  "media",
  "assignments",
] as const;

const TEACHING = ["content.", "teaching.", "people.students."];

function teaches(role: RoleSummary): boolean {
  return role.permissions.some((key) =>
    TEACHING.some((prefix) => key.startsWith(prefix)),
  );
}

function readOnlyKinds(
  owned: AdminUserDetail["ownedContent"],
  role: RoleSummary | undefined,
): (typeof COUNTED)[number][] {
  if (role === undefined) return [];
  const held = new Set(role.permissions);
  return COUNTED.filter((kind) => owned[kind] > 0 && !held.has(WRITE_KEYS[kind]));
}

function Recipients({
  failed,
  loading,
  options,
  value,
  onChange,
}: Readonly<{
  failed: boolean;
  loading: boolean;
  options: readonly AdminUser[];
  value: string;
  onChange: (id: string) => void;
}>) {
  const { t } = useTranslation();
  if (failed)
    return (
      <p role="alert" className="text-danger-ink text-xs">
        {t("users.transfer.recipientsFailed")}
      </p>
    );
  if (options.length === 0 && !loading)
    return <p className="text-muted-fg text-meta">{t("users.transfer.noRecipient")}</p>;
  return (
    <RadioCards
      label={t("users.transfer.recipient")}
      value={value}
      onChange={onChange}
      className="grid-cols-1"
      options={options.map((one) => ({
        value: one.id,
        label: one.fullName,
        hint: one.email,
      }))}
    />
  );
}

function Moved({
  name,
  result,
}: Readonly<{ name: string; result: UserContentTransferResult }>) {
  const { t } = useTranslation();
  return (
    <>
      <p className="text-ui">{t("users.transfer.movedTo", { name })}</p>
      <ul
        aria-label={t("users.transfer.moved")}
        className="text-meta flex flex-col gap-1"
      >
        {MOVED.map((kind) => (
          <li key={kind}>
            {t(`users.transfer.count.${kind}`, { count: result.moved[kind] })}
          </li>
        ))}
      </ul>
      {result.leftBehind.imports > 0 ? (
        <p className="text-meta">
          {t("users.transfer.importsLeft", { count: result.leftBehind.imports })}
        </p>
      ) : null}
      {result.leftBehind.students > 0 ? (
        <p className="text-meta">
          {t("users.transfer.studentsLeft", { count: result.leftBehind.students })}
        </p>
      ) : null}
    </>
  );
}

/**
 * TransferContentDialog gives everything a disabled `user` owns to another
 * teacher (DG-56): "Chuyển nội dung của {name} cho" with a picker of the
 * active teachers and Admins whose role opens the teacher workspace (when
 * the caller can read the roles), searched by name or email, the counts that
 * will move, a warning naming the kinds the chosen recipient's role cannot
 * edit (D-052, read from the roles when the caller holds
 * `people.roles.manage`), and "Báo cho giáo viên nhận", on by default. It then lists what
 * moved, the Word imports left behind with the account, and the students the
 * recipient still does not reach.
 */
export function TransferContentDialog({
  open,
  user,
  owned,
  returnFocus,
  onClose,
  onMoved,
}: Readonly<{
  open: boolean;
  user: AdminUser;
  owned: AdminUserDetail["ownedContent"];
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onClose: () => void;
  onMoved: () => void;
}>) {
  const { t } = useTranslation();
  const notifyId = useId();
  const [typed, setTyped] = useState("");
  const q = searchOf(useDebounced(typed, 300));
  const [toUserId, setToUserId] = useState("");
  const [notify, setNotify] = useState(true);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<UserContentTransferResult | null>(null);
  const [movedTo, setMovedTo] = useState("");
  const locale = useLocale();
  const manageRoles = useCan("people.roles.manage");
  const roles = useQuery({ ...rolesQuery(), enabled: open && manageRoles });
  const recipients = useQuery({
    ...recipientsQuery(q),
    enabled: open && result === null,
  });
  const byRole = new Map((roles.data?.roles ?? []).map((role) => [role.id, role]));
  const options = (recipients.data ?? []).filter((one) => {
    if (one.id === user.id) return false;
    const role = byRole.get(one.role.id);
    return role === undefined || teaches(role);
  });
  const chosen = options.find((one) => one.id === toUserId);
  const readOnly = readOnlyKinds(
    owned,
    chosen === undefined ? undefined : byRole.get(chosen.role.id),
  );

  function finish() {
    if (pending) return;
    setTyped("");
    setToUserId("");
    setNotify(true);
    setError(null);
    setResult(null);
    setMovedTo("");
    onClose();
  }

  async function transfer() {
    if (pending) return;
    if (toUserId === "") {
      setError(t("users.transfer.pick"));
      return;
    }
    setPending(true);
    setError(null);
    try {
      const name = chosen?.fullName ?? "";
      setResult(await transferUserContent(user.id, toUserId, notify));
      setMovedTo(name);
      onMoved();
    } catch (cause) {
      setError(refusalText(cause, t, t("users.transfer.failed")));
    } finally {
      setPending(false);
    }
  }

  return (
    <DialogShell
      open={open}
      width={480}
      returnFocus={returnFocus}
      onOpenChange={(next) => {
        if (!next) finish();
      }}
    >
      <DialogShellHeader
        icon={ArrowRightLeft}
        title={
          result === null
            ? t("users.transfer.title", { name: user.fullName })
            : t("users.transfer.doneTitle")
        }
        description={result === null ? t("users.transfer.lead") : undefined}
      />
      <DialogShellBody>
        {result === null ? (
          <>
            <ul
              aria-label={t("users.transfer.willMove")}
              className="text-meta flex flex-wrap gap-x-4 gap-y-1"
            >
              {COUNTED.map((kind) => (
                <li key={kind}>
                  {t(`users.transfer.count.${kind}`, { count: owned[kind] })}
                </li>
              ))}
            </ul>
            {chosen === undefined || readOnly.length === 0 ? null : (
              <p role="status" className="text-warning-ink text-meta">
                {t("users.transfer.readOnly", {
                  name: chosen.fullName,
                  kinds: new Intl.ListFormat(locale, { type: "conjunction" }).format(
                    readOnly.map((kind) => t(`users.transfer.kind.${kind}`)),
                  ),
                })}
              </p>
            )}
            <SearchInput
              className="w-full"
              value={typed}
              onChange={setTyped}
              placeholder={t("users.transfer.search")}
            />
            <Recipients
              failed={recipients.isError}
              loading={recipients.isPending}
              options={options}
              value={toUserId}
              onChange={setToUserId}
            />
            <label
              htmlFor={notifyId}
              className="text-ui flex items-center justify-between gap-3"
            >
              {t("users.transfer.notify")}
              <Switch id={notifyId} checked={notify} onCheckedChange={setNotify} />
            </label>
            {error === null ? null : (
              <p role="alert" className="text-danger-ink text-xs">
                {error}
              </p>
            )}
          </>
        ) : (
          <Moved name={movedTo} result={result} />
        )}
      </DialogShellBody>
      <DialogShellFooter>
        {result === null ? (
          <>
            <Button type="button" variant="outline" onClick={finish}>
              {t("common.cancel")}
            </Button>
            <Button
              type="button"
              aria-disabled={pending || toUserId === "" || undefined}
              onClick={() => void transfer()}
            >
              {t("users.transfer.submit")}
            </Button>
          </>
        ) : (
          <Button type="button" onClick={finish}>
            {t("users.transfer.done")}
          </Button>
        )}
      </DialogShellFooter>
    </DialogShell>
  );
}
