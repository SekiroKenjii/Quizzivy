import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { KeyRound } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { toast } from "@/components/ui/sonner";
import type { OneTimeCredential } from "@/features/students/credentials";
import { ResetSheet } from "@/features/students/components/ResetSheet";
import {
  USERS_KEY,
  resetUserPassword,
  type AdminUser,
  type UserDelivery,
} from "@/features/users/api";
import { DeliveryChoice } from "@/features/users/components/DeliveryChoice";
import { refusalText } from "@/features/users/userText";

/**
 * ResetUserPassword resets `user`'s password: a confirmation with the
 * delivery choice, a link by email by default, then either the temporary
 * password in the ResetSheet, shown once and held only in this component's
 * state until it closes, or a toast that the link was sent.
 */
export function ResetUserPassword({
  open,
  user,
  onClose,
}: Readonly<{ open: boolean; user: AdminUser; onClose: () => void }>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [delivery, setDelivery] = useState<UserDelivery>("email_link");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [credential, setCredential] = useState<OneTimeCredential | null>(null);

  function finish() {
    setCredential(null);
    setError(null);
    setDelivery("email_link");
    onClose();
  }

  async function reset() {
    setPending(true);
    setError(null);
    try {
      const { temporaryPassword } = await resetUserPassword(user.id, delivery);
      if (temporaryPassword === undefined) {
        toast.success(t("users.reset.linkSent", { email: user.email }));
        finish();
      } else {
        setCredential({
          studentId: user.id,
          fullName: user.fullName,
          email: user.email,
          temporaryPassword,
        });
      }
      void client.invalidateQueries({ queryKey: USERS_KEY });
    } catch (cause) {
      setError(refusalText(cause, t, t("users.reset.failed")));
    } finally {
      setPending(false);
    }
  }

  return (
    <>
      <ConfirmDialog
        open={open && credential === null}
        onOpenChange={(next) => {
          if (!next && !pending) finish();
        }}
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
      <ResetSheet
        open={credential !== null}
        title={t("users.reset.done", { name: user.fullName })}
        credentials={credential === null ? [] : [credential]}
        onDone={finish}
      />
    </>
  );
}
