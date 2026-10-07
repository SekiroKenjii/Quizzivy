import { authStore } from "@/stores/auth";
import { useEffect, useRef, useState } from "react";
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
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const [pending, setPending] = useState(false);

  return (
    <Button
      variant={variant}
      className={cn(className)}
      disabled={pending}
      onClick={() => {
        setPending(true);
        const completion = logout();
        const lease = authStore.captureActor();
        void completion.finally(() => {
          if (mounted.current && authStore.isCurrent(lease)) setPending(false);
        });
      }}
    >
      <LogOut aria-hidden="true" />
      {t("common.signOut")}
    </Button>
  );
}
