import { useMemo, type ReactNode } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import {
  Archive,
  ArchiveRestore,
  Copy,
  Pencil,
  RefreshCw,
  UserRoundCog,
} from "lucide-react";
import { DataTable, type DataColumn } from "@/components/shared/data/DataTable";
import { Pager } from "@/components/shared/data/Pager";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { ADMIN_CLASS_PAGE_SIZES, type AdminClass } from "@/features/classes/adminApi";
import type { Locale } from "@/lib/i18n";
import { monthYear } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";

/**
 * AdminClassActions are what a row's controls do to its class. `from` is the
 * control the action started from, which a dialog gives focus back to.
 */
export type AdminClassActions = Readonly<{
  onEdit: (klass: AdminClass, from: HTMLElement | null) => void;
  onChangeTeacher: (klass: AdminClass, from: HTMLElement | null) => void;
  onRotate: (klass: AdminClass, from: HTMLElement | null) => void;
  onArchive: (klass: AdminClass, from: HTMLElement | null) => void;
  onRestore: (klass: AdminClass) => void;
  onCopyCode: (code: string) => void;
}>;

type Row = AdminClass & { readonly tab: "active" | "archived" };

const INLINE_BUTTON =
  "text-muted-fg hover:bg-hover hover:text-foreground grid size-6.5 flex-none cursor-pointer place-items-center rounded-md aria-disabled:cursor-not-allowed aria-disabled:text-muted-fg/50 aria-disabled:hover:bg-transparent";

function scheduleOf(klass: AdminClass, t: TFunction): string {
  return klass.scheduleLabel ?? t("classes.noSchedule");
}

function teacherText(klass: AdminClass, t: TFunction): string {
  return klass.teacher.disabled
    ? t("classes.admin.teacherDisabledInline", { name: klass.teacher.name })
    : klass.teacher.name;
}

function codeState(code: NonNullable<AdminClass["joinCode"]>, now: number) {
  if (Date.parse(code.expiresAt) <= now) return "expired";
  if (code.maxUses !== null && code.usesCount >= code.maxUses) return "usedUp";
  return null;
}

function InlineAction({
  named,
  title,
  paused,
  onAct,
  children,
}: Readonly<{
  named: string;
  title: string;
  paused: boolean;
  onAct: (button: HTMLButtonElement) => void;
  children: ReactNode;
}>) {
  return (
    <button
      type="button"
      className={INLINE_BUTTON}
      aria-label={named}
      aria-disabled={paused || undefined}
      title={title}
      onClick={(event) => {
        if (!paused) onAct(event.currentTarget);
      }}
    >
      {children}
    </button>
  );
}

function CodeText({
  code,
  state,
}: Readonly<{
  code: AdminClass["joinCode"];
  state: "expired" | "usedUp" | null;
}>) {
  const { t } = useTranslation();
  if (code === null)
    return (
      <span className="text-muted-fg text-[13px]">{t("classes.admin.noCode")}</span>
    );
  const look = "font-mono text-[13px] font-semibold tracking-[.06em] whitespace-nowrap";
  if (state === null) return <span className={look}>{code.code}</span>;
  return (
    <span
      title={t(`classes.codeState.${state}`)}
      className={cn(look, "text-muted-fg line-through")}
    >
      {code.code}
      <span className="sr-only"> {t(`classes.codeState.${state}`)}</span>
    </span>
  );
}

function CodeCell({
  klass,
  joinOff,
  now,
  actions,
}: Readonly<{
  klass: Row;
  joinOff: boolean;
  now: number;
  actions: AdminClassActions;
}>) {
  const { t } = useTranslation();
  const code = klass.joinCode;
  if (klass.tab === "archived")
    return (
      <span className="text-muted-fg text-[13px]">
        <span aria-hidden="true">—</span>
        <span className="sr-only">{t("classes.admin.noCodeArchived")}</span>
      </span>
    );
  const why = t("classes.joinCodesOff");
  const heldBack = (named: string) =>
    joinOff ? t("classes.heldBackNamed", { named, why }) : named;
  const state = code === null ? null : codeState(code, now);
  return (
    <span className="flex min-w-0 items-center gap-1">
      <CodeText code={code} state={state} />
      {code === null ? null : (
        <InlineAction
          named={heldBack(t("classes.admin.copyCodeNamed", { name: klass.name }))}
          title={joinOff ? why : t("classes.admin.copyCode")}
          paused={joinOff}
          onAct={() => actions.onCopyCode(code.code)}
        >
          <Copy aria-hidden="true" className="size-3.25" />
        </InlineAction>
      )}
      <InlineAction
        named={heldBack(
          t(code === null ? "classes.createCodeNamed" : "classes.admin.rotateNamed", {
            name: klass.name,
          }),
        )}
        title={joinOff ? why : t("classes.admin.rotate")}
        paused={joinOff}
        onAct={(button) => actions.onRotate(klass, button)}
      >
        <RefreshCw aria-hidden="true" className="size-3.25" />
      </InlineAction>
    </span>
  );
}

