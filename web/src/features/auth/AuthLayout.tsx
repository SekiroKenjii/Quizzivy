import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/**
 * The shell for the signed-out screens: a two-column split, form on the right.
 *
 * §12 asks for a "single centered card" on **join** screens specifically --
 * that is about the first thing a new student sees, and /join keeps it. /login
 * is not a join screen, so the split is available, and it earns its place by
 * giving the form a comfortable column on a laptop instead of a narrow card
 * floating in white space.
 *
 * Everything else is §12 as written: theme tokens rather than literal colours,
 * `rounded-md` controls, no gradient, no blur, no glow. The dark panel is
 * `bg-primary`, which is the zinc-900 §12 mandates for primary surfaces -- so
 * the split cannot drift to blue, and dark mode stays a token change.
 *
 * Below `lg` the panel is gone entirely rather than stacked. §1.1 puts students
 * on phones; a decorative band above the form would push the fields under the
 * fold for no gain.
 */
export function AuthLayout({ children }: { children: ReactNode }) {
  const { t } = useTranslation();

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      <aside className="bg-primary text-primary-foreground hidden flex-col justify-between p-10 lg:flex">
        <span className="text-lg font-semibold tracking-tight">{t("app.name")}</span>
        {/* One plain sentence. Not a testimonial: an invented quote from an
            invented person is exactly the "marketing page" §12 warns against,
            and this product has real users who did not say it. */}
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
