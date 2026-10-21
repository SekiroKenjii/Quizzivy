import { Link } from "react-router";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import {
  ChevronRight,
  GraduationCap,
  HardDrive,
  KeyRound,
  MailWarning,
  MailX,
  Sparkles,
  UserX,
  type LucideIcon,
} from "lucide-react";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { EmptyState } from "@/components/shared/ListState";
import { IconTile, type IconTileProps } from "@/components/shared/IconTile";
import { formatRelative } from "@/lib/i18n/datetime";
import { credits } from "@/features/ai/credits";
import { useLocale } from "@/lib/i18n/useLocale";
import type { AdminAttention } from "../api";

type Row = Readonly<{
  icon: LucideIcon;
  tone: NonNullable<IconTileProps["tone"]>;
  to: string;
  title: string;
  hint: string;
}>;

const READINGS = [
  ["seatsPercent", "seats"],
  ["teachersPercent", "teachers"],
  ["storagePercent", "files"],
  ["mediaPercent", "media"],
  ["importsPercent", "imports"],
  ["databasePercent", "database"],
] as const;

function readings(attention: AdminAttention, t: TFunction): string {
  const detail = attention.detail ?? {};
  return READINGS.flatMap(([field, name]) => {
    const percent = detail[field];
    return percent === undefined
      ? []
      : [t(`adminOverview.attention.storage.${name}`, { percent })];
  }).join(", ");
}

/**
 * AttentionList is the Overview's list of what needs doing, most urgent first
 * as the server sends it, each row a link to where it is dealt with. With no
 * rows it says so.
 */
export function AttentionList({
  items,
}: Readonly<{ items: readonly AdminAttention[] }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  if (items.length === 0) {
    return (
      <div className="p-4.5 pt-0">
        <EmptyState>{t("adminOverview.attention.none")}</EmptyState>
      </div>
    );
  }
  const rows: Row[] = items.map((item) => {
    const { count, detail } = item;
    switch (item.kind) {
      case "must_change_password":
        return {
          icon: KeyRound,
          tone: "warning",
          to: `${ADMIN_PATHS.users}?status=must_change_password`,
          title: t("adminOverview.attention.mustChangePassword", { count }),
          hint: t("adminOverview.attention.mustChangePasswordHint"),
        };
      case "never_signed_in":
        return {
          icon: UserX,
          tone: "neutral",
          to: `${ADMIN_PATHS.users}?status=never_signed_in`,
          title: t("adminOverview.attention.neverSignedIn", { count }),
          hint:
            detail?.oldestCreatedAt === undefined
              ? ""
              : t("adminOverview.attention.neverSignedInHint", {
                  when: formatRelative(detail.oldestCreatedAt, locale),
                }),
        };
      case "disabled_teacher_classes":
        return {
          icon: GraduationCap,
          tone: "info",
          to: `${ADMIN_PATHS.classes}?teacher=disabled`,
          title: t("adminOverview.attention.disabledTeacher", { count }),
          hint: t("adminOverview.attention.disabledTeacherHint", {
            className: detail?.className ?? "",
            teacherName: detail?.teacherName ?? "",
          }),
        };
      case "storage":
        return {
          icon: HardDrive,
          tone: "neutral",
          to: `${ADMIN_PATHS.settings}/plan`,
          title: t("adminOverview.attention.storage.title", {
            readings: readings(item, t),
          }),
          hint: t("adminOverview.attention.storageHint"),
        };
      case "email_held":
        return {
          icon: MailWarning,
          tone: "warning",
          to: `${ADMIN_PATHS.settings}/plan`,
          title: t("adminOverview.attention.emailHeld", { count }),
          hint: t("adminOverview.attention.emailHeldHint"),
        };
      case "email_expired":
        return {
          icon: MailX,
          tone: "warning",
          to: `${ADMIN_PATHS.settings}/plan`,
          title: t("adminOverview.attention.emailExpired", { count }),
          hint: t("adminOverview.attention.emailExpiredHint"),
        };
      case "ai_credit_requests":
        return {
          icon: Sparkles,
          tone: "accent",
          to: ADMIN_PATHS.ai,
          title: t("adminOverview.attention.creditRequests", { count }),
          hint: t("adminOverview.attention.creditRequestsHint", {
            teacherName: detail?.teacherName ?? "",
            used: credits(detail?.used ?? 0, locale),
            allowance: credits(detail?.allowance ?? 0, locale),
          }),
        };
    }
  });
  return (
    <ul>
      {rows.map((row, position) => (
        <li key={items[position]?.kind}>
          <Link
            to={row.to}
            className="hover:bg-muted flex items-center gap-3 border-t px-4.5 py-3 transition-colors ease-[ease]"
          >
            <IconTile icon={row.icon} size={30} tone={row.tone} />
            <span className="min-w-0 flex-1">
              <span className="text-ui block leading-normal font-medium">
                {row.title}
              </span>
              {row.hint === "" ? null : (
                <span className="text-muted-fg text-meta block leading-normal">
                  {row.hint}
                </span>
              )}
            </span>
            <ChevronRight
              aria-hidden="true"
              className="text-muted-fg size-4 shrink-0"
            />
          </Link>
        </li>
      ))}
    </ul>
  );
}