function columns(
  t: TFunction,
  locale: Locale,
  joinOff: boolean,
  now: number,
  actions: AdminClassActions,
): DataColumn<Row>[] {
  return [
    {
      id: "class",
      header: t("classes.admin.column.class"),
      track: "minmax(180px,2fr)",
      cell: (klass, shown) => (
        <span className="block min-w-0">
          <span className="block truncate font-medium">{klass.name}</span>
          <span className="text-muted-fg block truncate text-xs">
            {shown.has("teacher")
              ? scheduleOf(klass, t)
              : `${teacherText(klass, t)} · ${scheduleOf(klass, t)}`}
          </span>
        </span>
      ),
    },
    {
      id: "teacher",
      header: t("classes.admin.column.teacher"),
      track: "minmax(150px,1.4fr)",
      showFrom: 600,
      cell: (klass) => (
        <span className="flex min-w-0 items-center gap-2">
          <Avatar
            name={klass.teacher.name}
            src={klass.teacher.avatarUrl}
            size="26"
            className="text-[10.5px]"
          />
          <span className="min-w-0 truncate">{klass.teacher.name}</span>
          {klass.teacher.disabled ? (
            <Badge variant="secondary" className="bg-muted text-muted-fg flex-none">
              {t("classes.admin.teacherDisabled")}
            </Badge>
          ) : null}
        </span>
      ),
    },
    {
      id: "students",
      header: t("classes.admin.column.students"),
      track: "80px",
      align: "end",
      cell: (klass) => (
        <span className="tabular-nums">
          {new Intl.NumberFormat(locale).format(klass.studentCount)}
        </span>
      ),
    },
    {
      id: "code",
      header: t("classes.admin.column.code"),
      track: "160px",
      showFrom: 760,
      cell: (klass) => (
        <CodeCell klass={klass} joinOff={joinOff} now={now} actions={actions} />
      ),
    },
    {
      id: "created",
      header: t("classes.admin.column.created"),
      track: "100px",
      showFrom: 900,
      cell: (klass) => (
        <span className="text-muted-fg text-[13px] whitespace-nowrap">
          {monthYear(klass.createdAt, locale)}
        </span>
      ),
    },
  ];
}

/**
 * AdminClassesTable is a page of every teacher's classes as the deck draws
 * them, and under it the pager. Class and Students always show; Teacher joins
 * from a content width of 600, the join code from 760 and Created from 900,
 * and a folded teacher joins the class's sub-line. Rows are not links. A
 * code reads in full with Copy and Rotate beside it (Create on a class
 * without one). An archived class takes no student, so its code is a dash,
 * with nothing to copy or rotate, and the menu offers no rotation there. While `joinOff` says the centre has
 * turned joining by code off, Copy, Rotate and Create stay but are paused with
 * the centre's sentence as their reason (D-028). `empty` is what an empty
 * page says.
 */
export function AdminClassesTable({
  classes,
  tab,
  page,
  pageSize,
  total,
  joinOff,
  now,
  locale,
  empty,
  actions,
}: Readonly<{
  classes: readonly AdminClass[];
  tab: "active" | "archived";
  page: number;
  pageSize: number;
  total: number;
  joinOff: boolean;
  now: number;
  locale: Locale;
  empty: ReactNode;
  actions: AdminClassActions;
}>) {
  const { t } = useTranslation();
  const rows = useMemo(
    () => classes.map((klass): Row => ({ ...klass, tab })),
    [classes, tab],
  );
  const table = useMemo(
    () => columns(t, locale, joinOff, now, actions),
    [t, locale, joinOff, now, actions],
  );
  const why = t("classes.joinCodesOff");
  return (
    <DataTable
      label={t("classes.admin.table")}
      columns={table}
      rows={rows}
      rowSize={{ height: 54 }}
      menuTrack="40px"
      menuLabel={(klass) => t("classes.actionsFor", { name: klass.name })}
      empty={empty}
      menu={(klass, { triggerRef }) => (
        <>
          <DropdownMenuItem onSelect={() => actions.onEdit(klass, triggerRef.current)}>
            <Pencil aria-hidden="true" />
            {t("classes.edit")}
          </DropdownMenuItem>
          <DropdownMenuItem
            onSelect={() => actions.onChangeTeacher(klass, triggerRef.current)}
          >
            <UserRoundCog aria-hidden="true" />
            {t("classes.admin.changeTeacher")}
          </DropdownMenuItem>
          {tab === "archived" ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => actions.onRestore(klass)}>
                <ArchiveRestore aria-hidden="true" />
                {t("classes.admin.restore")}
              </DropdownMenuItem>
            </>
          ) : (
            <>
              <DropdownMenuItem
                aria-disabled={joinOff || undefined}
                title={joinOff ? why : undefined}
                className="aria-disabled:text-muted-fg aria-disabled:[&_svg]:text-muted-fg"
                onSelect={(event) => {
                  if (joinOff) event.preventDefault();
                  else actions.onRotate(klass, triggerRef.current);
                }}
              >
                <RefreshCw aria-hidden="true" />
                {t(
                  klass.joinCode === null
                    ? "classes.createCode"
                    : "classes.admin.rotate",
                )}
                {joinOff ? <span className="sr-only">{why}</span> : null}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onSelect={() => actions.onArchive(klass, triggerRef.current)}
              >
                <Archive aria-hidden="true" />
                {t("classes.archiveClass")}
              </DropdownMenuItem>
            </>
          )}
        </>
      )}
      footer={
        <Pager
          page={page}
          pageSize={pageSize}
          total={total}
          sizes={ADMIN_CLASS_PAGE_SIZES}
          noun={(count) => t("classes.noun", { count })}
        />
      }
    />
  );
}
