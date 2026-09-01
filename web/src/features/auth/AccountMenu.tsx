import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { LogOut, Settings } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useLogout } from "@/features/auth/useSession";
import { useAuthStore } from "@/stores/auth";

/**
 * The deck's topbar ends in an avatar, not a "Đăng xuất" button.
 *
 * Signing out is a rare, destructive-ish action; putting its button permanently
 * in the chrome spends the most valuable corner of the screen on it and puts it
 * one mis-click from every page. Behind the avatar it is still two keystrokes
 * away and no longer competing with the work.
 */
export function AccountMenu() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const logout = useLogout();
  const [pending, setPending] = useState(false);

  if (!user) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t("nav.account", { name: user.fullName })}
        className="focus-visible:ring-ring rounded-full focus-visible:ring-2 focus-visible:outline-none"
      >
        <Avatar name={user.fullName} size="sm" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-48">
        <div className="px-2 py-1.5">
          <p className="truncate text-sm font-medium">{user.fullName}</p>
          <p className="text-muted-foreground truncate text-xs">{user.email}</p>
        </div>
        <DropdownMenuItem asChild>
          <Link to="/admin/settings">
            <Settings aria-hidden="true" />
            {t("nav.settings")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={pending}
          onSelect={() => {
            setPending(true);
            void logout();
          }}
        >
          <LogOut aria-hidden="true" />
          {t("common.signOut")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
