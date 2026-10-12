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
  "hover:bg-hover data-[state=open]:bg-hover grid size-7.5 flex-none cursor-pointer place-items-center rounded-[0.4375rem]";

const FOOTER_BUTTON =
  "bg-card hover:bg-muted rounded-seg h-7 flex-none border px-2.5 text-xs font-medium whitespace-nowrap";

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
 * without it the card has no menu.
 */
export function ClassCard({
  klass,
  canWrite,
  now,
  ...actions
}: Readonly<
  { klass: ClassListItem; canWrite: boolean; now: number } & ClassCardActions
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
          {canWrite ? <CardMenu klass={klass} {...actions} /> : null}
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
          <CodeFooter klass={klass} canWrite={canWrite} now={now} {...actions} />
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

function CodeFooter({
  klass,
  canWrite,
  now,
  onCopyLink,
  onShowQr,
  onNewCode,
}: Readonly<
  { klass: ClassListItem; canWrite: boolean; now: number } & ClassCardActions
>) {
  const { t } = useTranslation();
  const code = klass.joinCode;
  if (code === null || code === undefined)
    return (
      <>
        <span className="text-muted-fg text-xs">{t("classes.noCode")}</span>
        {canWrite ? (
          <button
            type="button"
            className={cn(FOOTER_BUTTON, "ml-auto")}
            aria-label={t("classes.createCodeNamed", { name: klass.name })}
            onClick={(event) => onNewCode(klass, event.currentTarget)}
          >
            {t("classes.createCode")}
          </button>
        ) : null}
      </>
    );
  const state = codeState(code, now);
  const readable = code.code;
  return (
    <>
      <span
        title={t("classes.joinCode")}
        className="text-muted-fg min-w-0 truncate text-xs whitespace-nowrap"
      >
        {t("classes.joinCode")}
      </span>
      <span className="flex-none font-mono text-[0.84375rem] font-semibold tracking-[.08em] whitespace-nowrap">
        {readable ?? t("classes.maskedCode", { hint: code.hint })}
      </span>
      {state === null ? null : (
        <span
          title={t(`classes.codeState.${state}`)}
          className="text-warning-ink flex-none text-xs whitespace-nowrap"
        >
          {t(`classes.codeState.${state}`)}
        </span>
      )}
      <span className="ml-auto flex flex-none gap-0.5">
        {readable === null && canWrite ? (
          <button
            type="button"
            className={FOOTER_BUTTON}
            aria-label={t("classes.newCodeNamed", { name: klass.name })}
            onClick={(event) => onNewCode(klass, event.currentTarget)}
          >
            {t("classes.newCodeShort")}
          </button>
        ) : null}
        {readable === null ? null : (
          <>
            <button
              type="button"
              className={ICON_BUTTON}
              aria-label={t("classes.copyLinkNamed", { name: klass.name })}
              title={t("classes.copyLink")}
              onClick={() => onCopyLink(readable)}
            >
              <Copy aria-hidden="true" className="size-3.75" />
            </button>
            <button
              type="button"
              className={ICON_BUTTON}
              aria-label={t("classes.showQrNamed", { name: klass.name })}
              title={t("classes.showQr")}
              onClick={(event) => onShowQr(klass, readable, event.currentTarget)}
            >
              <QrCode aria-hidden="true" className="size-3.75" />
            </button>
          </>
        )}
      </span>
    </>
  );
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
  onCopyLink,
  onShowQr,
  onEdit,
  onNewCode,
  onArchive,
  onRestore,
  onDelete,
}: Readonly<{ klass: ClassListItem } & ClassCardActions>) {
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
      <DropdownMenuContent align="end" className="w-52 data-[scale=deck]:w-55">
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
                <DropdownMenuItem onSelect={() => onShowQr(klass, code)}>
                  <QrCode aria-hidden="true" />
                  {t("classes.showQr")}
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => onCopyLink(code)}>
                  <Link2 aria-hidden="true" />
                  {t("classes.copyLink")}
                </DropdownMenuItem>
              </>
            )}
            <DropdownMenuItem onSelect={() => onNewCode(klass)}>
              <RefreshCw aria-hidden="true" />
              {klass.joinCode ? t("classes.newCode") : t("classes.createCode")}
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
