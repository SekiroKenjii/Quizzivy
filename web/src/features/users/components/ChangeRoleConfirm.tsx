import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRightLeft, UserCog, UserX } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/sonner";
import {
  changeUserRole,
  type AdminUser,
  type AssignableRole,
} from "@/features/users/api";
import { refusalText } from "@/features/users/userText";
import { ApiError } from "@/lib/api/errors";
import { roleName } from "@/lib/roleName";

function OwnsContent({
  user,
  canDisable,
  canTransfer,
  onDisable,
  onTransfer,
}: Readonly<{
  user: AdminUser;
  canDisable: boolean;
  canTransfer: boolean;
  onDisable: () => void;
  onTransfer: () => void;
}>) {
  const { t } = useTranslation();
  const disabled = user.status === "disabled";
  const step = disabled ? canTransfer : canDisable;
  let text = t("users.error.ownsContentAsk");
  if (step)
    text = t(disabled ? "users.error.ownsContentTransfer" : "users.error.ownsContent");
  return (
    <div role="alert" className="flex flex-col items-start gap-2">
      <p className="text-danger-ink text-meta">{text}</p>
      {step ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={disabled ? onTransfer : onDisable}
        >
          {disabled ? (
            <ArrowRightLeft aria-hidden="true" />
          ) : (
            <UserX aria-hidden="true" />
          )}
          {t(disabled ? "users.transfer.open" : "users.sheet.disable")}
        </Button>
      ) : null}
    </div>
  );
}

/**
 * ChangeRoleConfirm asks before `user` is given `role` ("Chuyển {name} thành
 * quản trị viên?"), with the deck's body: what an Admin can do, or that the
 * permissions change at the next page load. It is open while `role` is set.
 * A refusal stays in the dialog. One for an account that still owns content
 * names the next step by the account's state, with its button when the
 * caller may take it: Disable for an active account (`canDisable`,
 * `onDisable`), then Transfer content for a disabled one (`canTransfer`,
 * `onTransfer`); otherwise it says an Admin who sees everything must do it.
 */
export function ChangeRoleConfirm({
  user,
  role,
  canDisable,
  canTransfer,
  returnFocus,
  onDisable,
  onTransfer,
  onClose,
  onChanged,
}: Readonly<{
  user: AdminUser;
  role: AssignableRole | null;
  canDisable: boolean;
  canTransfer: boolean;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onDisable: () => void;
  onTransfer: () => void;
  onClose: () => void;
  onChanged: () => void;
}>) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [owns, setOwns] = useState(false);
  const admin = role?.builtinKey === "admin";
  const name = role === null ? "" : roleName(role, t);
  const inSentence = role?.builtinKey === null ? name : name.toLowerCase();

  function close() {
    setError(null);
    setOwns(false);
    onClose();
  }

  async function change(roleId: string) {
    if (pending) return;
    setPending(true);
    setError(null);
    setOwns(false);
    try {
      await changeUserRole(user.id, roleId);
      toast.success(t("users.changeRole.done", { role: inSentence }));
      close();
      onChanged();
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === "USER_OWNS_CONTENT")
        setOwns(true);
      else setError(refusalText(cause, t, t("users.changeRole.failed")));
    } finally {
      setPending(false);
    }
  }

  return (
    <ConfirmDialog
      open={role !== null}
      onOpenChange={(open) => {
        if (!open && !pending) close();
      }}
      returnFocus={returnFocus}
      icon={UserCog}
      title={t("users.changeRole.title", { name: user.fullName, role: inSentence })}
      description={t(admin ? "users.changeRole.adminBody" : "users.changeRole.body")}
      confirmLabel={t("users.changeRole.submit")}
      pending={pending}
      error={error}
      onConfirm={() => {
        if (role !== null) void change(role.id);
      }}
    >
      {owns ? (
        <OwnsContent
          user={user}
          canDisable={canDisable}
          canTransfer={canTransfer}
          onDisable={() => {
            close();
            onDisable();
          }}
          onTransfer={() => {
            close();
            onTransfer();
          }}
        />
      ) : null}
    </ConfirmDialog>
  );
}
