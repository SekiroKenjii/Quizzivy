import { useTranslation } from "react-i18next";

/**
 * Placeholder. The password form and the Google flow (O-13: our own
 * authorization request, not the GIS SDK) arrive in T-1.11.
 *
 * §16 names `/login` rendering as a Phase 0 exit criterion, which is what this
 * satisfies.
 */
export default function LoginPage() {
  const { t } = useTranslation();
  return (
    <div className="w-full max-w-sm space-y-2 rounded-lg border p-6">
      <h1 className="text-xl font-semibold tracking-tight">{t("app.name")}</h1>
      <p className="text-muted-foreground text-sm">{t("common.comingSoon")}</p>
    </div>
  );
}
