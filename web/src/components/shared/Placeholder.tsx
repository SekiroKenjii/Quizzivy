import { useTranslation } from "react-i18next";

/**
 * Scaffolding. Every route in the tree resolves to a real component from T-0.10
 * so navigation, guards and code-splitting can be exercised before the screens
 * exist. Replaced feature by feature in Phases 1–4.
 */
export function Placeholder({ titleKey }: Readonly<{ titleKey: string }>) {
  const { t } = useTranslation();
  return (
    <section className="space-y-2">
      <h1 className="text-xl font-semibold tracking-tight">{t(titleKey)}</h1>
      <p className="text-muted-foreground text-sm">{t("common.comingSoon")}</p>
    </section>
  );
}
