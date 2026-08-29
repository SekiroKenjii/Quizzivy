import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Card } from "@/components/ui/card";

/**
 * The shell for /login, per the deck's S-02: a wordmark over a single card, one
 * column at every width.
 *
 * There is no marketing panel beside it. §12 asks for calm and legitimate, and
 * for the student arriving from a join link this is the second screen they have
 * ever seen of the product -- a half-screen of brand copy would be the first
 * thing suggesting they are somewhere they should not be.
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
    <div className="min-h-svh p-4 pt-14">
      <div className="mx-auto w-full max-w-sm">
        <p className="mb-6 text-center text-lg font-semibold tracking-tight">
          {t("app.name")}
        </p>
        <Card className="gap-0 p-5">{children}</Card>
        {footer === undefined ? null : (
          <div className="text-muted-foreground mt-5 px-2 text-center text-xs leading-relaxed">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
