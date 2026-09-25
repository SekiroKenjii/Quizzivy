import {
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Segmented } from "@/components/ui/segmented";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { ApiError } from "@/lib/api/errors";
import { cn } from "@/lib/utils";
import type { ContentMark } from "@/components/shared/content/model";
import {
  type ImportSource,
  type ImportSourceBlock,
  type ImportSourceRef,
  type ImportSourceRole,
} from "../../api";
import { blockKey } from "../../draft";
import { sourceViewQuery } from "../../queries";

/**
 * SourceFocus is what the source pane highlights, a nonce that asks it to
 * scroll there once, and whether keyboard focus should move there too.
 */
export interface SourceFocus {
  refs: readonly ImportSourceRef[];
  nonce: number;
  take: boolean;
}

type Range = { start: number; end: number };

type Piece = {
  text: string;
  marks: ContentMark[];
  colored: boolean;
  highlighted: boolean;
};

function pieces(block: ImportSourceBlock, ranges: readonly Range[]): Piece[] {
  const points = Array.from(block.text);
  const cuts = new Set<number>([0, points.length]);
  for (const span of block.spans) {
    cuts.add(Math.max(0, Math.min(points.length, span.start)));
    cuts.add(Math.max(0, Math.min(points.length, span.end)));
  }
  for (const range of ranges) {
    cuts.add(Math.max(0, Math.min(points.length, range.start)));
    cuts.add(Math.max(0, Math.min(points.length, range.end)));
  }
  const ordered = [...cuts].sort((a, b) => a - b);
  const out: Piece[] = [];
  for (let i = 0; i + 1 < ordered.length; i += 1) {
    const start = ordered[i]!;
    const end = ordered[i + 1]!;
    if (start === end) continue;
    const covering = block.spans.filter(
      (span) => span.start <= start && span.end >= end,
    );
    out.push({
      text: points.slice(start, end).join(""),
      marks: [...new Set(covering.flatMap((span) => span.marks))],
      colored: covering.some((span) => span.colored === true),
      highlighted: ranges.some((range) => range.start <= start && range.end >= end),
    });
  }
  return out;
}

function marked(piece: Piece, colouredLabel: string, colouredNote: string): ReactNode {
  let node: ReactNode = piece.text;
  for (const mark of piece.marks) {
    if (mark === "bold") node = <strong>{node}</strong>;
    else if (mark === "italic") node = <em>{node}</em>;
    else if (mark === "underline") node = <u>{node}</u>;
    else if (mark === "strike") node = <s>{node}</s>;
    else if (mark === "superscript") node = <sup>{node}</sup>;
    else if (mark === "subscript") node = <sub>{node}</sub>;
  }
  if (piece.colored)
    node = (
      <span
        className="border-muted-foreground border-b border-dashed"
        title={colouredLabel}
      >
        {node}
        <span className="sr-only">{colouredNote}</span>
      </span>
    );
  if (piece.highlighted)
    node = (
      <mark className="bg-foreground/10 text-foreground ring-foreground/40 rounded-sm ring-1">
        {node}
      </mark>
    );
  return node;
}

type Segment =
  | { kind: "block"; block: ImportSourceBlock }
  | { kind: "table"; id: string; blocks: ImportSourceBlock[] };

function segmentsOf(blocks: readonly ImportSourceBlock[]): Segment[] {
  const out: Segment[] = [];
  for (const block of blocks) {
    const last = out.at(-1);
    if (block.tableId === undefined) out.push({ kind: "block", block });
    else if (last?.kind === "table" && last.id === block.tableId)
      last.blocks.push(block);
    else out.push({ kind: "table", id: block.tableId, blocks: [block] });
  }
  return out;
}

function rowsOf(blocks: readonly ImportSourceBlock[]): ImportSourceBlock[][] {
  const rows = new Map<number, ImportSourceBlock[]>();
  for (const block of blocks) {
    const row = block.row ?? 0;
    rows.set(row, [...(rows.get(row) ?? []), block]);
  }
  return [...rows.entries()]
    .sort(([a], [b]) => a - b)
    .map(([, cells]) => [...cells].sort((a, b) => (a.column ?? 0) - (b.column ?? 0)));
}

