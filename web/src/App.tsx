import { useTranslation } from "react-i18next";

/**
 * Placeholder shell. The router, the three route trees and the four layouts
 * arrive in T-0.10; this exists so the build has an entry point.
 */
export default function App() {
  const { t } = useTranslation();

  return (
    <main className="mx-auto flex min-h-svh max-w-md flex-col justify-center gap-4 p-6">
      <h1 className="text-2xl font-semibold tracking-tight">{t("app.name")}</h1>
      <p className="text-muted-foreground text-sm">{t("common.loading")}</p>
    </main>
  );
}
