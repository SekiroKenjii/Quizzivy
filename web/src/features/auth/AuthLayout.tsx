import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Card } from "@/components/ui/card";

/**
 * The shell for /login: a brand panel on the left, the form card on the right.
 *
 * The card is the same one S-02 draws; it sits in the right half rather than
 * centred on the page. Below `lg` the panel is dropped entirely and the card
 * carries the wordmark itself, because on a phone half a screen of brand is
 * half a screen the student cannot type into.
 */
export function AuthLayout({
  children,
  footer,
}: {
  children: ReactNode;
  /** Sits below the card, per the deck: it is about the product, not the form. */
  footer?: ReactNode;
}) {
  const { t } = useTranslation();

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      <aside className="bg-primary text-primary-foreground hidden flex-col justify-between p-10 lg:flex">
        <span className="text-lg font-semibold tracking-tight">{t("app.name")}</span>
        <p className="max-w-sm text-sm leading-relaxed opacity-80">
          {t("login.panelBlurb")}
        </p>
      </aside>

      <main className="flex items-center justify-center p-4">
        <div className="w-full max-w-sm">
          <p className="mb-6 text-center text-lg font-semibold tracking-tight lg:hidden">
            {t("app.name")}
          </p>
          <Card className="gap-0 p-5">{children}</Card>
          {footer === undefined ? null : (
            <div className="text-muted-foreground mt-5 px-2 text-center text-xs leading-relaxed">
              {footer}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
