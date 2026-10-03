import { useEffect, useMemo, useRef, useState } from "react";
import { Link, NavLink, Outlet, useLocation, useMatches } from "react-router";
import {
  ArrowLeft,
  BookOpen,
  ChartColumn,
  CircleUser,
  House,
  Users,
  type LucideIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { registerSkeleton } from "@/app/boot/handoff";
import { modules } from "@/app/modules";
import { BrandMark } from "@/components/shared/Brand";
import { DeckScale } from "@/components/ui/deck-scale";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { useDueSoonCount } from "@/features/assignments/dueSoon";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import type { DetailShell } from "@/layouts/detailShell";
import { StudentSkeleton } from "@/layouts/StudentSkeleton";

registerSkeleton("student", () => <StudentSkeleton />);

/**
 * StudentDetail is what a detail route under StudentLayout declares as
 * `handle.detail`: the title its phone header shows and the path its back
 * arrow goes to.
 */
export interface StudentDetail {
  titleKey: string;
  back: string;
}

interface Destination {
  to: string;
  label: string;
  icon: LucideIcon;
  count?: number;
}

/**
 * StudentLayout is the student console's shell, as the design deck draws it.
 * From 768px it is a 60px top bar with the destinations beside the logo;
 * below that the destinations move to a bottom tab bar, and a detail route
 * swaps the logo for a back arrow and its title and hides the tab bar. One
 * outlet serves both, so a page keeps its state when the width crosses 768.
 * A destination whose module has not shipped is absent (`app/modules`).
 */
export default function StudentLayout() {
  const { t } = useTranslation();
  const wide = useMediaQuery("(min-width: 768px)");
  const detail = useDetail();
  const dueSoon = useDueSoonCount();
  const [own, setTitle] = useState<string | null>(null);
  const context = useMemo(() => ({ setTitle }) satisfies DetailShell, []);
  const main = useRef<HTMLElement>(null);
  const { pathname } = useLocation();

  useEffect(() => {
    if (main.current) main.current.scrollTop = 0;
  }, [pathname]);

  const destinations: Destination[] = [
    { to: "/app", label: t("student.shell.home"), icon: House, count: dueSoon },
    { to: "/app/classes", label: t("student.classesNav"), icon: Users },
    ...(modules.learn
      ? [{ to: "/app/learn", label: t("student.shell.learn"), icon: BookOpen }]
      : []),
    ...(modules.grades
      ? [{ to: "/app/grades", label: t("student.shell.grades"), icon: ChartColumn }]
      : []),
  ];
  const tabs: Destination[] = [
    ...destinations,
    { to: "/app/settings", label: t("student.shell.me"), icon: CircleUser },
  ];
  const phoneDetail = !wide && detail !== null;

  return (
    <DeckScale className="bg-bg text-fg @container/student flex h-svh flex-col overflow-hidden">
      <header
        className={cn(
          "bg-bg flex h-15 flex-none items-center gap-3.5 border-b",
          wide ? "px-6" : "px-3.5",
        )}
      >
        {phoneDetail ? (
          <>
            <Link
              to={detail.back}
              aria-label={t("common.back")}
              className="hover:bg-hover -ml-1.5 grid size-9 flex-none place-items-center rounded-md"
            >
              <ArrowLeft className="size-4.5" aria-hidden="true" />
            </Link>
            <p className="text-md min-w-0 truncate font-semibold">
              {own ?? t(detail.titleKey)}
            </p>
          </>
        ) : (
          <Link
            to="/app"
            aria-label={t("student.shell.homeLink")}
            className="flex-none"
          >
            <BrandMark height={24} wordmark="header" theme="auto" />
          </Link>
        )}
        {wide && (
          <nav
            aria-label={t("nav.mainNavigation")}
            className="ml-3.5 flex items-center gap-0.5"
          >
            {destinations.map((destination) => (
              <NavLink
                key={destination.to}
                to={destination.to}
                end
                className={({ isActive }) =>
                  cn(
                    "hover:bg-hover hover:text-fg flex h-9 items-center gap-[0.4375rem] rounded-md px-3 text-base whitespace-nowrap",
                    isActive ? "bg-hover text-fg font-medium" : "text-muted-fg",
                  )
                }
              >
                <destination.icon className="size-4" aria-hidden="true" />
                {destination.label}
                <DueSoon count={destination.count} placement="nav" />
              </NavLink>
            ))}
          </nav>
        )}
        <div className="ml-auto flex items-center gap-1">
          <AccountMenu settingsTo="/app/settings" deck />
        </div>
      </header>
      <main
        ref={main}
        className={cn(
          "student-surface min-h-0 w-full min-w-0 flex-1 overflow-y-auto",
          wide ? "px-6 pt-8 pb-12" : "px-4 pt-4.5 pb-7",
        )}
      >
        <Outlet context={context} />
      </main>
      {!wide && detail === null && (
        <nav
          aria-label={t("nav.mainNavigation")}
          data-slot="student-tabs"
          className="bg-bg grid flex-none border-t px-1 pt-1 pb-2"
          style={{ gridTemplateColumns: `repeat(${tabs.length}, minmax(0, 1fr))` }}
        >
          {tabs.map((tab) => (
            <NavLink
              key={tab.to}
              to={tab.to}
              end
              className={({ isActive }) =>
                cn(
                  "group text-2xs flex h-13 flex-col items-center justify-center gap-0.5",
                  isActive ? "text-fg font-semibold" : "text-muted-fg font-medium",
                )
              }
            >
              <span className="group-aria-[current=page]:bg-brand-soft relative grid h-7 w-12 place-items-center rounded-full">
                <tab.icon className="size-[1.1875rem]" aria-hidden="true" />
                <DueSoon count={tab.count} placement="tab" />
              </span>
              {tab.label}
            </NavLink>
          ))}
        </nav>
      )}
    </DeckScale>
  );
}

function DueSoon({
  count,
  placement,
}: Readonly<{ count: number | undefined; placement: "nav" | "tab" }>) {
  const { t } = useTranslation();
  if (!count) return null;
  return (
    <span
      className={cn(
        "bg-brand text-brand-fg inline-flex items-center justify-center rounded-full leading-none",
        placement === "nav"
          ? "text-2xs h-4.5 min-w-4.5 px-[0.3125rem] font-semibold"
          : "absolute -top-0.5 right-1.5 h-4 min-w-4 px-1 text-[0.625rem] font-bold",
      )}
    >
      <span aria-hidden="true">{count}</span>
      <span className="sr-only">{t("student.shell.dueSoon", { count })}</span>
    </span>
  );
}

function useDetail(): StudentDetail | null {
  const matches = useMatches();
  for (const match of [...matches].reverse()) {
    const handle = match.handle;
    if (typeof handle === "object" && handle !== null && "detail" in handle) {
      return handle.detail as StudentDetail;
    }
  }
  return null;
}
