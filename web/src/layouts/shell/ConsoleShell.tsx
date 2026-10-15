import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Outlet, useLocation, useMatches } from "react-router";
import { useTranslation } from "react-i18next";
import { DeckScale } from "@/components/ui/deck-scale";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { registerContentElement } from "@/layouts/shell/contentWidth";
import { CrumbTailContext, trailOf, type PageCrumb } from "@/layouts/shell/crumbs";
import { teacherHandleOf, type TeacherCrumb } from "@/layouts/shell/handle";
import type { NavFigures, NavGroup, StaffWorkspace } from "@/layouts/shell/nav";
import { Sidebar, SidebarDrawer } from "@/layouts/shell/Sidebar";
import { useSidebarState, writeSidebarState } from "@/layouts/shell/sidebarState";
import { TopBar } from "@/layouts/shell/TopBar";
import { cn } from "@/lib/utils";

const NO_CRUMB: readonly TeacherCrumb[] = [];

interface Visit {
  route: string | undefined;
  collapsed: boolean;
}

type Drawer = { at: string } | "left" | null;

/**
 * ConsoleShell is the frame both consoles share, as the design deck draws it:
 * a sidebar, a top bar and the page. It reads the leaf route's handle for
 * the breadcrumb trail, the page's width and a collapsed start, and sets the
 * document title from the last crumb, restoring the earlier title when it
 * unmounts. From 768px the sidebar is in the page's flow, collapsed or
 * expanded as the user last chose; a route that asks for a collapsed start
 * gets one for the visit and leaves that choice alone. Below 768px the
 * sidebar is a drawer that closes on every navigation. `<main>` is the
 * scroller, the containing block of everything positioned in the page, so
 * nothing in it extends the document, and the element pages measure their
 * width by. It sets the page's gutters (`--page-gutter`) and the one page
 * frame every route renders in: the full width up to `--page-max`, centred
 * beyond it, to the maximum the handle's `width` gives in pixels in the Admin
 * console, which the teacher's console ignores, or the full width when it
 * says `"full"` (DG-205). It returns to the top and takes
 * focus when the route changes, and does neither when only a parameter does.
 * One outlet serves every width, so a page keeps its state across 768.
 * `onSearch` adds the search button to the top bar, and `end` follows its
 * theme switch.
 */
export function ConsoleShell({
  workspace,
  groups,
  activeId,
  counts,
  onSearch,
  end,
}: Readonly<{
  workspace: StaffWorkspace;
  groups: readonly NavGroup[];
  activeId: string | null;
  counts: NavFigures | null;
  onSearch?: () => void;
  end?: ReactNode;
}>) {
  const { t } = useTranslation();
  const { key } = useLocation();
  const leaf = useMatches().at(-1);
  const route = leaf?.id;
  const handle = teacherHandleOf(leaf?.handle);
  const wide = useMediaQuery("(min-width: 768px)");
  const stored = useSidebarState();
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

  const crumb = handle?.crumb ?? NO_CRUMB;
  const trail = useMemo(() => trailOf(crumb, tail, t), [crumb, tail, t]);
  const page = trail.at(-1)?.label;
  const declared = handle?.width;
  const width =
    declared === "full" || (workspace === "admin" && typeof declared === "number")
      ? declared
      : undefined;

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
    <DeckScale className="bg-bg text-fg flex h-svh overflow-hidden text-base">
      {wide && (
        <Sidebar
          workspace={workspace}
          collapsed={collapsed}
          groups={groups}
          activeId={activeId}
          counts={counts}
        />
      )}
      <SidebarDrawer
        workspace={workspace}
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
          {...(onSearch ? { onSearch } : {})}
          end={end}
        />
        <main
          ref={registerMain}
          tabIndex={-1}
          className="relative min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-(--page-gutter) pt-(--page-pad-top) pb-8 outline-none! min-[768px]:pb-10"
        >
          {registered && (
            <div
              data-slot="page"
              className={cn(
                "mx-auto w-full",
                width === undefined && "max-w-(--page-max)",
              )}
              style={typeof width === "number" ? { maxWidth: width } : undefined}
            >
              <CrumbTailContext value={setTail}>
                <Outlet />
              </CrumbTailContext>
            </div>
          )}
        </main>
      </div>
    </DeckScale>
  );
}
