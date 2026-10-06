import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Dialog } from "radix-ui";
import { AccountMenu } from "@/features/auth/AccountMenu";
import type { NavCounts } from "@/layouts/shell/useNavCounts";
import {
  WorkspaceSwitcher,
  type WorkspaceDestination,
} from "@/layouts/shell/WorkspaceSwitcher";
import type { NavCount, TeacherNavGroup, TeacherNavItem } from "@/layouts/teacherNav";
import { cn } from "@/lib/utils";

/** SIDEBAR_ID is the id of the teacher sidebar, which its toggle controls. */
export const SIDEBAR_ID = "teacher-sidebar";

const DESTINATIONS: readonly WorkspaceDestination[] = [];

const COUNT_SENTENCE = {
  liveAssignments: "teacherShell.count.live",
  toGrade: "teacherShell.count.toGrade",
} as const;

interface SidebarContent {
  groups: readonly TeacherNavGroup[];
  activeId: string | null;
  counts: NavCounts | null;
}

function CountBadge({
  count,
  value,
  collapsed,
}: Readonly<{ count: NavCount; value: number; collapsed: boolean }>) {
  const { t } = useTranslation();
  const sentence = (
    <span className="sr-only">{t(COUNT_SENTENCE[count.figure], { count: value })}</span>
  );
  if (collapsed) return sentence;
  return (
    <span
      data-slot="nav-count"
      className={cn(
        "text-caption grid h-5 min-w-5 flex-none place-items-center rounded-full px-1.5 leading-none font-semibold tabular-nums",
        count.tone === "urgent" ? "bg-brand text-brand-fg" : "bg-muted text-muted-fg",
      )}
    >
      <span aria-hidden="true">{value}</span>
      {sentence}
    </span>
  );
}

function NavItem({
  item,
  active,
  collapsed,
  counts,
}: Readonly<{
  item: TeacherNavItem;
  active: boolean;
  collapsed: boolean;
  counts: NavCounts | null;
}>) {
  const { t } = useTranslation();
  const label = t(item.label);
  const value = item.count && counts ? counts[item.count.figure] : 0;
  return (
    <Link
      to={item.to}
      aria-current={active ? "page" : undefined}
      title={collapsed ? label : undefined}
      className={cn(
        "rounded-seg text-ui hover:bg-hover hover:text-fg flex h-8.5 items-center gap-2.5 px-2.5",
        active ? "bg-hover text-fg font-medium" : "text-muted-fg",
        collapsed && "justify-center",
      )}
    >
      <item.icon className="size-4 flex-none" aria-hidden="true" />
      <span className={collapsed ? "sr-only" : "min-w-0 flex-1 truncate leading-4.25"}>
        {label}
      </span>
      {item.count && value !== null && value > 0 && (
        <CountBadge count={item.count} value={value} collapsed={collapsed} />
      )}
    </Link>
  );
}

function SidebarBody({
  groups,
  activeId,
  counts,
  collapsed,
}: Readonly<SidebarContent & { collapsed: boolean }>) {
  const { t } = useTranslation();
  return (
    <>
      <div className="flex-none px-2 pt-2.5 pb-1.5">
        <WorkspaceSwitcher collapsed={collapsed} destinations={DESTINATIONS} />
      </div>
      <nav
        aria-label={t("nav.mainNavigation")}
        className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-x-hidden overflow-y-auto px-2 py-1"
      >
        {groups.map((group) => (
          <div key={group.id} className="flex flex-col gap-px">
            {group.label !== undefined && !collapsed && (
              <p className="text-caption text-muted-fg px-2.5 py-1 leading-normal font-medium">
                {t(group.label)}
              </p>
            )}
            <ul className="flex flex-col gap-px">
              {group.items.map((item) => (
                <li key={item.id}>
                  <NavItem
                    item={item}
                    active={item.id === activeId}
                    collapsed={collapsed}
                    counts={counts}
                  />
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>
      <div className="flex-none border-t p-2">
        <AccountMenu
          deck
          sidebar={collapsed ? "collapsed" : "expanded"}
          settingsTo="/teacher/settings"
        />
      </div>
    </>
  );
}

/**
 * Sidebar is the teacher console's sidebar from 768px, in the page's flow:
 * the workspace block, the destinations in their groups, and the account
 * button at its foot. Expanded it is 248px wide. Collapsed it is 60px: an
 * icon for each destination, whose label is its `title` and stays its
 * accessible name, and no badge. The destination with `activeId` is the
 * current page.
 */
export function Sidebar({
  collapsed,
  ...content
}: Readonly<SidebarContent & { collapsed: boolean }>) {
  return (
    <div
      id={SIDEBAR_ID}
      data-state={collapsed ? "collapsed" : "expanded"}
      className={cn(
        "bg-sidebar flex flex-none flex-col border-r transition-[width] duration-200 ease-[cubic-bezier(0,0,0.58,1)]",
        collapsed ? "w-15" : "w-62",
      )}
    >
      <SidebarBody {...content} collapsed={collapsed} />
    </div>
  );
}

/**
 * SidebarDrawer is the teacher console's sidebar below 768px: a 272px
 * drawer over an overlay, with labels and badges, that traps focus and
 * closes on Escape and on a press outside it. The caller owns `open`, and
 * decides in `onCloseAutoFocus` where focus goes when it closes.
 */
export function SidebarDrawer({
  open,
  onOpenChange,
  onCloseAutoFocus,
  ...content
}: Readonly<
  SidebarContent & {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onCloseAutoFocus: (event: Event) => void;
  }
>) {
  const { t } = useTranslation();
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="bg-overlay data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0 fixed inset-0 z-(--z-sheet) duration-200" />
        <Dialog.Content
          id={SIDEBAR_ID}
          data-scale="deck"
          aria-describedby={undefined}
          onCloseAutoFocus={onCloseAutoFocus}
          className="bg-sidebar text-fg data-[state=closed]:animate-out data-[state=closed]:slide-out-to-left data-[state=open]:animate-in data-[state=open]:slide-in-from-left fixed inset-y-0 left-0 z-(--z-sheet) flex w-68 max-w-full flex-col border-r text-base duration-200 ease-[cubic-bezier(0,0,0.58,1)] outline-none"
        >
          <Dialog.Title className="sr-only">{t("nav.mainNavigation")}</Dialog.Title>
          <SidebarBody {...content} collapsed={false} />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
