import { useState } from "react";
import { useTranslation } from "react-i18next";
import { GraduationCap, KeyRound, UserX } from "lucide-react";
import { BulkActions, BulkBarButton } from "@/components/shared/BulkActions";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { toast } from "@/components/ui/sonner";
import { useCan } from "@/features/auth/permissions";
import {
  BULK_LIMIT,
  BULK_TEMPORARY_LIMIT,
  disableUsers,
  resetUsersPasswords,
  type AdminUser,
  type ClassChoice,
  type UserActionFailure,
  type UserDelivery,
  type UserPasswordReset,
} from "@/features/users/api";
import { AddToClassDialog } from "@/features/users/components/AddToClassDialog";
import {
  CredentialResult,
  type BulkFailure,
} from "@/features/users/components/CredentialResult";
import { DeliveryChoice } from "@/features/users/components/DeliveryChoice";
import { refusalText } from "@/features/users/userText";

type Acting = "reset" | "disable" | "class" | null;

type Result = Readonly<{
  kind: "reset" | "disable" | "class";
  credentials: readonly UserPasswordReset[];
  sent: number;
  failures: readonly BulkFailure[];
}>;

const RESULT_ICONS = { reset: KeyRound, disable: UserX, class: GraduationCap } as const;
const RESULT_TITLES = {
  reset: "users.bulk.resetDoneTitle",
  disable: "users.bulk.disabledTitle",
  class: "users.bulk.addedTitle",
} as const;

function named(
  failed: readonly UserActionFailure[],
  selected: readonly AdminUser[],
): BulkFailure[] {
  const byId = new Map(selected.map((user) => [user.id, user.fullName]));
  return failed.map((failure) => ({
    userId: failure.userId,
    fullName: byId.get(failure.userId) ?? "",
    code: failure.code,
  }));
}

/**
 * UsersBulkBar is the Users screen's bar while accounts are selected, on any
 * page: Reset password, a confirmation with the delivery choice (a link by
 * default, at most 40 for temporary passwords), and Disable, a danger
 * confirmation, each one call for at most 100 accounts. With `scope.all`
 * and `teaching.classes.write`, Add to class puts them in one class. The temporary
 * passwords are shown once in CredentialResult, which also names every
 * account a call did not reach and why. The accounts a call reached leave the
 * selection (`onReached`); those it did not stay selected. `onSettled`
 * refreshes the list.
 */
