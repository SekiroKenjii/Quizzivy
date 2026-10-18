import { Avatar } from "@/components/ui/avatar";
import {
  LastActive,
  RoleBadge,
  StatusPill,
} from "@/features/users/components/UserCells";
import type { AdminUser } from "@/features/users/api";
import type { Locale } from "@/lib/i18n";

/**
 * UserCard is an account where the list is too narrow for columns: the
 * avatar, the name and the email, and under them the role, the state and the
 * last activity.
 */
export function UserCard({
  user,
  locale,
}: Readonly<{ user: AdminUser; locale: Locale }>) {
  return (
    <span className="flex w-full min-w-0 items-start gap-3">
      <Avatar
        name={user.fullName}
        src={user.avatarUrl}
        className="size-8.5 text-[11.5px]"
      />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="block truncate text-sm font-medium">{user.fullName}</span>
        <span className="text-muted-fg block truncate text-[12.5px]">{user.email}</span>
        <span className="mt-0.5 flex flex-wrap items-center gap-1.5">
          <RoleBadge role={user.role} />
          <StatusPill user={user} />
          <span className="text-muted-fg text-xs">
            <LastActive at={user.lastActiveAt} locale={locale} />
          </span>
        </span>
      </span>
    </span>
  );
}
