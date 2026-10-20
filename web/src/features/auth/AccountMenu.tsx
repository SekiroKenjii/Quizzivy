import {
  chooseAccountPreference,
  refreshAccount,
  rememberWorkspace,
  retryAccountPreference,
  useAccountPreferenceStatus,
} from "@/features/auth/accountPreferences";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router";
import {
  ArrowUpRight,
  BookOpenText,
  ChevronsUpDown,
  GraduationCap,
  KeyRound,
  Languages,
  LogOut,
  Moon,
  Settings,
  Shield,
  Sun,
  type LucideIcon,
} from "lucide-react";
import { ADMIN_HOME, ADMIN_PATHS } from "@/app/adminPaths";
import { CHANGE_PASSWORD_PATH } from "@/app/guards/RequireSession";
import { Avatar } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuMeta,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { holds } from "@/features/auth/permissions";
import { useLogout } from "@/features/auth/useSession";
import type { StaffWorkspace } from "@/layouts/shell/nav";
import type { components } from "@/lib/api/schema";
import { asLocale } from "@/lib/i18n/useLocale";
import { roleName } from "@/lib/roleName";
import { useResolvedTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";
import { authStore, useAuthStore } from "@/stores/auth";

type User = components["schemas"]["CurrentUser"];
type SidebarForm = "expanded" | "collapsed";

interface LinkEntry {
  kind: "link";
  id: string;
  to: string;
  label: string;
  icon: LucideIcon;
  leaves?: StaffWorkspace;
}

type AccountEntry =
  LinkEntry | { kind: "language"; id: string; label: string; icon: LucideIcon };

function entriesFor(
  user: User,
  workspace: StaffWorkspace | undefined,
  settingsTo: string,
  from: string,
): AccountEntry[] {
  if (workspace !== "admin") {
    const settings: AccountEntry = {
      kind: "link",
      id: "settings",
      to: settingsTo,
      label: "nav.settings",
      icon: Settings,
    };
    if (workspace === "teacher" && holds(user, "workspace.admin"))
      return [
        settings,
        {
          kind: "link",
          id: "admin",
          to: ADMIN_HOME,
          label: "adminShell.workspace",
          icon: Shield,
          leaves: "admin",
        },
      ];
    return [settings];
  }
  const entries: AccountEntry[] = [];
  if (holds(user, "system.settings.write"))
    entries.push({
      kind: "link",
      id: "settings",
      to: ADMIN_PATHS.settings,
      label: "adminShell.account.settings",
      icon: Settings,
    });
  if (holds(user, "system.api_reference"))
    entries.push({
      kind: "link",
      id: "api",
      to: ADMIN_PATHS.api,
      label: "adminShell.nav.api",
      icon: BookOpenText,
    });
  if (holds(user, "workspace.teacher"))
    entries.push({
      kind: "link",
      id: "teacher",
      to: "/teacher",
      label: "teacherShell.workspace",
      icon: GraduationCap,
      leaves: "teacher",
    });
  else {
    if (user.hasPassword)
      entries.push({
        kind: "link",
        id: "password",
        to: `${CHANGE_PASSWORD_PATH}?next=${encodeURIComponent(from)}`,
        label: "adminShell.account.password",
        icon: KeyRound,
      });
    entries.push({
      kind: "language",
      id: "language",
      label: "common.language",
      icon: Languages,
    });
  }
  return entries;
}

const DECK_ITEM =
  "focus:bg-hover focus:text-fg text-sm gap-2.5 p-2 leading-4 data-[disabled]:opacity-50 [&_svg:not([class*='size-'])]:size-[0.9375rem]";

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

function LinkRow({ entry }: Readonly<{ entry: LinkEntry }>) {
  const { t } = useTranslation();
  const leaves = entry.leaves;
  return (
    <DropdownMenuItem asChild className={DECK_ITEM}>
      <Link
        to={entry.to}
        onClick={leaves === undefined ? undefined : () => rememberWorkspace(leaves)}
      >
        <entry.icon aria-hidden="true" />
        {leaves === undefined ? (
          t(entry.label)
        ) : (
          <>
            <span className="min-w-0 flex-1 truncate">{t(entry.label)}</span>
            <ArrowUpRight
              className="text-muted-fg size-3.5 flex-none"
              aria-hidden="true"
            />
          </>
        )}
      </Link>
    </DropdownMenuItem>
  );
}

function useSignOut() {
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const logout = useLogout();
  const [pending, setPending] = useState(false);
  const signOut = () => {
    setPending(true);
    const completion = logout();
    const lease = authStore.captureActor();
    void completion.finally(() => {
      if (mounted.current && authStore.isCurrent(lease)) setPending(false);
    });
  };
  return { pending, signOut };
}

function FootIdentity({ name, role }: Readonly<{ name: string; role: User["role"] }>) {
  const { t } = useTranslation();
  return (
    <>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm leading-tight font-medium">{name}</span>
        {role && (
          <span className="text-muted-fg truncate text-xs leading-tight">
            {roleName(role, t)}
          </span>
        )}
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
  workspace,
  settingsTo,
  sidebar,
}: Readonly<{
  user: User;
  workspace: StaffWorkspace | undefined;
  settingsTo: string;
  sidebar: SidebarForm | undefined;
}>) {
  const { t, i18n } = useTranslation();
  const locale = asLocale(i18n.language);
  const { pathname, search } = useLocation();
  const entries = entriesFor(user, workspace, settingsTo, pathname + search);
  const theme = useResolvedTheme();
  const preferenceStatus = useAccountPreferenceStatus();
  const { pending, signOut } = useSignOut();
  const preferenceMessage = {
    idle: "settings.preferenceSaved",
    failed: "common.retry",
    saving: "settings.preferenceSaving",
    saved: "settings.preferenceSaved",
  }[preferenceStatus.phase];
  const form = sidebar === undefined ? TOP_BAR : FOOT;
  const dark = theme === "dark";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t("nav.account", { name: user.displayName ?? user.fullName })}
        className={cn(form.trigger, sidebar === "collapsed" && "justify-center")}
      >
        <Avatar
          name={user.displayName ?? user.fullName}
          src={user.avatarUrl}
          tone="self"
          {...form.avatar}
        />
        {sidebar === "expanded" && (
          <FootIdentity name={user.displayName ?? user.fullName} role={user.role} />
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent
        {...form.menu}
        className="bg-card shadow-float z-(--z-popover) w-60 rounded-lg p-1.5"
      >
        <div className="mb-1 flex items-center gap-2.5 border-b px-2 pt-1.5 pb-2.5">
          <Avatar
            name={user.displayName ?? user.fullName}
            src={user.avatarUrl}
            tone="self"
            {...form.avatar}
          />
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold">
              {user.displayName ?? user.fullName}
            </p>
            <p className="text-muted-fg truncate text-xs">{user.email}</p>
          </div>
        </div>
        {entries.map((entry) =>
          entry.kind === "language" ? (
            <DropdownMenuItem
              key={entry.id}
              className={DECK_ITEM}
              disabled={preferenceStatus.phase === "saving"}
              onSelect={() =>
                void chooseAccountPreference({ locale: locale === "vi" ? "en" : "vi" })
              }
            >
              <entry.icon aria-hidden="true" />
              {t(entry.label)}
              <DropdownMenuMeta>{t(`settings.locale.${locale}`)}</DropdownMenuMeta>
            </DropdownMenuItem>
          ) : (
            <LinkRow key={entry.id} entry={entry} />
          ),
        )}
        <DropdownMenuItem
          className={DECK_ITEM}
          disabled={preferenceStatus.phase === "saving"}
          onSelect={() =>
            void chooseAccountPreference({ theme: dark ? "light" : "dark" })
          }
        >
          {dark ? <Sun aria-hidden="true" /> : <Moon aria-hidden="true" />}
          {dark ? t("common.lightMode") : t("common.darkMode")}
        </DropdownMenuItem>
        {preferenceStatus.phase === "failed" && (
          <div role="alert" className="text-meta px-2 py-1">
            {t("settings.preferenceFailed")}
          </div>
        )}
        {preferenceStatus.unsupportedZone && (
          <>
            <div role="alert" className="text-meta px-2 py-1">
              {t("settings.accountZoneUnsupported")}
            </div>
            <DropdownMenuItem
              onSelect={() => void refreshAccount().catch(() => undefined)}
            >
              {t("auth.transition.checkStatus")}
            </DropdownMenuItem>
          </>
        )}
        {(preferenceStatus.phase === "saving" ||
          preferenceStatus.phase === "failed") && (
          <DropdownMenuItem
            disabled={preferenceStatus.phase !== "failed"}
            onSelect={() => void retryAccountPreference()}
          >
            {t(preferenceMessage)}
          </DropdownMenuItem>
        )}
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

/**
 * AccountMenu is a console's account button and the menu it opens, in the
 * design deck's form: a 40px avatar button at the end of the top bar and a
 * 240px menu with the user's name and email, Settings, a switch between the
 * light and dark themes, and Sign out. With `sidebar` it is the row at the
 * foot of a console's sidebar instead: a square avatar, and while `expanded`
 * the name over a role line, with the same menu opening above it.
 *
 * `workspace` names the console the menu is in. In the teacher's, an Admin
 * also gets a link to the Admin console. In the Admin's, the entries are the
 * pages the user's permissions open, a link to the teacher workspace for a
 * user who has one, and for one who has not the way to change their password
 * and their language, which no other page of the console offers. A link to
 * the other console remembers the choice (`rememberWorkspace`).
 */
export function AccountMenu({
  workspace,
  settingsTo = "/teacher/settings",
  sidebar,
}: Readonly<{
  workspace?: StaffWorkspace;
  settingsTo?: string;
  sidebar?: SidebarForm;
}>) {
  const user = useAuthStore((s) => s.user);
  if (!user) return null;
  return (
    <DeckAccountMenu
      user={user}
      workspace={workspace}
      settingsTo={settingsTo}
      sidebar={sidebar}
    />
  );
}
