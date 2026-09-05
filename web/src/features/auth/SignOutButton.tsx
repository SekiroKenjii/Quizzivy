import { useState } from "react";
import { useTranslation } from "react-i18next";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useLogout } from "./useSession";

/**
 * §5.4's logout, as a control. Disabled while in flight so a second click
 * cannot start a second revocation against a family the first one already
 * revoked.
 */
export function SignOutButton({
  variant = "ghost",
  className,
}: Readonly<{
  variant?: "ghost" | "outline";
  className?: string;
}>) {
  const { t } = useTranslation();
  const logout = useLogout();
  const [pending, setPending] = useState(false);

  return (
    <Button
      variant={variant}
      size="sm"
      className={cn(className)}
      disabled={pending}
      onClick={() => {
        setPending(true);
        void logout();
      }}
    >
      <LogOut aria-hidden="true" />
      {t("common.signOut")}
    </Button>
  );
}
