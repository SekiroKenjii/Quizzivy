import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronUp } from "lucide-react";
import { DataTable, type DataColumn } from "@/components/shared/data/DataTable";
import { Avatar } from "@/components/ui/avatar";
import { auditMoment, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import { dotOf, humanise, sentenceOf } from "../actions";
import type { AuditEntry } from "../api";
import { AuditDiff } from "./AuditDiff";

type Row = Readonly<{ id: string; entry: AuditEntry }>;

const WHEN_FROM = 700;
const ENTITY_FROM = 860;
const IP_FROM = 1040;

const SUB = "text-muted-fg block text-xs whitespace-nowrap";
const TRUNCATE = "block truncate";

function Words({ children, sub }: Readonly<{ children: ReactNode; sub?: ReactNode }>) {
  return (
    <span className="min-w-0">
      <span className={TRUNCATE}>{children}</span>
      {sub === undefined ? null : sub}
    </span>
  );
}

/**
 * AuditTable is the audit log's rows as the deck draws them: When, Who,
 * Action, Entity and IP address, which fold by the content's width (When
 * under Who below 700px, Entity under Action below 860px, the address gone
 * below 1040px), and a chevron that opens the diff under the row. A row is
 * open by `open`, which the screen owns, and `onToggle` flips it.
 */
export function AuditTable({
  entries,
  open,
  onToggle,
  empty,
  footer,
}: Readonly<{
  entries: readonly AuditEntry[];
  open: ReadonlySet<number>;
  onToggle: (id: number) => void;
  empty: string;
  footer: ReactNode;
}>) {
  const { t, i18n } = useTranslation();
  const locale = useLocale();
  const zone = useDisplayTimeZone();
  const sentences = useMemo(
    () =>
      i18n.getResource(locale, "translation", "audit.action") as Record<string, string>,
    [i18n, locale],
  );
  const rows = useMemo<readonly Row[]>(
    () => entries.map((entry) => ({ id: String(entry.id), entry })),
    [entries],
  );
  const columns = useMemo<readonly DataColumn<Row>[]>(() => {
    const moment = ({ entry }: Row) => auditMoment(entry.occurredAt, locale, zone);
    const type = (entry: AuditEntry) => {
      const key = `audit.entity.types.${entry.entity.type}`;
      return i18n.exists(key) ? t(key) : humanise(entry.entity.type);
    };
    const entity = ({ entry }: Row) => {
      const { label, id } = entry.entity;
      if (label !== null) {
        return t("audit.entity.named", { type: type(entry), label });
      }
      return id === null ? type(entry) : t("audit.entity.gone", { type: type(entry) });
    };
    return [
      {
        id: "when",
        header: t("audit.columns.when"),
        track: "130px",
        showFrom: WHEN_FROM,
        cell: (row) => (
          <span className="text-muted-fg text-[0.8125rem] whitespace-nowrap tabular-nums">
            {moment(row)}
          </span>
        ),
      },
      {
        id: "who",
        header: t("audit.columns.who"),
        track: "minmax(150px,1.2fr)",
        cell: (row, shown) => {
          const { actor } = row.entry;
          const name = actor?.name ?? t("audit.system");
          return (
            <span className="flex min-w-0 items-center gap-2">
              <Avatar name={name} src={actor?.avatarUrl} size="26" />
              <Words
                sub={
                  shown.has("when") ? undefined : (
                    <span className={SUB}>{moment(row)}</span>
                  )
                }
              >
                {name}
              </Words>
            </span>
          );
        },
      },
      {
        id: "action",
        header: t("audit.columns.action"),
        track: "minmax(200px,2fr)",
        cell: (row, shown) => (
          <span className="flex min-w-0 items-center gap-2">
            <span
              aria-hidden="true"
              className={cn("size-2 flex-none rounded-full", dotOf(row.entry.category))}
            />
            <Words
              sub={
                shown.has("entity") ? undefined : (
                  <span className={cn(SUB, "truncate")}>{entity(row)}</span>
                )
              }
            >
              {sentenceOf(sentences, row.entry.action)}
            </Words>
          </span>
        ),
      },
      {
        id: "entity",
        header: t("audit.columns.entity"),
        track: "minmax(150px,1.2fr)",
        showFrom: ENTITY_FROM,
        cell: (row) => (
          <span className="text-muted-fg block truncate text-[0.8125rem]">
            {entity(row)}
          </span>
        ),
      },
      {
        id: "ip",
        header: t("audit.columns.ip"),
        track: "120px",
        showFrom: IP_FROM,
        cell: ({ entry }) => (
          <span className="text-muted-fg font-mono text-[0.78125rem]">
            {entry.ip ?? "—"}
          </span>
        ),
      },
      {
        id: "details",
        header: t("audit.columns.details"),
        hideHeader: true,
        track: "32px",
        cell: ({ entry }) => {
          const Chevron = open.has(entry.id) ? ChevronUp : ChevronDown;
          return <Chevron aria-hidden="true" className="text-muted-fg size-3.75" />;
        },
      },
    ];
  }, [t, i18n, locale, zone, sentences, open]);
  return (
    <DataTable
      label={t("audit.title")}
      columns={columns}
      rows={rows}
      rowSize={{ minHeight: 50 }}
      rowTone={({ entry }) => (open.has(entry.id) ? "selected" : undefined)}
      onOpen={({ entry }) => onToggle(entry.id)}
      expansion={{
        isOpen: ({ entry }) => open.has(entry.id),
        detail: ({ entry }) => <AuditDiff entry={entry} />,
        label: ({ entry }) =>
          t("audit.row", {
            action: sentenceOf(sentences, entry.action),
            who: entry.actor?.name ?? t("audit.system"),
            when: auditMoment(entry.occurredAt, locale, zone),
          }),
      }}
      empty={empty}
      footer={footer}
    />
  );
}
