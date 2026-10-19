import { useMemo } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowRight } from "lucide-react";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { EmptyState } from "@/components/shared/ListState";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Avatar } from "@/components/ui/avatar";
import { humanise, isGone, sentenceOf } from "@/features/audit/actions";
import type { AuditEntry } from "@/features/audit/api";
import { formatRelativeShort } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

/**
 * RecentChanges is the Overview's newest audit entries, each as the actor and
 * the sentence that words the action, and a link to the whole log.
 */
export function RecentChanges({
  entries,
}: Readonly<{ entries: readonly AuditEntry[] }>) {
  const { t, i18n } = useTranslation();
  const locale = useLocale();
  const sentences = useMemo(
    () =>
      i18n.getResource(locale, "translation", "audit.action") as Record<string, string>,
    [i18n, locale],
  );
  const entityOf = (entry: AuditEntry) => {
    const key = `audit.entity.types.${entry.entity.type}`;
    const type = i18n.exists(key) ? t(key) : humanise(entry.entity.type);
    const { label } = entry.entity;
    if (label !== null) return t("audit.entity.named", { type, label });
    return isGone(entry.entity) ? t("audit.entity.gone", { type }) : type;
  };
  return (
    <SettingsCard
      title={t("adminOverview.recent.title")}
      headerClassName="border-b-0 pb-2.5"
      action={
        <Link
          to={ADMIN_PATHS.audit}
          className="text-muted-fg hover:text-fg flex items-center gap-1 text-sm transition-colors ease-[ease]"
        >
          {t("adminOverview.recent.all")}
          <ArrowRight aria-hidden="true" className="size-3.5" />
        </Link>
      }
      className="min-w-0 flex-[2_1_32.5rem] overflow-hidden"
    >
      {entries.length === 0 ? (
        <div className="p-4.5 pt-0">
          <EmptyState>{t("adminOverview.recent.none")}</EmptyState>
        </div>
      ) : (
        <ul>
          {entries.map((entry) => {
            const name = entry.actor?.name ?? t("audit.system");
            return (
              <li
                key={entry.id}
                className="flex items-center gap-3 border-t px-4.5 py-2.5"
              >
                <Avatar name={name} src={entry.actor?.avatarUrl} size="28" />
                <span className="text-ui min-w-0 flex-1 leading-snug">
                  <span className="font-medium">{name}</span>{" "}
                  {sentenceOf(sentences, entry.action)} · {entityOf(entry)}
                </span>
                <span className="text-muted-fg text-meta whitespace-nowrap">
                  {formatRelativeShort(entry.occurredAt, locale, t("common.agoNow"))}
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </SettingsCard>
  );
}