const BlockView = memo(function BlockView({
  block,
  ranges,
  owner,
  active,
  tabbable,
  onSelect,
  onRove,
  onFocusBlock,
}: Readonly<{
  block: ImportSourceBlock;
  ranges: readonly Range[];
  owner: string | undefined;
  active: boolean;
  tabbable: boolean;
  onSelect: (questionId: string) => void;
  onRove: (event: KeyboardEvent<HTMLButtonElement>, blockId: string) => void;
  onFocusBlock: (blockId: string) => void;
}>) {
  const { t } = useTranslation();
  const content = pieces(block, ranges).map((piece, index) => (
    <span key={index}>
      {marked(piece, t("imports.source.colored"), t("imports.source.coloredNote"))}
    </span>
  ));
  const className = cn(
    "block w-full rounded-sm px-1.5 py-1 text-left text-sm leading-relaxed whitespace-pre-wrap break-words transition-colors duration-150",
    active && "bg-secondary",
  );
  if (owner === undefined)
    return (
      <p data-block-id={block.id} className={cn(className, "text-muted-foreground")}>
        {content}
      </p>
    );
  return (
    <button
      type="button"
      data-block-id={block.id}
      aria-pressed={active}
      tabIndex={tabbable ? 0 : -1}
      onClick={() => onSelect(owner)}
      onKeyDown={(event) => onRove(event, block.id)}
      onFocus={() => onFocusBlock(block.id)}
      className={cn(
        className,
        "hover:bg-secondary/60 focus-visible:ring-ring focus-visible:ring-2 focus-visible:outline-none",
      )}
    >
      {content}
    </button>
  );
});

const NO_RANGES: readonly Range[] = [];

/**
 * SourcePane shows the extracted text of one source with its marks, tables as
 * rows, and each block linked to the question it became. The blocks share one
 * tab stop and are moved between with the arrow keys. The pane scrolls to the
 * focused block when `focus.nonce` changes or the pane becomes `visible`, and
 * moves keyboard focus there once per nonce when `focus.take` is set.
 */
