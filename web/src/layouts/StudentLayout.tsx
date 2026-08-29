import { NavLink, Outlet } from "react-router";
import { User } from "lucide-react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

/**
 * §9: "minimal top bar, no sidebar, mobile-first, safe-area padding."
 *
 * §1.1 says students are often on a phone, and §16 requires 360px to work, so
 * this stays deliberately sparse. Safe-area insets are applied on `body`
 * (index.css) for left/right; the bottom inset is applied here because only
 * this tree has content that can reach the home indicator.
 */
export default function StudentLayout() {
  const { t } = useTranslation();

  const link = ({ isActive }: { isActive: boolean }) =>
    cn(
      "inline-flex h-8 items-center rounded-md px-3 text-sm transition-colors",
      isActive
        ? "bg-secondary text-secondary-foreground"
        : "text-muted-foreground hover:text-foreground",
    );

  return (
    <div className="flex min-h-svh flex-col">
      <header className="border-b">
        <div className="mx-auto flex h-14 max-w-3xl items-center justify-between gap-4 px-4">
          <span className="text-sm font-semibold tracking-tight">{t("app.name")}</span>
          <nav aria-label={t("nav.mainNavigation")} className="flex items-center gap-1">
            <NavLink to="/app" end className={link}>
              {t("student.myAssignments")}
            </NavLink>
            <NavLink to="/app/classes" className={link}>
              {t("student.myClasses")}
            </NavLink>
            <NavLink
              to="/app/settings"
              className="text-muted-foreground hover:text-foreground inline-flex size-8 items-center justify-center rounded-md transition-colors"
              aria-label={t("nav.settings")}
            >
              <User className="size-4" aria-hidden="true" />
            </NavLink>
          </nav>
        </div>
      </header>
      <main
        className="mx-auto w-full max-w-3xl flex-1 p-4"
        style={{ paddingBottom: "max(1rem, env(safe-area-inset-bottom))" }}
      >
        <Outlet />
      </main>
    </div>
  );
}
