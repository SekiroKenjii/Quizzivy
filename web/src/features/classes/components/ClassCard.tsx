import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  Archive,
  ArchiveRestore,
  Copy,
  Ellipsis,
  Link2,
  Pencil,
  QrCode,
  RefreshCw,
  Trash2,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ClassListItem } from "@/features/classes/api";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

/**
 * ClassCardActions are what a card's controls do to its class. The menu's
 * trigger carries `data-class-menu` with the class's id, so a dialog opened
 * from the menu can give focus back to it.
 */
export type ClassCardActions = Readonly<{
  onCopyLink: (code: string) => void;
  onShowQr: (klass: ClassListItem, code: string, from?: HTMLElement) => void;
  onEdit: (klass: ClassListItem) => void;
  onNewCode: (klass: ClassListItem, from?: HTMLElement) => void;
  onArchive: (klass: ClassListItem) => void;
  onRestore: (klass: ClassListItem) => void;
  onDelete: (klass: ClassListItem) => void;
}>;

const ICON_BUTTON =
  "hover:bg-hover data-[state=open]:bg-hover grid size-7.5 flex-none cursor-pointer place-items-center rounded-[0.4375rem] aria-disabled:text-foreground/50 aria-disabled:cursor-not-allowed aria-disabled:hover:bg-transparent";

const FOOTER_BUTTON =
  "bg-card hover:bg-muted rounded-seg h-7 flex-none border px-2.5 text-xs font-medium whitespace-nowrap aria-disabled:text-foreground/50 aria-disabled:border-border/50 aria-disabled:cursor-not-allowed aria-disabled:hover:bg-card";

/**
 * ClassCard is one class of the Classes list, as the deck draws it: the
 * name, which opens the class, over "{schedule} · {room}"; its students,
 * live tests and average; and a footer with the join code in full, mono,
 * with Copy join link and Show QR code. A code that cannot be read shows its
 * last four characters and, for a teacher who may change the class, New
 * code; a class without one offers Create code. The footer stays one row:
 * an Expired or Used up note shrinks and truncates, with its text as its
 * title, before the buttons would wrap. An archived class's footer offers
 * Restore. `canWrite` says whether the reader may change the class;
 * without it the card has no menu. While `joinOff` says the centre has turned
 * joining by code off (D-023, D-028), Create code, New code, Copy join link
 * and Show QR code, in the footer and the menu, stay but are disabled with
 * that reason, and a code is marked inactive, naming why it also is Expired
 * or Used up.
 */
export function ClassCard({
  klass,
  canWrite,
  now,
  joinOff = false,
  ...actions
}: Readonly<
  {
    klass: ClassListItem;
    canWrite: boolean;
    now: number;
    joinOff?: boolean;
  } & ClassCardActions
>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const archived = klass.archivedAt !== null;
  const where = [klass.scheduleLabel, klass.room].filter(Boolean).join(" · ");
  const percent =
    klass.averageScore && klass.averageScore.total > 0
      ? Math.round((klass.averageScore.earned / klass.averageScore.total) * 100)
      : null;
  const figure = new Intl.NumberFormat(locale);
  return (
    <article
      data-class-card={klass.id}
      className="bg-card shadow-card flex min-w-0 flex-col overflow-hidden rounded-xl border"
    >
      <div className="flex flex-1 flex-col gap-3.5 p-4">
        <div className="flex justify-between gap-2.5">
          <div className="min-w-0">
            <h2 className="text-md leading-tight font-semibold [overflow-wrap:anywhere]">
              <Link to={`/teacher/classes/${klass.id}`} className="hover:underline">
                {klass.name}
              </Link>
            </h2>
            <p className="text-muted-fg text-meta leading-4 [overflow-wrap:anywhere]">
              {where === "" ? t("classes.noSchedule") : where}
            </p>
          </div>
          {canWrite ? <CardMenu klass={klass} joinOff={joinOff} {...actions} /> : null}
        </div>
        <dl className="m-0 grid grid-cols-3 gap-2">
          <Stat label={t("classes.stats.students")}>
            {figure.format(klass.studentCount)}
          </Stat>
          <Stat label={t("classes.stats.live")}>
            {figure.format(klass.openAssignmentCount)}
          </Stat>
          <Stat label={t("classes.stats.average")}>
            {percent === null
              ? t("classes.noAverage")
              : t("classes.percent", { value: percent })}
          </Stat>
        </dl>
      </div>
      <div className="bg-sidebar flex min-h-12.5 items-center gap-2 border-t py-2.5 pr-3 pl-4">
        {archived ? (
          <ArchivedFooter
            klass={klass}
            canWrite={canWrite}
            onRestore={actions.onRestore}
          />
        ) : (
          <CodeFooter
            klass={klass}
            canWrite={canWrite}
            now={now}
            joinOff={joinOff}
            {...actions}
          />
        )}
      </div>
    </article>
  );
}

