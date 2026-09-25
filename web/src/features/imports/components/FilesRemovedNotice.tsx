import { useTranslation } from "react-i18next";
import { formatDate } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

/** FilesRemovedNotice says, calmly, that retention removed an import's files on the given day. */
export function FilesRemovedNotice({ at }: Readonly<{ at: string }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  return (
    <p role="status" className="bg-muted/40 rounded-md p-3 text-sm leading-relaxed">
      {t("imports.retention.removed", { date: formatDate(at, locale) })}
    </p>
  );
}
