import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Dialog } from "radix-ui";
import { AccountMenu } from "@/features/auth/AccountMenu";
import type {
  NavCount,
  NavDestination,
  NavFigures,
  NavGroup,
  NavTone,
  StaffWorkspace,
} from "@/layouts/shell/nav";
import { WorkspaceSwitcher } from "@/layouts/shell/WorkspaceSwitcher";
import { cn } from "@/lib/utils";

/** SIDEBAR_ID is the id of a console's sidebar, which its toggle controls. */
export const SIDEBAR_ID = "console-sidebar";

const TONE: Record<NavTone, string> = {
  muted: "bg-muted text-muted-fg",
  urgent: "bg-brand text-brand-fg",
  warning: "bg-warning-soft text-warning-ink",
};

interface SidebarContent {
  workspace: StaffWorkspace;
  groups: readonly NavGroup[];
  activeId: string | null;
  counts: NavFigures | null;
}

function CountBadge({
  count,
  value,
  collapsed,
}: Readonly<{ count: NavCount; value: number; collapsed: boolean }>) {
  const { t } = useTranslation();
  const sentence = (
    <span className="sr-only">{t(count.sentence, { count: value })}</span>
  );
  if (collapsed) return sentence;
  return (
    <span
      data-slot="nav-count"
      className={cn(
        "text-caption grid h-5 min-w-5 flex-none place-items-center rounded-full px-1.5 leading-none font-semibold tabular-nums",
        TONE[count.tone],
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
  item: NavDestination;
  active: boolean;
  collapsed: boolean;
  counts: NavFigures | null;
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
      {item.count && value != null && value > 0 && (
        <CountBadge count={item.count} value={value} collapsed={collapsed} />
      )}
    </Link>
  );
}

function SidebarBody({
  workspace,
  groups,
  activeId,
  counts,
  collapsed,
}: Readonly<SidebarContent & { collapsed: boolean }>) {
  const { t } = useTranslation();
  return (
    <>
      <div className="flex-none px-2 pt-2.5 pb-1.5">
        <WorkspaceSwitcher collapsed={collapsed} workspace={workspace} />
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
            {group.label !== undefined && collapsed && (
              <div role="separator" className="bg-border mx-2.5 mb-1 h-px flex-none" />
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
          workspace={workspace}
          sidebar={collapsed ? "collapsed" : "expanded"}
        />
      </div>
    </>
  );
}

/**
 * Sidebar is a console's sidebar from 768px, in the page's flow: the
 * workspace block, the destinations in their groups, and the account button
 * at its foot, all of the `workspace` it belongs to. Expanded it is 248px wide.
 * Collapsed it is 60px: an icon for each destination, whose label is its
 * `title` and stays its accessible name, no badge, and a separator in place
 * of each group's label. The destination with `activeId` is the current page.
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
 * SidebarDrawer is a console's sidebar below 768px: a 272px
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
