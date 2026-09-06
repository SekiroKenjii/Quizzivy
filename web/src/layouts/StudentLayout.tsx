import { useState } from "react";
import { NavLink, Outlet, useMatches, useNavigate } from "react-router";
import { ArrowLeft, User } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { BrandLockup, BrandMark } from "@/components/shared/Brand";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import type { DetailShell } from "@/layouts/detailShell";
import { PageAsideSlot } from "@/layouts/slots";

interface StudentHandle {
  /** S-04/S-10's detail chrome below 1024px: a back arrow and the screen's name. */
  detail?: boolean;
  titleKey?: string;
}

/**
 * §9's student shell: "minimal top bar, no sidebar, mobile-first, safe-area
 * padding". Below 1024px that is S-03's bar, or S-04's back arrow on a detail
 * screen. From 1024px it is S-13: the bar with the student's name and menu,
 * and under it the admin's two columns -- a scrolling middle and F-11's panel,
 * which a screen fills through PageAside exactly as the teacher's screens do.
 */
export default function StudentLayout() {
  const { t } = useTranslation();
  const wide = useMediaQuery("(min-width: 1024px)");
  const matches = useMatches();
  const handle = [...matches].reverse().find((m) => isStudentHandle(m.handle))
    ?.handle as StudentHandle | undefined;
  const [own, setTitle] = useState<string | null>(null);
  const [asideSlot, setAsideSlot] = useState<HTMLDivElement | null>(null);
  const context = { setTitle } satisfies DetailShell;

  if (wide) {
    return (
      <div className="flex h-svh flex-col">
        <header className="bg-background shrink-0 border-b">
          <div className="flex h-14 items-center gap-3 px-6">
            <BrandLockup height={28} />
            <nav
              aria-label={t("nav.mainNavigation")}
              className="ml-4 flex items-center gap-1"
            >
              <NavLink to="/app" end className={link}>
                {t("student.myAssignments")}
              </NavLink>
              <NavLink to="/app/classes" className={link}>
                {t("student.classesNav")}
              </NavLink>
            </nav>
            <div className="ml-auto">
              <AccountMenu settingsTo="/app/settings" named />
            </div>
          </div>
        </header>
        <div data-columns className="flex min-h-0 flex-1">
          <main data-resize-middle className="min-w-0 flex-1 overflow-y-auto p-8">
            <PageAsideSlot.Provider value={asideSlot}>
              <Outlet context={context} />
            </PageAsideSlot.Provider>
          </main>
          <div ref={setAsideSlot} className="contents" />
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-svh flex-col">
      {handle?.detail ? (
        <DetailBar
          title={own ?? (handle.titleKey ? t(handle.titleKey) : t("app.name"))}
        />
      ) : (
        <header className="border-b">
          <div className="mx-auto flex h-14 max-w-[40rem] items-center justify-between gap-4 px-4">
            <BrandMark height={24} />
            <nav
              aria-label={t("nav.mainNavigation")}
              className="flex items-center gap-1"
            >
              <NavLink to="/app" end className={link}>
                {t("student.myAssignments")}
              </NavLink>
              <NavLink to="/app/classes" className={link}>
                {t("student.classesNav")}
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
      )}
      <main
        className="mx-auto w-full max-w-[40rem] flex-1 p-4"
        style={{ paddingBottom: "max(1rem, env(safe-area-inset-bottom))" }}
      >
        <Outlet context={context} />
      </main>
    </div>
  );
}

const link = ({ isActive }: { isActive: boolean }) =>
  cn(
    "inline-flex h-8 items-center rounded-md px-3 text-sm transition-colors",
    isActive
      ? "bg-secondary text-secondary-foreground"
      : "text-muted-foreground hover:text-foreground",
  );

/** The deck's S-10 detail shell: a back arrow and the screen's name, in place of the nav bar. */
function DetailBar({ title }: Readonly<{ title: string }>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
    <header className="flex h-14 items-center gap-2 border-b px-4">
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={t("common.back")}
        onClick={() => void (cameFromInside() ? navigate(-1) : navigate("/app"))}
      >
        <ArrowLeft aria-hidden="true" />
      </Button>
      <h1 className="truncate text-sm font-medium">{title}</h1>
    </header>
  );
}

// A deep link has no in-app history to go back to, so the arrow goes home.
function cameFromInside(): boolean {
  const state = window.history.state as { idx?: number } | null;
  return typeof state?.idx === "number" && state.idx > 0;
}

function isStudentHandle(handle: unknown): handle is StudentHandle {
  return typeof handle === "object" && handle !== null && "detail" in handle;
}
