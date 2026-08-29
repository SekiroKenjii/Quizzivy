import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/**
 * The two-column shell for /login and /join, per §12.
 *
 * The brand panel is hidden below `lg`, so a phone gets the form alone.
 */
export function AuthLayout({ children }: { children: ReactNode }) {
  const { t } = useTranslation();

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      <aside className="bg-primary text-primary-foreground hidden flex-col justify-between p-10 lg:flex">
        <span className="text-lg font-semibold tracking-tight">{t("app.name")}</span>
        <p className="max-w-sm text-sm leading-relaxed opacity-80">
          {t("login.panelBlurb")}
        </p>
      </aside>

      <main className="flex items-center justify-center p-6 lg:p-10">
        <div className="w-full max-w-sm">{children}</div>
      </main>
    </div>
  );
}