export function UsersBulkBar({
  selected,
  onReached,
  onClear,
  onSettled,
}: Readonly<{
  selected: readonly AdminUser[];
  onReached: (ids: string[]) => void;
  onClear: () => void;
  onSettled: () => Promise<unknown>;
}>) {
  const { t } = useTranslation();
  const [acting, setActing] = useState<Acting>(null);
  const [delivery, setDelivery] = useState<UserDelivery>("email_link");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<Result | null>(null);
  const count = selected.length;
  const tooMany = delivery === "temporary_password" && count > BULK_TEMPORARY_LIMIT;
  const seesAll = useCan("scope.all");
  const writesClasses = useCan("teaching.classes.write");
  const enrols = seesAll && writesClasses;

  function open(next: Exclude<Acting, null>) {
    if (count > BULK_LIMIT) {
      toast.error(t("users.bulk.tooMany", { count: BULK_LIMIT }));
      return;
    }
    setError(null);
    setDelivery("email_link");
    setActing(next);
  }

  function close() {
    if (pending) return;
    setActing(null);
    setError(null);
  }

  async function settle(failed: readonly Pick<UserActionFailure, "userId">[]) {
    const missed = new Set(failed.map((failure) => failure.userId));
    onReached(selected.filter((user) => !missed.has(user.id)).map((user) => user.id));
    await onSettled().catch(() => toast(t("common.refreshFailed")));
  }

  async function added(choice: ClassChoice, failures: BulkFailure[]) {
    setActing(null);
    if (failures.length === 0)
      toast.success(t("users.bulk.added", { count, name: choice.name }));
    else setResult({ kind: "class", credentials: [], sent: 0, failures });
    await settle(failures);
  }

  async function reset() {
    if (pending || tooMany) return;
    setPending(true);
    setError(null);
    try {
      const answer = await resetUsersPasswords(
        selected.map((user) => user.id),
        delivery,
      );
      const failures = named(answer.failed, selected);
      setActing(null);
      if (delivery === "email_link" && failures.length === 0)
        toast.success(t("users.credential.linksSent", { count: answer.items.length }));
      else
        setResult({
          kind: "reset",
          credentials: answer.items,
          sent: delivery === "email_link" ? answer.items.length : 0,
          failures,
        });
      await settle(answer.failed);
    } catch (cause) {
      setError(refusalText(cause, t, t("users.reset.failed")));
    } finally {
      setPending(false);
    }
  }

  async function disable() {
    if (pending) return;
    setPending(true);
    setError(null);
    try {
      const answer = await disableUsers(selected.map((user) => user.id));
      const failures = named(answer.failed, selected);
      setActing(null);
      if (failures.length === 0)
        toast.success(t("users.bulk.disabled", { count: answer.disabled.length }));
      else setResult({ kind: "disable", credentials: [], sent: 0, failures });
      await settle(answer.failed);
    } catch (cause) {
      setError(refusalText(cause, t, t("users.sheet.disableFailed")));
    } finally {
      setPending(false);
    }
  }

  return (
    <>
      <BulkActions
        selected={selected}
        name={(user) => user.fullName}
        actions={[]}
        onRemoved={onReached}
        onClear={onClear}
        onSettled={onSettled}
      >
        <BulkBarButton icon={KeyRound} pending={pending} onClick={() => open("reset")}>
          {t("users.bulk.reset")}
        </BulkBarButton>
        {enrols ? (
          <BulkBarButton
            icon={GraduationCap}
            pending={pending}
            onClick={() => open("class")}
          >
            {t("users.bulk.addToClass")}
          </BulkBarButton>
        ) : null}
        <BulkBarButton icon={UserX} pending={pending} onClick={() => open("disable")}>
          {t("users.bulk.disable")}
        </BulkBarButton>
      </BulkActions>
      <ConfirmDialog
        open={acting === "reset"}
        onOpenChange={(next) => {
          if (!next) close();
        }}
        icon={KeyRound}
        title={t("users.bulk.resetTitle", { count })}
        description={
          delivery === "email_link"
            ? t("users.bulk.resetLinkBody")
            : t("users.bulk.resetPasswordBody")
        }
        confirmLabel={t("users.bulk.resetSubmit")}
        pending={pending}
        error={
          tooMany
            ? t("users.bulk.temporaryTooMany", { count: BULK_TEMPORARY_LIMIT })
            : error
        }
        onConfirm={() => void reset()}
      >
        <DeliveryChoice value={delivery} onChange={setDelivery} />
      </ConfirmDialog>
      <ConfirmDialog
        open={acting === "disable"}
        onOpenChange={(next) => {
          if (!next) close();
        }}
        destructive
        icon={UserX}
        title={t("users.bulk.disableTitle", { count })}
        description={t("users.bulk.disableBody")}
        confirmLabel={t("users.bulk.disableSubmit")}
        pending={pending}
        error={error}
        onConfirm={() => void disable()}
      />
      <AddToClassDialog
        open={acting === "class"}
        selected={selected}
        onClose={close}
        onFinished={(choice, failures) => void added(choice, failures)}
      />
      <CredentialResult
        open={result !== null}
        icon={RESULT_ICONS[result?.kind ?? "reset"]}
        title={t(RESULT_TITLES[result?.kind ?? "reset"])}
        credentials={result?.credentials ?? []}
        sent={result?.sent ?? 0}
        failures={result?.failures ?? []}
        onDone={() => setResult(null)}
      />
    </>
  );
}