export function SourcePane({
  importId,
  visible,
  sourceRevision,
  sources,
  role,
  onRoleChange,
  focus,
  owners,
  selectedBlocks,
  onSelect,
}: Readonly<{
  importId: string;
  visible: boolean;
  sourceRevision: number;
  sources: readonly ImportSource[];
  role: ImportSourceRole;
  onRoleChange: (role: ImportSourceRole) => void;
  focus: SourceFocus;
  owners: ReadonlyMap<string, string>;
  selectedBlocks: ReadonlySet<string>;
  onSelect: (questionId: string) => void;
}>) {
  const { t } = useTranslation();
  const scroller = useRef<HTMLDivElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const focused = useRef(0);
  const hasKey = sources.some((source) => source.role === "answer_key");
  const view = useQuery(sourceViewQuery(importId, role, sourceRevision));
  const removed =
    view.error instanceof ApiError && view.error.code === "IMPORT_FILES_REMOVED";
  const sourceId = view.data?.sourceId;
  const ranges = useMemo(() => {
    const byBlock = new Map<string, Range[]>();
    for (const ref of focus.refs) {
      if (ref.sourceId !== sourceId) continue;
      const list = byBlock.get(ref.blockId) ?? [];
      list.push({ start: ref.start, end: ref.end });
      byBlock.set(ref.blockId, list);
    }
    return byBlock;
  }, [focus.refs, sourceId]);
  const segments = useMemo(() => segmentsOf(view.data?.blocks ?? []), [view.data]);
  const target = focus.refs.find((ref) => ref.sourceId === sourceId)?.blockId;
  const [roving, setRoving] = useState<string | null>(null);
  const linked = useMemo(
    () =>
      segments
        .flatMap((segment) =>
          segment.kind === "block" ? [segment.block] : rowsOf(segment.blocks).flat(),
        )
        .map((block) => block.id)
        .filter((id) => owners.has(blockKey(sourceId ?? "", id))),
    [owners, segments, sourceId],
  );
  const tabStop =
    roving !== null && linked.includes(roving)
      ? roving
      : (linked.find((id) => selectedBlocks.has(blockKey(sourceId ?? "", id))) ??
        linked[0]);

  const rove = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>, blockId: string) => {
      const at = linked.indexOf(blockId);
      let next: number;
      if (event.key === "ArrowDown" || event.key === "ArrowRight") next = at + 1;
      else if (event.key === "ArrowUp" || event.key === "ArrowLeft") next = at - 1;
      else if (event.key === "Home") next = 0;
      else if (event.key === "End") next = linked.length - 1;
      else return;
      event.preventDefault();
      const id = linked[Math.max(0, Math.min(linked.length - 1, next))];
      if (id === undefined) return;
      setRoving(id);
      const element = [
        ...(scroller.current?.querySelectorAll<HTMLElement>("button[data-block-id]") ??
          []),
      ].find((candidate) => candidate.dataset["blockId"] === id);
      element?.focus();
    },
    [linked],
  );

  useEffect(() => {
    if (!visible || focus.nonce === 0) return;
    const found =
      target === undefined
        ? undefined
        : [
            ...(scroller.current?.querySelectorAll<HTMLElement>("[data-block-id]") ??
              []),
          ].find((element) => element.dataset["blockId"] === target);
    found?.scrollIntoView({ block: "center" });
    if (!focus.take || focused.current === focus.nonce) return;
    focused.current = focus.nonce;
    if (found instanceof HTMLButtonElement) found.focus({ preventScroll: true });
    else heading.current?.focus({ preventScroll: true });
  }, [focus.nonce, focus.take, target, visible]);

  const renderBlock = (block: ImportSourceBlock) => (
    <BlockView
      key={block.id}
      block={block}
      ranges={ranges.get(block.id) ?? NO_RANGES}
      owner={owners.get(blockKey(sourceId ?? "", block.id))}
      active={
        selectedBlocks.has(blockKey(sourceId ?? "", block.id)) || ranges.has(block.id)
      }
      tabbable={block.id === tabStop}
      onSelect={onSelect}
      onRove={rove}
      onFocusBlock={setRoving}
    />
  );

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b px-4 py-2">
        <h2 ref={heading} tabIndex={-1} className="text-sm font-medium outline-none">
          {t("imports.source.title")}
        </h2>
        {hasKey ? (
          <Segmented
            className="ml-auto"
            label={t("imports.source.which")}
            value={role}
            options={[
              { value: "exam", label: t("imports.role.exam") },
              { value: "answer_key", label: t("imports.role.answer_key") },
            ]}
            onChange={(value) =>
              onRoleChange(value === "answer_key" ? "answer_key" : "exam")
            }
          />
        ) : null}
      </div>
      <div ref={scroller} className="min-h-0 flex-1 overflow-y-auto px-3 py-3">
        {view.data === undefined ? null : (
          <p className="text-muted-foreground mb-2 px-1.5 text-xs break-all">
            {t("imports.source.extracted", { name: view.data.filename })}
          </p>
        )}
        {view.isPending ? <ListSkeleton rows={8} /> : null}
        {removed ? (
          <p className="text-muted-foreground text-sm">
            {t("imports.retention.sourceRemoved")}
          </p>
        ) : null}
        {view.isError && !removed ? (
          <LoadError error={view.error} onRetry={() => void view.refetch()}>
            {view.error instanceof ApiError &&
            view.error.code === "IMPORT_NOT_PROCESSED"
              ? t("imports.source.notExtracted")
              : t("imports.source.failed")}
          </LoadError>
        ) : null}
        {view.data !== undefined && view.data.blocks.length === 0 ? (
          <p className="text-muted-foreground text-sm">{t("imports.source.empty")}</p>
        ) : null}
        <div className="max-w-prose space-y-0.5">
          {segments.map((segment) =>
            segment.kind === "block" ? (
              renderBlock(segment.block)
            ) : (
              <div key={segment.id} className="overflow-x-auto py-1">
                <table
                  className="w-full border-collapse text-sm"
                  aria-label={t("imports.source.table")}
                >
                  <tbody>
                    {rowsOf(segment.blocks).map((row, index) => (
                      <tr key={index} className="border-b last:border-b-0">
                        {row.map((block) => (
                          <td
                            key={block.id}
                            className="border-r p-0.5 align-top last:border-r-0"
                          >
                            {renderBlock(block)}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ),
          )}
        </div>
      </div>
    </div>
  );
}
