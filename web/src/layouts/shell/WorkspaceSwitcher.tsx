import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  ArrowUpRight,
  Check,
  ChevronsUpDown,
  GraduationCap,
  Shield,
  type LucideIcon,
} from "lucide-react";
import { ADMIN_HOME } from "@/app/adminPaths";
import { BrandMark } from "@/components/shared/Brand";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { rememberWorkspace } from "@/features/auth/accountPreferences";
import { useWorkspace } from "@/features/auth/permissions";
import type { StaffWorkspace } from "@/layouts/shell/nav";
import { cn } from "@/lib/utils";

interface Destination {
  workspace: StaffWorkspace;
  label: string;
  to: string;
  icon: LucideIcon;
}

const DESTINATIONS: readonly Destination[] = [
  {
    workspace: "teacher",
    label: "teacherShell.workspace",
    to: "/teacher",
    icon: GraduationCap,
  },
  { workspace: "admin", label: "adminShell.workspace", to: ADMIN_HOME, icon: Shield },
];

const ROW = "flex w-full items-center gap-2.5 rounded-md p-1.5 text-left";
const MENU_ROW =
  "focus:bg-hover focus:text-fg text-sm in-data-[scale=deck]:gap-2.5 in-data-[scale=deck]:p-2 in-data-[scale=deck]:leading-4";

function Tile({
  workspace,
  collapsed,
}: Readonly<{ workspace: StaffWorkspace; collapsed: boolean }>) {
  const { t } = useTranslation();
  if (workspace === "admin") {
    return (
      <span
        role={collapsed ? "img" : undefined}
        aria-label={collapsed ? t("app.name") : undefined}
        aria-hidden={collapsed ? undefined : true}
        className="bg-primary text-primary-fg grid size-8 flex-none place-items-center rounded-md"
      >
        <Shield className="size-4" aria-hidden="true" />
      </span>
    );
  }
  return (
    <span
      aria-hidden={collapsed ? undefined : true}
      className="bg-card grid size-8 flex-none place-items-center rounded-md border"
    >
      <BrandMark height={18} label={false} theme="auto" belowFloor="DG-123" />
    </span>
  );
}

/**
 * WorkspaceSwitcher is the block at the head of a console's sidebar: a tile,
 * the product's name and the name of the console it heads (`workspace`), the
 * teacher's with the brand mark and the Admin's with a shield on the primary
 * colour. For a user who holds only that console's workspace it is a label.
 * For one who holds both it is a button that opens the deck's "Switch
 * workspace" menu, whose row for the other console links to it and remembers
 * the choice (`rememberWorkspace`). `collapsed` leaves the tile alone.
 */
export function WorkspaceSwitcher({
  workspace,
  collapsed,
}: Readonly<{ workspace: StaffWorkspace; collapsed: boolean }>) {
  const { t } = useTranslation();
  const teacher = useWorkspace("teacher");
  const admin = useWorkspace("admin");
  const switches = teacher && admin;
  const current = DESTINATIONS.find((row) => row.workspace === workspace)!;

  const identity = (
    <>
      <Tile workspace={workspace} collapsed={collapsed} />
      {!collapsed && (
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="text-ui truncate leading-tight font-semibold">
            {t("app.name")}
          </span>
          <span className="text-muted-fg truncate text-xs leading-tight">
            {t(current.label)}
          </span>
        </span>
      )}
    </>
  );

  if (!switches) {
    return (
      <div data-slot="workspace" className={cn(ROW, collapsed && "justify-center")}>
        {identity}
      </div>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        data-slot="workspace"
        className={cn(
          ROW,
          "hover:bg-hover data-[state=open]:bg-hover",
          collapsed && "justify-center",
        )}
      >
        {identity}
        {!collapsed && (
          <ChevronsUpDown
            className="text-muted-fg size-[0.9375rem] flex-none"
            aria-hidden="true"
          />
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        sideOffset={0}
        className="data-[scale=deck]:w-60 data-[scale=deck]:p-1.5"
      >
        <DropdownMenuLabel>{t("teacherShell.switchWorkspace")}</DropdownMenuLabel>
        {DESTINATIONS.map((destination) =>
          destination.workspace === workspace ? (
            <DropdownMenuItem
              key={destination.workspace}
              aria-current="true"
              className={cn(MENU_ROW, "bg-muted font-medium")}
            >
              <destination.icon className="size-4" aria-hidden="true" />
              <span className="min-w-0 flex-1 truncate">{t(destination.label)}</span>
              <Check className="size-[0.9375rem]" aria-hidden="true" />
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem key={destination.workspace} asChild className={MENU_ROW}>
              <Link
                to={destination.to}
                onClick={() => rememberWorkspace(destination.workspace)}
              >
                <destination.icon className="size-4" aria-hidden="true" />
                <span className="min-w-0 flex-1 truncate">{t(destination.label)}</span>
                <ArrowUpRight
                  className="text-muted-fg size-3.5 flex-none"
                  aria-hidden="true"
                />
              </Link>
            </DropdownMenuItem>
          ),
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
