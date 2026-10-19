import { useState } from "react";
import { useTranslation } from "react-i18next";
import { UserCog } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { toast } from "@/components/ui/sonner";
import {
  changeUserRole,
  type AdminUser,
  type AssignableRole,
} from "@/features/users/api";
import { refusalText } from "@/features/users/userText";
import { ApiError } from "@/lib/api/errors";
import { roleName } from "@/lib/roleName";

/**
 * ChangeRoleConfirm asks before `user` is given `role` ("Chuyển {name} thành
 * quản trị viên?"), with the deck's body: what an Admin can do, or that the
 * permissions change at the next page load. It is open while `role` is set.
 * A refusal stays in the dialog; one for an account that still owns content
 * says to move it first with Transfer content, which `canTransfer` says the
 * caller may do.
 */
export function ChangeRoleConfirm({
  user,
  role,
  canTransfer,
  onClose,
  onChanged,
}: Readonly<{
  user: AdminUser;
  role: AssignableRole | null;
  canTransfer: boolean;
  onClose: () => void;
  onChanged: () => void;
}>) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const admin = role?.builtinKey === "admin";
  const name = role === null ? "" : roleName(role, t);
  const inSentence = role?.builtinKey === null ? name : name.toLowerCase();

  async function change(roleId: string) {
    setPending(true);
    setError(null);
    try {
      await changeUserRole(user.id, roleId);
      toast.success(t("users.changeRole.done", { role: inSentence }));
      onClose();
      onChanged();
    } catch (cause) {
      const owns = cause instanceof ApiError && cause.code === "USER_OWNS_CONTENT";
      setError(
        owns && !canTransfer
          ? t("users.error.ownsContentAsk")
          : refusalText(cause, t, t("users.changeRole.failed")),
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <ConfirmDialog
      open={role !== null}
      onOpenChange={(open) => {
        if (open || pending) return;
        setError(null);
        onClose();
      }}
      icon={UserCog}
      title={t("users.changeRole.title", { name: user.fullName, role: inSentence })}
      description={t(admin ? "users.changeRole.adminBody" : "users.changeRole.body")}
      confirmLabel={t("users.changeRole.submit")}
      pending={pending}
      error={error}
      onConfirm={() => {
        if (role !== null) void change(role.id);
      }}
    />
  );
}
