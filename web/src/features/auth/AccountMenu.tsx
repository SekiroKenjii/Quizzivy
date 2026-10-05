import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ChevronDown, ChevronsUpDown, LogOut, Moon, Settings, Sun } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useWorkspace } from "@/features/auth/permissions";
import { useLogout } from "@/features/auth/useSession";
import { givenName } from "@/features/assignments/studentTime";
import type { components } from "@/lib/api/schema";
import { useResolvedTheme, writeThemePreference } from "@/lib/theme";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth";

type User = components["schemas"]["CurrentUser"];
type SidebarForm = "expanded" | "collapsed";

const DECK_ITEM =
  "focus:bg-hover focus:text-fg text-sm in-data-[scale=deck]:gap-2.5 in-data-[scale=deck]:p-2 in-data-[scale=deck]:leading-4 in-data-[scale=deck]:data-[disabled]:opacity-50 in-data-[scale=deck]:[&_svg:not([class*='size-'])]:size-[0.9375rem]";

const TOP_BAR = {
  trigger:
    "hover:bg-hover data-[state=open]:bg-hover inline-flex h-10 items-center rounded-[0.625rem] p-1",
  avatar: {},
  menu: { align: "end", sideOffset: 6 },
} as const;

const FOOT = {
  trigger:
    "hover:bg-hover data-[state=open]:bg-hover flex w-full items-center gap-2.5 rounded-md p-1.5 text-left",
  avatar: { shape: "square", className: "rounded-md" },
  menu: { align: "start", side: "top", sideOffset: 8 },
} as const;

function useSignOut() {
  const logout = useLogout();
  const [pending, setPending] = useState(false);
  const signOut = () => {
    setPending(true);
    void logout();
  };
  return { pending, signOut };
}

function FootIdentity({ name }: Readonly<{ name: string }>) {
  const { t } = useTranslation();
  const admin = useWorkspace("admin");
  return (
    <>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm leading-tight font-medium">{name}</span>
        <span className="text-muted-fg truncate text-xs leading-tight">
          {admin ? t("teacherShell.role.admin") : t("teacherShell.role.teacher")}
        </span>
      </span>
      <ChevronsUpDown
        className="text-muted-fg size-[0.9375rem] flex-none"
        aria-hidden="true"
      />
    </>
  );
}

function DeckAccountMenu({
  user,
  settingsTo,
  sidebar,
}: Readonly<{ user: User; settingsTo: string; sidebar: SidebarForm | undefined }>) {
  const { t } = useTranslation();
  const theme = useResolvedTheme();
  const { pending, signOut } = useSignOut();
  const form = sidebar === undefined ? TOP_BAR : FOOT;
  const dark = theme === "dark";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t("nav.account", { name: user.fullName })}
        className={cn(form.trigger, sidebar === "collapsed" && "justify-center")}
      >
        <Avatar name={user.fullName} tone="self" {...form.avatar} />
        {sidebar === "expanded" && <FootIdentity name={user.fullName} />}
      </DropdownMenuTrigger>
      <DropdownMenuContent
        {...form.menu}
        className="data-[scale=deck]:bg-card data-[scale=deck]:shadow-float data-[scale=deck]:z-(--z-popover) data-[scale=deck]:w-60 data-[scale=deck]:rounded-lg data-[scale=deck]:p-1.5"
      >
        <div className="mb-1 flex items-center gap-2.5 border-b px-2 pt-1.5 pb-2.5">
          <Avatar name={user.fullName} tone="self" {...form.avatar} />
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
          onSelect={() => writeThemePreference(dark ? "light" : "dark")}
        >
          {dark ? <Sun aria-hidden="true" /> : <Moon aria-hidden="true" />}
          {dark ? t("common.lightMode") : t("common.darkMode")}
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

function LegacyAccountMenu({
  user,
  settingsTo,
  named,
}: Readonly<{ user: User; settingsTo: string; named: boolean }>) {
  const { t } = useTranslation();
  const { pending, signOut } = useSignOut();

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

/**
 * AccountMenu is a console's account button and the menu it opens. `deck`
 * draws the design deck's form, for a console rebuilt to it: a 40px avatar
 * button at the end of the top bar and a 240px menu with the user's name and
 * email, Settings, a switch between the light and dark themes, and Sign out.
 * With `sidebar` the deck form is the row at the foot of the teacher sidebar
 * instead: a square avatar, and while `expanded` the name over a role line,
 * with the same menu opening above it. Without `deck` the menu is the old
 * consoles': Settings and Sign out, and with `named` the given name beside
 * the avatar.
 */
export function AccountMenu({
  settingsTo = "/teacher/settings",
  named = false,
  deck = false,
  sidebar,
}: Readonly<{
  settingsTo?: string;
  named?: boolean;
  deck?: boolean;
  sidebar?: SidebarForm;
}>) {
  const user = useAuthStore((s) => s.user);
  if (!user) return null;
  if (deck)
    return <DeckAccountMenu user={user} settingsTo={settingsTo} sidebar={sidebar} />;
  return <LegacyAccountMenu user={user} settingsTo={settingsTo} named={named} />;
}
