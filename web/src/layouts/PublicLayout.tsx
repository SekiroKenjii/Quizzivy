import { Outlet } from "react-router";
import { useTranslation } from "react-i18next";

/**
 * §9: "logo + content, nothing else."
 *
 * This is what an anonymous visitor sees, including the join flow — the first
 * thing a new student encounters. §12: calm and legitimate, not a marketing
 * page. The deck's S-01 centres the wordmark and keeps the content near the
 * top, so a phone keyboard opening does not push the card off screen.
 */
export default function PublicLayout() {
  const { t } = useTranslation();
  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex items-center justify-center border-b px-4 py-3">
        <span className="text-sm font-semibold tracking-tight">{t("app.name")}</span>
      </header>
      <main className="flex-1 p-4 pt-10">
        <div className="mx-auto w-full max-w-sm">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
