import { Fragment, useState } from "react";
import { NavLink, Outlet } from "react-router";
import { PageAsideSlot, PageBarSlot, PageRailSlot } from "@/layouts/slots";
import { useTranslation } from "react-i18next";
import {
  LayoutDashboard,
  FileText,
  Library,
  AudioLines,
  ClipboardList,
  Users,
  GraduationCap,
  Settings,
  PanelLeft,
  SquarePen,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { CommandPalette } from "@/features/search/CommandPalette";
import {
  commandKeyLabel,
  useCommandPalette,
} from "@/features/search/useCommandPalette";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useQuery } from "@tanstack/react-query";
import { getDashboard } from "@/features/dashboard/api";
import { NotificationsButton } from "@/features/dashboard/NotificationsButton";
import { BrandLockup } from "@/components/shared/Brand";

/**
 * §8: "sidebar + top bar. Collapsible sidebar ≤1280px. Minimum supported width
 * 768px."
 */

// Grouped as the design deck's sidebar template groups them: authoring above,
// the class-facing work below, settings pinned to the bottom.
const SECTIONS = [
  {
    label: "nav.sectionTeaching",
    items: [
      { to: "/admin", end: true, icon: LayoutDashboard, key: "nav.dashboard" },
      { to: "/admin/tests", icon: FileText, key: "nav.tests" },
      { to: "/admin/question-bank", icon: Library, key: "nav.questionBank" },
      { to: "/admin/media", icon: AudioLines, key: "nav.media" },
    ],
  },
  {
    label: "nav.sectionClasses",
    items: [
      {
        to: "/admin/assignments",
        icon: ClipboardList,
        key: "nav.assignments",
        count: "openAssignments",
      },
      {
        to: "/admin/grading",
        icon: SquarePen,
        key: "nav.grading",
        count: "awaitingGrading",
      },
      { to: "/admin/students", icon: Users, key: "nav.students" },
      { to: "/admin/classes", icon: GraduationCap, key: "nav.classes" },
    ],
  },
] as const;

const SETTINGS = {
  to: "/admin/settings",
  icon: Settings,
  key: "nav.settings",
} as const;

const itemClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    "focus-visible:ring-ring flex items-center gap-2.5 rounded-md px-3 py-[0.4375rem] text-[0.8125rem] transition-colors focus-visible:ring-2 focus-visible:outline-none",
    isActive
      ? "bg-secondary text-secondary-foreground font-medium"
      : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground",
  );

export default function AdminLayout() {
  const { t } = useTranslation();
  // §8's breakpoint: at or below 1280px the sidebar collapses by default.
  const isNarrow = useMediaQuery("(max-width: 1280px)");
  const [override, setOverride] = useState<boolean | null>(null);
  const palette = useCommandPalette();

  // A-00 puts a count beside the class-facing items.
  const summary = useQuery({
    queryKey: ["admin-dashboard"],
    queryFn: ({ signal }) => getDashboard(signal),
    staleTime: 60_000,
  });
  const open = override ?? !isNarrow;
  const toggle = () => setOverride(!open);

  // The elements a screen's PageHeader bar and PageAside portal into; see layouts/slots.ts.
  const [barSlot, setBarSlot] = useState<HTMLDivElement | null>(null);
  const [asideSlot, setAsideSlot] = useState<HTMLDivElement | null>(null);
  const [railSlot, setRailSlot] = useState<HTMLDivElement | null>(null);
  return (
    <div className="flex h-svh min-w-[768px] overflow-hidden">
      <nav
        id="admin-sidebar"
        aria-label={t("nav.mainNavigation")}
        hidden={!open}
        className="flex w-60 shrink-0 flex-col gap-0.5 overflow-y-auto border-r p-2"
      >
        {SECTIONS.map((section) => (
          <Fragment key={section.label}>
            <p className="text-muted-foreground px-3 pt-3 pb-1 text-[0.6875rem] font-semibold tracking-[0.06em] uppercase">
              {t(section.label)}
            </p>
            {section.items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={"end" in item ? item.end : false}
                className={itemClass}
              >
                <item.icon className="size-4 shrink-0" aria-hidden="true" />
                {t(item.key)}
                {"count" in item && summary.data ? (
                  <span className="text-muted-foreground ml-auto text-xs tabular-nums">
                    {summary.data[item.count]}
                  </span>
                ) : null}
              </NavLink>
            ))}
          </Fragment>
        ))}

        <div className="mt-auto">
          <NavLink to={SETTINGS.to} className={itemClass}>
            <SETTINGS.icon className="size-4 shrink-0" aria-hidden="true" />
            {t(SETTINGS.key)}
          </NavLink>
        </div>
      </nav>

      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        {/* Outside the scroll container now, so it stays put without sticky. */}
        <header className="bg-background shrink-0 border-b">
          <div className="flex h-14 items-center gap-3 px-4">
            <button
              type="button"
              onClick={toggle}
              aria-label={open ? t("nav.closeMenu") : t("nav.openMenu")}
              aria-expanded={open}
              aria-controls="admin-sidebar"
              className="hover:bg-secondary focus-visible:ring-ring rounded-md p-2 transition-colors focus-visible:ring-2 focus-visible:outline-none"
            >
              <PanelLeft className="size-5" aria-hidden="true" />
            </button>
            <BrandLockup height={28} />

            <Button
              variant="outline"
              size="sm"
              className="text-muted-foreground ml-4 w-80 justify-start font-normal"
              onClick={() => palette.setOpen(true)}
            >
              <Search aria-hidden="true" />
              {t("palette.open")}
              <span className="ml-auto flex items-center gap-0.5">
                <Kbd>{commandKeyLabel()}</Kbd>
                <Kbd>{t("palette.keyK")}</Kbd>
              </span>
            </Button>

            <div className="ml-auto flex items-center gap-2">
              <NotificationsButton />
              <AccountMenu />
            </div>
          </div>
        </header>

        <div ref={setBarSlot} className="shrink-0" />

        <CommandPalette open={palette.open} onOpenChange={palette.setOpen} />

        <div className="flex min-h-0 flex-1">
          <div ref={setRailSlot} className="contents" />
          <main className="min-w-0 flex-1 overflow-y-auto p-6">
            <PageBarSlot.Provider value={barSlot}>
              <PageAsideSlot.Provider value={asideSlot}>
                <PageRailSlot.Provider value={railSlot}>
                  <Outlet />
                </PageRailSlot.Provider>
              </PageAsideSlot.Provider>
            </PageBarSlot.Provider>
          </main>
          <div ref={setAsideSlot} className="contents" />
        </div>
      </div>
    </div>
  );
}
