import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Outlet, useLocation, useMatches } from "react-router";
import { useTranslation } from "react-i18next";
import { registerSkeleton } from "@/app/boot/handoff";
import { modules } from "@/app/modules";
import { DeckScale } from "@/components/ui/deck-scale";
import { NotificationBell } from "@/features/notifications/components/NotificationBell";
import { CommandPalette } from "@/features/search/CommandPalette";
import { useCommandPalette } from "@/features/search/useCommandPalette";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { registerContentElement } from "@/layouts/shell/contentWidth";
import { CrumbTailContext, trailOf, type PageCrumb } from "@/layouts/shell/crumbs";
import {
  teacherHandleOf,
  type ContentWidth,
  type TeacherCrumb,
} from "@/layouts/shell/handle";
import { Sidebar, SidebarDrawer } from "@/layouts/shell/Sidebar";
import { useSidebarState, writeSidebarState } from "@/layouts/shell/sidebarState";
import { TopBar } from "@/layouts/shell/TopBar";
import { useNavCounts } from "@/layouts/shell/useNavCounts";
import { activeNavId, navFor, reaches } from "@/layouts/teacherNav";
import { TeacherSkeleton } from "@/layouts/TeacherSkeleton";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth";

registerSkeleton("teacher", () => <TeacherSkeleton />);

const NO_CRUMB: readonly TeacherCrumb[] = [];

const MAX_WIDTH: Record<ContentWidth, string> = {
  1320: "max-w-330",
  1080: "max-w-270",
  860: "max-w-215",
  720: "max-w-180",
  full: "",
};

interface Visit {
  route: string | undefined;
  collapsed: boolean;
}

type Drawer = { at: string } | "left" | null;

/**
 * TeacherLayout is the teacher console's shell, as the design deck draws it:
 * a sidebar, a top bar and the page. It reads the leaf route's handle for
 * the breadcrumb trail, the content's maximum width and a collapsed start,
 * and sets the document title from the last crumb, restoring the earlier
 * title when it unmounts. From 768px the sidebar is in the page's flow,
 * collapsed or expanded as the user last chose; a route that asks for a
 * collapsed start gets one for the visit and leaves that choice alone. Below
 * 768px the sidebar is a drawer that closes on every navigation.
 * `<main>` is the scroller and the element pages measure their width by. It
 * sets the page's padding and maximum width, returns to the top and takes
 * focus when the route changes, and does neither when only a parameter
 * does. One outlet serves every width, so a page keeps its state across 768.
 */
export default function TeacherLayout() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const { key, pathname } = useLocation();
  const leaf = useMatches().at(-1);
  const route = leaf?.id;
  const handle = teacherHandleOf(leaf?.handle);
  const wide = useMediaQuery("(min-width: 768px)");
  const stored = useSidebarState();
  const counts = useNavCounts();
  const palette = useCommandPalette();
  const toggleButton = useRef<HTMLButtonElement>(null);
  const main = useRef<HTMLElement | null>(null);
  const [registered, setRegistered] = useState(false);
  const [visit, setVisit] = useState<Visit | null>(null);
  const [drawer, setDrawer] = useState<Drawer>(null);
  const [tail, setTail] = useState<readonly PageCrumb[] | null>(null);

  const openedAt = drawer === null || drawer === "left" ? null : drawer.at;

  if (visit !== null && visit.route !== route) setVisit(null);
  if (drawer !== null && wide) setDrawer(null);
  else if (openedAt !== null && openedAt !== key) setDrawer("left");

  const transient = handle?.sidebar === "collapsed";
  const onVisit = visit === null ? true : visit.collapsed;
  const collapsed = transient ? onVisit : stored === "collapsed";
  const drawerOpen = openedAt !== null;

  const groups = useMemo(() => navFor(user), [user]);
  const activeId = activeNavId(pathname);
  const crumb = handle?.crumb ?? NO_CRUMB;
  const trail = useMemo(() => trailOf(crumb, tail, t), [crumb, tail, t]);
  const page = trail.at(-1)?.label;

  const registerMain = useCallback((element: HTMLElement | null) => {
    main.current = element;
    registerContentElement(element);
    setRegistered(element !== null);
    return () => {
      main.current = null;
      registerContentElement(null);
    };
  }, []);

  useLayoutEffect(() => {
    const element = main.current;
    if (element === null) return;
    element.scrollTop = 0;
    if (!element.contains(document.activeElement))
      element.focus({ preventScroll: true });
  }, [route]);

  useEffect(() => {
    const before = document.title;
    return () => {
      document.title = before;
    };
  }, []);

  useEffect(() => {
    document.title =
      page === undefined ? t("app.name") : t("teacherShell.documentTitle", { page });
  }, [page, t]);

  const toggle = () => {
    if (!wide) {
      setDrawer(drawerOpen ? null : { at: key });
      return;
    }
    if (transient) {
      setVisit({ route, collapsed: !collapsed });
      return;
    }
    writeSidebarState(collapsed ? "expanded" : "collapsed");
  };

  const focusAfterDrawer = (event: Event) => {
    event.preventDefault();
    if (drawer !== "left") {
      toggleButton.current?.focus();
      return;
    }
    const element = main.current;
    if (element !== null && !element.contains(document.activeElement))
      element.focus({ preventScroll: true });
  };

  return (
    <>
      <DeckScale className="bg-bg text-fg flex h-svh overflow-hidden text-base">
        {wide && (
          <Sidebar
            collapsed={collapsed}
            groups={groups}
            activeId={activeId}
            counts={counts}
          />
        )}
        <SidebarDrawer
          open={drawerOpen}
          onOpenChange={(open) => setDrawer(open ? { at: key } : null)}
          onCloseAutoFocus={focusAfterDrawer}
          groups={groups}
          activeId={activeId}
          counts={counts}
        />
        <div className="flex h-full min-w-0 flex-1 flex-col">
          <TopBar
            drawer={!wide}
            expanded={wide ? !collapsed : drawerOpen}
            onToggle={toggle}
            toggleRef={toggleButton}
            trail={trail}
            onSearch={() => palette.setOpen(true)}
            bell={
              modules.notifications ? (
                <NotificationBell
                  audience="teacher"
                  unread={counts?.unread ?? 0}
                  canGrade={reaches(user, "grading")}
                />
              ) : null
            }
          />
          <main
            ref={registerMain}
            tabIndex={-1}
            className="min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-3.5 pt-4 pb-8 outline-none! min-[768px]:px-7 min-[768px]:pt-6 min-[768px]:pb-10"
          >
            {registered && (
              <div
                data-slot="page"
                className={cn("mx-auto w-full", MAX_WIDTH[handle?.width ?? 1320])}
              >
                <CrumbTailContext value={setTail}>
                  <Outlet />
                </CrumbTailContext>
              </div>
            )}
          </main>
        </div>
      </DeckScale>
      <CommandPalette open={palette.open} onOpenChange={palette.setOpen} />
    </>
  );
}
