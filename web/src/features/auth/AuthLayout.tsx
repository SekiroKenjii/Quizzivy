import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Card } from "@/components/ui/card";
import { BrandLockup } from "@/components/shared/Brand";

/**
 * The shell for /login and for the three failure screens: a brand panel on the
 * left, one card on the right.
 *
 * Below `lg` the panel is dropped entirely and the card carries the lockup
 * itself, because on a phone half a screen of brand is half a screen the
 * student cannot act in. That rule is why the panel may hold a drawing at all —
 * everything in it is decoration in the precise sense that the screen works
 * without it.
 */
export function AuthLayout({
  children,
  footer,
  art,
}: {
  children: ReactNode;
  /** Sits below the card, per the deck: it is about the product, not the form. */
  footer?: ReactNode;
  /**
   * The panel drawing the failure screens pass (E-01..E-03). Login passes
   * nothing and the panel keeps its two-element shape.
   */
  art?: ReactNode;
}) {
  const { t } = useTranslation();

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      <aside className="bg-primary text-primary-foreground hidden flex-col justify-between p-10 lg:flex">
        {/* on-dark, not color: the kit forbids the colour variant on a dark
            surface, and this panel is zinc-900 in every theme. */}
        <BrandLockup height={44} onDark />
        {art === undefined ? null : (
          <div className="flex justify-center py-6" aria-hidden="true">
            {art}
          </div>
        )}
        <p className="max-w-sm text-sm leading-relaxed opacity-80">
          {t("login.panelBlurb")}
        </p>
      </aside>

      <main className="flex items-center justify-center p-4">
        <div className="w-full max-w-sm">
          <div className="mb-6 flex justify-center lg:hidden">
            <BrandLockup height={35} />
          </div>
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
