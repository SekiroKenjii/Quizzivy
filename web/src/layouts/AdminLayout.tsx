import { useState } from "react";
import { Fragment } from "react";
import { NavLink, Outlet } from "react-router";
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

/**
 * §8: "sidebar + top bar. Collapsible sidebar ≤1280px. Minimum supported width
 * 768px."
 *
 * Desktop/tablet only (§1.1) — this is not made to work on a phone, and §16's
 * 360px requirement deliberately covers the student tree only. Data-dense is
 * fine here; §12 puts admin tables at ~40px rows.
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
      { to: "/admin/assignments", icon: ClipboardList, key: "nav.assignments" },
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
  const open = override ?? !isNarrow;
  const toggle = () => setOverride(!open);

  return (
    /**
     * The deck's `.shell`: the sidebar is the FIRST child of a flex row and
     * runs the full height, with the topbar inset beside it.
     *
     * Navigation is the most persistent thing on the screen, so it gets the
     * viewport edge and never moves; the topbar then belongs to the content
     * area, which is what its search actually searches. A-04 is the one board
     * that drops the sidebar entirely — the builder is a focus mode and gets
     * its own route outside this layout.
     */
    <div className="flex min-h-svh min-w-[768px]">
      <nav
        id="admin-sidebar"
        aria-label={t("nav.mainNavigation")}
        hidden={!open}
        className="flex w-60 shrink-0 flex-col gap-0.5 border-r p-2"
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

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="bg-background sticky top-0 z-10 border-b">
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
            <span className="text-base font-semibold tracking-tight">
              {t("app.name")}
            </span>

            {/* A-02's trigger: the palette is the only navigation model that
              survives an LMS-sized sidebar, so it sits in the chrome rather
              than on one screen. */}
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
              <AccountMenu />
            </div>
          </div>
        </header>

        <CommandPalette open={palette.open} onOpenChange={palette.setOpen} />

        <main className="min-w-0 flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
