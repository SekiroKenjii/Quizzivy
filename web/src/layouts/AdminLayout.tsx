import { useState } from "react";
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
import { SignOutButton } from "@/features/auth/SignOutButton";
import { useMediaQuery } from "@/hooks/useMediaQuery";

/**
 * §8: "sidebar + top bar. Collapsible sidebar ≤1280px. Minimum supported width
 * 768px."
 *
 * Desktop/tablet only (§1.1) — this is not made to work on a phone, and §16's
 * 360px requirement deliberately covers the student tree only. Data-dense is
 * fine here; §12 puts admin tables at ~40px rows.
 */

const NAV = [
  { to: "/admin", end: true, icon: LayoutDashboard, key: "nav.dashboard" },
  { to: "/admin/tests", icon: FileText, key: "nav.tests" },
  { to: "/admin/question-bank", icon: Library, key: "nav.questionBank" },
  { to: "/admin/media", icon: AudioLines, key: "nav.media" },
  { to: "/admin/assignments", icon: ClipboardList, key: "nav.assignments" },
  { to: "/admin/students", icon: Users, key: "nav.students" },
  { to: "/admin/classes", icon: GraduationCap, key: "nav.classes" },
  { to: "/admin/settings", icon: Settings, key: "nav.settings" },
] as const;

export default function AdminLayout() {
  const { t } = useTranslation();
  // §8's breakpoint: at or below 1280px the sidebar collapses by default.
  const isNarrow = useMediaQuery("(max-width: 1280px)");
  const [override, setOverride] = useState<boolean | null>(null);
  const open = override ?? !isNarrow;
  const toggle = () => setOverride(!open);

  return (
    <div className="flex min-h-svh min-w-[768px] flex-col">
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
          <div className="ml-auto">
            <SignOutButton />
          </div>
        </div>
      </header>

      <div className="flex flex-1">
        <nav
          id="admin-sidebar"
          aria-label={t("nav.mainNavigation")}
          hidden={!open}
          className="w-56 shrink-0 border-r p-2"
        >
          <ul className="space-y-0.5">
            {NAV.map((item) => (
              <li key={item.to}>
                <NavLink
                  to={item.to}
                  end={"end" in item ? item.end : false}
                  className={({ isActive }) =>
                    cn(
                      "focus-visible:ring-ring flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors focus-visible:ring-2 focus-visible:outline-none",
                      isActive
                        ? "bg-secondary text-secondary-foreground font-medium"
                        : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground",
                    )
                  }
                >
                  <item.icon className="size-4 shrink-0" aria-hidden="true" />
                  {t(item.key)}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>

        <main className="min-w-0 flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