function Stat({ label, children }: Readonly<{ label: string; children: string }>) {
  return (
    <div className="min-w-0">
      <dt className="text-muted-fg text-xs leading-normal">{label}</dt>
      <dd className="text-stat-sm m-0 leading-normal font-semibold tabular-nums">
        {children}
      </dd>
    </div>
  );
}

function ArchivedFooter({
  klass,
  canWrite,
  onRestore,
}: Readonly<{
  klass: ClassListItem;
  canWrite: boolean;
  onRestore: (klass: ClassListItem) => void;
}>) {
  const { t } = useTranslation();
  return (
    <>
      <span className="text-muted-fg text-xs">{t("classes.archivedBadge")}</span>
      {canWrite ? (
        <button
          type="button"
          className={cn(FOOTER_BUTTON, "ml-auto")}
          aria-label={t("classes.restoreNamed", { name: klass.name })}
          onClick={() => onRestore(klass)}
        >
          {t("classes.restore")}
        </button>
      ) : null}
    </>
  );
}

function IconAction({
  named,
  title,
  joinOff,
  onAct,
  children,
}: Readonly<{
  named: string;
  title: string;
  joinOff: boolean;
  onAct: (button: HTMLButtonElement) => void;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      className={ICON_BUTTON}
      aria-label={named}
      aria-disabled={joinOff || undefined}
      title={joinOff ? t("classes.joinCodesOff") : title}
      onClick={(event) => {
        if (!joinOff) onAct(event.currentTarget);
      }}
    >
      {children}
    </button>
  );
}

function CodeFooter({
  klass,
  canWrite,
  now,
  joinOff,
  onCopyLink,
  onShowQr,
  onNewCode,
}: Readonly<
  {
    klass: ClassListItem;
    canWrite: boolean;
    now: number;
    joinOff: boolean;
  } & ClassCardActions
>) {
  const { t } = useTranslation();
  const code = klass.joinCode;
  const heldBack = (named: string) =>
    joinOff
      ? t("classes.heldBackNamed", { named, why: t("classes.joinCodesOff") })
      : named;
  const newCode = (named: string, label: string, from: ClassListItem) => (
    <button
      type="button"
      className={FOOTER_BUTTON}
      aria-label={heldBack(named)}
      aria-disabled={joinOff || undefined}
      onClick={(event) => {
        if (!joinOff) onNewCode(from, event.currentTarget);
      }}
    >
      {label}
    </button>
  );
  if (code === null || code === undefined)
    return (
      <>
        <span className="text-muted-fg text-xs">{t("classes.noCode")}</span>
        {canWrite ? (
          <span className="ml-auto flex flex-none">
            {newCode(
              t("classes.createCodeNamed", { name: klass.name }),
              t("classes.createCode"),
              klass,
            )}
          </span>
        ) : null}
      </>
    );
  const reason = codeState(code, now);
  const state = joinOff ? "off" : reason;
  const both = joinOff && reason !== null;
  let stateTitle = "";
  if (both)
    stateTitle = t("classes.codeStateBoth", {
      state: t("classes.codeState.off"),
      why: t(`classes.codeState.${reason}`),
    });
  else if (state !== null) stateTitle = t(`classes.codeState.${state}`);
  const readable = code.code;
  return (
    <>
      <span
        title={t("classes.joinCode")}
        className={cn(
          "text-muted-fg min-w-0 truncate text-xs whitespace-nowrap",
          joinOff && "sr-only",
        )}
      >
        {t("classes.joinCode")}
      </span>
      <span className="flex-none font-mono text-[0.84375rem] font-semibold tracking-[.08em] whitespace-nowrap">
        {readable ?? t("classes.maskedCode", { hint: code.hint })}
      </span>
      {state === null ? null : (
        <span
          title={stateTitle}
          className="text-warning-ink flex-none text-xs whitespace-nowrap"
        >
          {t(`classes.codeState.${state}`)}
          {both ? (
            <span className="sr-only">{t(`classes.codeState.${reason}`)}</span>
          ) : null}
        </span>
      )}
      <span className="ml-auto flex flex-none gap-0.5">
        {readable === null && canWrite
          ? newCode(
              t("classes.newCodeNamed", { name: klass.name }),
              t("classes.newCodeShort"),
              klass,
            )
          : null}
        {readable === null ? null : (
          <>
            <IconAction
              named={heldBack(t("classes.copyLinkNamed", { name: klass.name }))}
              title={t("classes.copyLink")}
              joinOff={joinOff}
              onAct={() => onCopyLink(readable)}
            >
              <Copy aria-hidden="true" className="size-3.75" />
            </IconAction>
            <IconAction
              named={heldBack(t("classes.showQrNamed", { name: klass.name }))}
              title={t("classes.showQr")}
              joinOff={joinOff}
              onAct={(button) => onShowQr(klass, readable, button)}
            >
              <QrCode aria-hidden="true" className="size-3.75" />
            </IconAction>
          </>
        )}
      </span>
    </>
  );
}

