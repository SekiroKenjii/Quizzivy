import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { KeyRound } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { toast } from "@/components/ui/sonner";
import {
  USERS_KEY,
  resetUserPassword,
  type AdminUser,
  type UserDelivery,
  type UserPasswordReset,
} from "@/features/users/api";
import { DeliveryChoice } from "@/features/users/components/DeliveryChoice";
import { refusalText } from "@/features/users/userText";

/**
 * ResetUserPassword asks before it resets `user`'s password, with the
 * delivery choice, a link by email by default. A sent link ends in a toast;
 * a temporary password goes to `onCredential`, whose owner shows it once
 * outside the sheet, so a Back that closes the sheet cannot lose it.
 */
export function ResetUserPassword({
  open,
  user,
  returnFocus,
  onClose,
  onCredential,
}: Readonly<{
  open: boolean;
  user: AdminUser;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onClose: () => void;
  onCredential: (credential: UserPasswordReset) => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [delivery, setDelivery] = useState<UserDelivery>("email_link");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function finish() {
    setError(null);
    setDelivery("email_link");
    onClose();
  }

  async function reset() {
    if (pending) return;
    setPending(true);
    setError(null);
    try {
      const { temporaryPassword } = await resetUserPassword(user.id, delivery);
      if (temporaryPassword === undefined)
        toast.success(t("users.reset.linkSent", { email: user.email }));
      else
        onCredential({
          userId: user.id,
          fullName: user.fullName,
          email: user.email,
          temporaryPassword,
        });
      finish();
      void client.invalidateQueries({ queryKey: USERS_KEY });
    } catch (cause) {
      setError(refusalText(cause, t, t("users.reset.failed")));
    } finally {
      setPending(false);
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !pending) finish();
      }}
      returnFocus={returnFocus}
      icon={KeyRound}
      title={t("users.reset.title", { name: user.fullName })}
      description={t("users.reset.body")}
      confirmLabel={t("users.reset.submit")}
      pending={pending}
      error={error}
      onConfirm={() => void reset()}
    >
      <DeliveryChoice value={delivery} onChange={setDelivery} />
    </ConfirmDialog>
  );
}
