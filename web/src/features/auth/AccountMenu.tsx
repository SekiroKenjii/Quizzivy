import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ChevronDown, LogOut, Moon, Settings, Sun } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useLogout } from "@/features/auth/useSession";
import { givenName } from "@/features/assignments/studentTime";
import { useResolvedTheme, writeThemePreference } from "@/lib/theme";
import { useAuthStore } from "@/stores/auth";

const DECK_ITEM =
  "focus:bg-hover focus:text-fg gap-2.5 p-2 text-sm leading-4 [&_svg:not([class*='size-'])]:size-[0.9375rem]";

/**
 * AccountMenu is the avatar at the end of a console's top bar and the menu it
 * opens. `deck` draws the design deck's form, for a console rebuilt to it: a
 * 40px avatar button and a 240px menu with the user's name and email,
 * Settings, a switch between the light and dark themes, and Sign out. Without
 * it the menu is the old consoles': Settings and Sign out, and with `named`
 * the given name beside the avatar.
 */
export function AccountMenu({
  settingsTo = "/admin/settings",
  named = false,
  deck = false,
}: Readonly<{
  settingsTo?: string;
  named?: boolean;
  deck?: boolean;
}>) {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const logout = useLogout();
  const theme = useResolvedTheme();
  const [pending, setPending] = useState(false);

  if (!user) return null;

  const signOut = () => {
    setPending(true);
    void logout();
  };

  if (deck) {
    return (
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label={t("nav.account", { name: user.fullName })}
          className="hover:bg-hover data-[state=open]:bg-hover inline-flex h-10 items-center rounded-[0.625rem] p-1"
        >
          <Avatar name={user.fullName} tone="self" />
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          sideOffset={6}
          className="bg-card shadow-float z-(--z-popover) w-60 rounded-lg p-1.5"
        >
          <div className="mb-1 flex items-center gap-2.5 border-b px-2 pt-1.5 pb-2.5">
            <Avatar name={user.fullName} tone="self" />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold">{user.fullName}</p>
              <p className="text-muted-fg truncate text-xs">{user.email}</p>
            </div>
          </div>
          <DropdownMenuItem asChild className={DECK_ITEM}>
            <Link to={settingsTo}>
              <Settings aria-hidden="true" />
              {t("nav.settings")}
            </Link>
          </DropdownMenuItem>
          <DropdownMenuItem
            className={DECK_ITEM}
            onSelect={() => writeThemePreference(theme === "dark" ? "light" : "dark")}
          >
            {theme === "dark" ? (
              <Sun aria-hidden="true" />
            ) : (
              <Moon aria-hidden="true" />
            )}
            {theme === "dark" ? t("common.lightMode") : t("common.darkMode")}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={pending}
            onSelect={signOut}
            className={`${DECK_ITEM} text-danger-ink focus:text-danger-ink border-t`}
          >
            <LogOut aria-hidden="true" />
            {t("common.signOut")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }

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
        <DropdownMenuItem disabled={pending} onSelect={signOut}>
          <LogOut aria-hidden="true" />
          {t("common.signOut")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