function heldBackItem(joinOff: boolean, why: string) {
  return {
    "aria-disabled": joinOff || undefined,
    title: joinOff ? why : undefined,
    className: "aria-disabled:text-foreground/50",
  } as const;
}

function HeldBackReason() {
  const { t } = useTranslation();
  return <span className="sr-only">{t("classes.joinCodesOff")}</span>;
}

function codeState(
  code: NonNullable<ClassListItem["joinCode"]>,
  now: number,
): "expired" | "usedUp" | null {
  if (Date.parse(code.expiresAt) <= now) return "expired";
  if (code.maxUses !== null && code.usesCount >= code.maxUses) return "usedUp";
  return null;
}

function CardMenu({
  klass,
  joinOff,
  onCopyLink,
  onShowQr,
  onEdit,
  onNewCode,
  onArchive,
  onRestore,
  onDelete,
}: Readonly<{ klass: ClassListItem; joinOff: boolean } & ClassCardActions>) {
  const { t } = useTranslation();
  const archived = klass.archivedAt !== null;
  const code = klass.joinCode?.code ?? null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          data-class-menu={klass.id}
          aria-label={t("classes.actionsFor", { name: klass.name })}
          className={ICON_BUTTON}
        >
          <Ellipsis aria-hidden="true" className="size-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-55">
        {archived ? (
          <>
            <DropdownMenuItem onSelect={() => onRestore(klass)}>
              <ArchiveRestore aria-hidden="true" />
              {t("classes.restore")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => onDelete(klass)}>
              <Trash2 aria-hidden="true" />
              {t("classes.delete")}
            </DropdownMenuItem>
          </>
        ) : (
          <>
            <DropdownMenuItem onSelect={() => onEdit(klass)}>
              <Pencil aria-hidden="true" />
              {t("classes.edit")}
            </DropdownMenuItem>
            {code === null ? null : (
              <>
                <DropdownMenuItem
                  {...heldBackItem(joinOff, t("classes.joinCodesOff"))}
                  onSelect={(event) => {
                    if (joinOff) event.preventDefault();
                    else onShowQr(klass, code);
                  }}
                >
                  <QrCode aria-hidden="true" />
                  {t("classes.showQr")}
                  {joinOff ? <HeldBackReason /> : null}
                </DropdownMenuItem>
                <DropdownMenuItem
                  {...heldBackItem(joinOff, t("classes.joinCodesOff"))}
                  onSelect={(event) => {
                    if (joinOff) event.preventDefault();
                    else onCopyLink(code);
                  }}
                >
                  <Link2 aria-hidden="true" />
                  {t("classes.copyLink")}
                  {joinOff ? <HeldBackReason /> : null}
                </DropdownMenuItem>
              </>
            )}
            <DropdownMenuItem
              {...heldBackItem(joinOff, t("classes.joinCodesOff"))}
              onSelect={(event) => {
                if (joinOff) event.preventDefault();
                else onNewCode(klass);
              }}
            >
              <RefreshCw aria-hidden="true" />
              {klass.joinCode ? t("classes.newCode") : t("classes.createCode")}
              {joinOff ? <HeldBackReason /> : null}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => onArchive(klass)}>
              <Archive aria-hidden="true" />
              {t("classes.archiveClass")}
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
