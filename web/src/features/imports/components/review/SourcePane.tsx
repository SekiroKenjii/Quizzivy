import {
  memo,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Download } from "lucide-react";
import { Segmented } from "@/components/ui/segmented";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import type { ContentMark } from "@/components/shared/content/model";
import {
  downloadImportSource,
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

/** BlockTag is the chip a source block carries: the question it became, or what it is. */
export interface BlockTag {
  label: string;
  tone: "muted" | "warning";
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
      <span className="border-muted-fg border-b border-dashed" title={colouredLabel}>
        {node}
        <span className="sr-only">{colouredNote}</span>
      </span>
    );
  if (piece.highlighted)
    node = <mark className="bg-brand-soft text-fg rounded-sm">{node}</mark>;
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

const TAG_TONE: Record<BlockTag["tone"], string> = {
  muted: "bg-muted text-muted-fg",
  warning: "bg-warning-soft text-warning-ink",
};

const BlockView = memo(function BlockView({
  block,
  ranges,
  owner,
  tag,
  active,
  tabbable,
  onSelect,
  onRove,
  onFocusBlock,
}: Readonly<{
  block: ImportSourceBlock;
  ranges: readonly Range[];
  owner: string | undefined;
  tag: BlockTag | undefined;
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
  const chip =
    tag === undefined ? null : (
      <span
        className={cn(
          "text-2xs inline-flex h-5 flex-none items-center rounded-[6px] px-1.75 font-medium whitespace-nowrap",
          TAG_TONE[tag.tone],
        )}
      >
        {tag.label}
      </span>
    );
  const className = cn(
    "flex w-full items-start gap-2.5 rounded-[7px] px-2 py-1.5 text-left transition-colors duration-150",
    active && "bg-brand-soft ring-brand ring-1 ring-inset",
  );
  const text = (
    <span className="text-ui min-w-0 flex-1 leading-[1.6] break-words whitespace-pre-wrap">
      {content}
    </span>
  );
  if (owner === undefined)
    return (
      <div data-block-id={block.id} className={cn(className, "text-muted-fg")}>
        {text}
        {chip}
      </div>
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
      className={cn(className, "cursor-pointer", !active && "hover:bg-hover")}
    >
      {text}
      {chip}
    </button>
  );
});

const NO_RANGES: readonly Range[] = [];

function isPasted(source: ImportSource | undefined): boolean {
  return source?.format === "text";
}

function noteOf(
  source: ImportSource | undefined,
  role: ImportSourceRole,
  t: TFunction,
): string {
  if (isPasted(source)) return t("imports.source.notePasted");
  const name = source?.filename ?? "";
  return role === "exam"
    ? t("imports.source.noteExam", { name })
    : t("imports.source.noteKey", { name });
}

function SourceHeader({
  importId,
  sources,
  role,
  removed,
  onRoleChange,
}: Readonly<{
  importId: string;
  sources: readonly ImportSource[];
  role: ImportSourceRole;
  removed: boolean;
  onRoleChange: (role: ImportSourceRole) => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const pageNote = useId();
  const [downloading, setDownloading] = useState(false);
  const exam = sources.find((source) => source.role === "exam");
  const key = sources.find((source) => source.role === "answer_key");
  const current = role === "answer_key" ? key : exam;
  const pasted = isPasted(current);

  const download = async () => {
    if (current === undefined) return;
    setDownloading(true);
    try {
      const link = await downloadImportSource(importId, current.id);
      window.location.assign(link.url);
    } catch (cause) {
      notify.error(failureMessage(cause, t("imports.sources.downloadFailed")));
      if (cause instanceof ApiError && cause.code === "IMPORT_FILES_REMOVED")
        void client.invalidateQueries({ queryKey: ["word-import", importId] });
    } finally {
      setDownloading(false);
    }
  };

  return (
    <>
      <div className="flex flex-none flex-wrap items-center gap-2 border-b px-3 py-2">
        <Segmented
          size="xs"
          label={t("imports.source.which")}
          value={role}
          options={[
            {
              value: "exam",
              label: isPasted(exam)
                ? t("imports.detail.pastedText")
                : t("imports.role.exam"),
            },
            ...(key === undefined
              ? []
              : [{ value: "answer_key", label: t("imports.role.answer_key") }]),
          ]}
          onChange={(value) =>
            onRoleChange(value === "answer_key" ? "answer_key" : "exam")
          }
        />
        <Segmented
          size="xs"
          label={t("imports.source.viewLabel")}
          value="text"
          options={
            pasted
              ? [{ value: "text", label: t("imports.source.asPasted") }]
              : [
                  { value: "text", label: t("imports.source.extractedView") },
                  {
                    value: "page",
                    label: t("imports.source.pageView"),
                    disabled: true,
                    describedBy: pageNote,
                  },
                ]
          }
          onChange={() => undefined}
        />
        <span id={pageNote} hidden>
          {t("imports.source.pageViewLater")}
        </span>
        <span className="flex-1" />
        <button
          type="button"
          aria-label={t("imports.source.download")}
          title={t("imports.source.download")}
          disabled={current === undefined || removed || downloading}
          onClick={() => void download()}
          className="text-muted-fg hover:bg-hover hover:text-fg grid size-7.5 cursor-pointer place-items-center rounded-[7px] disabled:cursor-default disabled:opacity-45"
        >
          <Download aria-hidden="true" className="size-3.75" />
        </button>
      </div>
      <p className="text-muted-fg text-caption m-0 flex-none border-b px-3.5 py-1.5 [overflow-wrap:anywhere]">
        {noteOf(current, role, t)}
      </p>
    </>
  );
}

/**
 * SourcePane shows the extracted text of one source with its marks, tables as
 * rows, each block linked to the question it became and tagged by `tagOf`.
 * Its header switches between the exam and an answer key when there is one,
 * and downloads the original. A pasted exam is named "Pasted text" and read
 * "As pasted", never by its stored filename; a file's "Page view" is offered
 * but off until there is a rendering of it. The blocks share one tab stop
 * and are moved between with the arrow keys. The pane scrolls to the focused
 * block when `focus.nonce` changes or the pane becomes `visible`, and moves
 * keyboard focus there once per nonce when `focus.take` is set.
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
  tagOf,
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
  tagOf: (key: string) => BlockTag | undefined;
  selectedBlocks: ReadonlySet<string>;
  onSelect: (questionId: string) => void;
}>) {
  const { t } = useTranslation();
  const scroller = useRef<HTMLDivElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const focused = useRef(0);
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

  const renderBlock = (block: ImportSourceBlock) => {
    const blockAt = blockKey(sourceId ?? "", block.id);
    return (
      <BlockView
        key={block.id}
        block={block}
        ranges={ranges.get(block.id) ?? NO_RANGES}
        owner={owners.get(blockAt)}
        tag={tagOf(blockAt)}
        active={selectedBlocks.has(blockAt) || ranges.has(block.id)}
        tabbable={block.id === tabStop}
        onSelect={onSelect}
        onRove={rove}
        onFocusBlock={setRoving}
      />
    );
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <h2 ref={heading} tabIndex={-1} className="sr-only">
        {t("imports.source.title")}
      </h2>
      <SourceHeader
        importId={importId}
        sources={sources}
        role={role}
        removed={removed}
        onRoleChange={onRoleChange}
      />
      <div
        ref={scroller}
        className="bg-card min-h-0 flex-1 overflow-y-auto px-3 py-2.5"
      >
        {view.isPending ? <ListSkeleton rows={8} /> : null}
        {removed ? (
          <p className="text-muted-fg text-sm">
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
          <p className="text-muted-fg text-sm">{t("imports.source.empty")}</p>
        ) : null}
        <div className="mx-auto flex max-w-160 flex-col gap-0.5">
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
