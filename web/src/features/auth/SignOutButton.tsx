import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { useLogout } from "./useSession";

/**
 * §5.4's logout, as a control. Disabled while in flight so a second click
 * cannot start a second revocation against a family the first one already
 * revoked.
 */
export function SignOutButton() {
  const { t } = useTranslation();
  const logout = useLogout();
  const [pending, setPending] = useState(false);

  return (
    <Button
      variant="ghost"
      size="sm"
      disabled={pending}
      onClick={() => {
        setPending(true);
        void logout();
      }}
    >
      {t("common.signOut")}
    </Button>
  );
}
