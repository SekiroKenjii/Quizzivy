import { useTranslation } from "react-i18next";
import { useAuthStore } from "@/stores/auth";
import { formatAgo } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

/** usePasswordChanged is when the signed-in user last changed their password, as a phrase for "Changed …", or null when the account never has (DG-54). */
export function usePasswordChanged(): string | null {
  const at = useAuthStore((s) => s.user?.passwordChangedAt ?? null);
  const locale = useLocale();
  const { t } = useTranslation();
  return at === null ? null : formatAgo(at, locale, t("common.agoJustNow"));
}
