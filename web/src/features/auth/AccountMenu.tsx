import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ChevronDown, LogOut, Settings } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useLogout } from "@/features/auth/useSession";
import { givenName } from "@/features/assignments/studentTime";
import { useAuthStore } from "@/stores/auth";

/**
 * The deck's topbar ends in an avatar, not a "Đăng xuất" button. The admin's
 * bar shows the avatar alone (A-00); the student's shows the given name beside
 * it (S-13), because a pointer expects a name and a menu where a thumb had an icon.
 */
export function AccountMenu({
  settingsTo = "/admin/settings",
  named = false,
}: Readonly<{
  settingsTo?: string;
  named?: boolean;
}>) {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const logout = useLogout();
  const [pending, setPending] = useState(false);

  if (!user) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t("nav.account", { name: user.fullName })}
        className={
          named
            ? "hover:bg-accent inline-flex h-8 items-center gap-2 rounded-md px-2 text-sm font-medium transition-colors"
            : "rounded-full"
        }
      >
        <Avatar name={user.fullName} size="sm" />
        {named && (
          <>
            {givenName(user.fullName)}
            <ChevronDown className="text-muted-foreground size-4" aria-hidden="true" />
          </>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-48">
        <div className="px-2 py-1.5">
          <p className="truncate text-sm font-medium">{user.fullName}</p>
          <p className="text-muted-foreground truncate text-xs">{user.email}</p>
        </div>
        <DropdownMenuItem asChild>
          <Link to={settingsTo}>
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
