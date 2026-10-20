import { useTranslation } from "react-i18next";
import { KeyRound, type LucideIcon } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { iconOf } from "@/features/roles/roleLook";
import type { AdminUser, AdminUserRole } from "@/features/users/api";
import { statusKeyOf, type StatusKey } from "@/features/users/userText";
import type { Locale } from "@/lib/i18n";
import { formatDateTime, formatRelativeShort } from "@/lib/i18n/datetime";
import { roleName } from "@/lib/roleName";
import { cn } from "@/lib/utils";

const STATUS_BADGE: Record<
  StatusKey,
  { variant: "success" | "warning" | "secondary" | "danger"; className?: string }
> = {
  active: { variant: "success" },
  mustChangePassword: { variant: "warning" },
  neverSignedIn: { variant: "secondary", className: "bg-muted text-muted-fg" },
  disabled: { variant: "danger" },
};

function RoleGlyph({ icon: Icon }: Readonly<{ icon: LucideIcon }>) {
  return <Icon aria-hidden="true" className="size-3 flex-none" />;
}

/**
 * RoleBadge is an account's role in the deck's outline badge with the role's
 * own icon. A long name truncates and is the badge's title.
 */
export function RoleBadge({ role }: Readonly<{ role: AdminUserRole }>) {
  const { t } = useTranslation();
  const name = roleName(role, t);
  return (
    <Badge title={name} className="text-fg max-w-full gap-1.25 px-2 font-medium">
      <RoleGlyph icon={iconOf(role.icon)} />
      <span className="min-w-0 truncate">{name}</span>
    </Badge>
  );
}

/**
 * StatusPill is an account's state in the deck's pill: success for active,
 * warning for a temporary password still in use, muted for never signed in,
 * danger for disabled. `short` swaps the long label of a temporary password
 * for its short form while the Last active column is folded away, and the
 * full label stays the pill's title.
 */
export function StatusPill({
  user,
  short = false,
}: Readonly<{ user: Pick<AdminUser, "status" | "lastActiveAt">; short?: boolean }>) {
  const { t } = useTranslation();
  const key = statusKeyOf(user);
  const look = STATUS_BADGE[key];
  const label = t(`users.status.${key}`);
  const text =
    short && key === "mustChangePassword"
      ? t("users.status.mustChangePasswordShort")
      : label;
  return (
    <Badge
      variant={look.variant}
      dot
      title={label}
      className={cn("max-w-full px-2 font-medium", look.className)}
    >
      <span className="min-w-0 truncate">{text}</span>
    </Badge>
  );
}

/**
 * Person is the User column's first cell: the 32px avatar with the initials of
 * DG-22 (the photo when the account has one), the name, and under it `sub`,
 * the email with the columns that have no room folded in.
 */
export function Person({
  user,
  sub,
}: Readonly<{ user: Pick<AdminUser, "fullName" | "avatarUrl">; sub: string }>) {
  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <Avatar name={user.fullName} src={user.avatarUrl} className="text-[11.5px]" />
      <span className="min-w-0">
        <span className="block truncate font-medium">{user.fullName}</span>
        <span className="text-muted-fg block truncate text-xs">{sub}</span>
      </span>
    </span>
  );
}

/**
 * SignIn is the ways an account can sign in with: Password with the key icon
 * and Google with its mark, the one provider mark the Users table draws
 * (AGENTS.md, Design). An account with neither, an invite not yet used,
 * reads "None yet".
 */
export function SignIn({ signIn }: Readonly<{ signIn: AdminUser["signIn"] }>) {
  const { t } = useTranslation();
  if (!signIn.password && !signIn.google)
    return (
      <span className="text-muted-fg text-[12.5px]">{t("users.signIn.none")}</span>
    );
  return (
    <span className="text-muted-fg flex min-w-0 items-center gap-1.5 overflow-hidden text-[12.5px] whitespace-nowrap">
      {signIn.password ? (
        <span className="inline-flex items-center gap-1">
          <KeyRound aria-hidden="true" className="size-3.25 flex-none" />
          {t("users.signIn.password")}
        </span>
      ) : null}
      {signIn.google ? (
        <span className="inline-flex items-center gap-1">
          <GoogleMark className="size-3.25 flex-none" />
          {t("users.signIn.google")}
        </span>
      ) : null}
    </span>
  );
}

/**
 * ClassesCell is an account's classes in one line: the name of the first, which
 * truncates, then "+N" for the rest. A student in no class reads "No class
 * yet" and anyone else "—".
 */
export function ClassesCell({
  classes,
  student,
}: Readonly<{ classes: AdminUser["classes"]; student: boolean }>) {
  const { t } = useTranslation();
  if (classes.count === 0)
    return (
      <span className="text-muted-fg text-[13px]">
        {student ? t("users.noClass") : "—"}
      </span>
    );
  const rest = classes.count - 1;
  const first = classes.firstName ?? "";
  return (
    <span
      className="text-muted-fg flex min-w-0 items-center gap-1.5 text-[13px] whitespace-nowrap"
      title={first}
    >
      <span className="min-w-0 truncate">{first}</span>
      {rest > 0 ? (
        <span className="flex-none tabular-nums">
          <span aria-hidden="true">{t("users.moreCount", { n: rest })}</span>
          <span className="sr-only">{t("users.moreClasses", { count: rest })}</span>
        </span>
      ) : null}
    </span>
  );
}

/**
 * LastActive is when an account last made an authenticated request: the bare
 * short value (D-017) in a `<time>` holding the exact moment, "Never" before
 * the first request. The cell never wraps.
 */
export function LastActive({
  at,
  locale,
}: Readonly<{ at: string | null; locale: Locale }>) {
  const { t } = useTranslation();
  if (at === null) return <>{t("users.neverActive")}</>;
  return (
    <time
      dateTime={at}
      title={formatDateTime(at, locale)}
      className="whitespace-nowrap"
    >
      {formatRelativeShort(at, locale, t("common.agoNow"))}
    </time>
  );
}
