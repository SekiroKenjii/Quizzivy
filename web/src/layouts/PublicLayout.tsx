import { Outlet } from "react-router";
import { useTranslation } from "react-i18next";

/**
 * §9: "logo + content, nothing else."
 *
 * This is what an anonymous visitor sees, including the join flow — the first
 * thing a new student encounters. §12: calm and legitimate, not a marketing
 * page.
 */
export default function PublicLayout() {
  const { t } = useTranslation();
  return (
    <div className="flex min-h-svh flex-col">
      <header className="border-b">
        <div className="mx-auto flex h-14 max-w-5xl items-center px-4">
          <span className="text-base font-semibold tracking-tight">
            {t("app.name")}
          </span>
        </div>
      </header>
      <main className="flex flex-1 items-center justify-center p-4">
        <Outlet />
      </main>
    </div>
  );
}
