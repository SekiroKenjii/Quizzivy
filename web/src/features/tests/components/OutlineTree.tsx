import { Tooltip } from "@/components/shared/Tooltip";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragMoveEvent,
  type KeyboardCoordinateGetter,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";

import {
  ChevronDown,
  ChevronRight,
  ChevronUp,
  CircleAlert,
  CircleDot,
  ListChecks,
  ToggleLeft,
  TextCursorInput,
  PenLine,
  Ellipsis,
  GripVertical,
  Headphones,
  Library,
  Layers,
  Pencil,
  Plus,
  ScrollText,
  Trash2,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import { Textarea } from "@/components/ui/textarea";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { MarqueeText } from "@/components/shared/MarqueeText";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import { cn } from "@/lib/utils";
import { moveSection, type OutlineSection } from "@/features/tests/outline";
import {
  findUnit,
  moveUnit,
  removeUnit,
  sectionQuestionIds,
  stepUnit,
  unitKey,
  unitsOf,
} from "../outlineUnits";
import type { GroupBundle } from "@/features/question-groups/api";
import { OutlineGroupRow } from "./OutlineGroupRow";
import type { TFunction } from "i18next";

/** OutlineQuestion carries a loaded question’s title, score, type and publish finding in the outline. */
export interface OutlineQuestion {
  id: string;
  prompt: string;
  points: number;
  hasAudio: boolean;
  type?: QuestionValues["type"];
  problem: string | null;
}

type DropPosition = "before" | "after" | undefined;

interface OutlineTreeProps {
  sections: OutlineSection[];
  questions: Map<string, OutlineQuestion>;
  selectedId: string | null;
  creating: boolean;
  onSelect: (questionId: string) => void;
  onChange: (sections: OutlineSection[]) => void;
  onCreateQuestion: () => void;
  onPickFromBank: () => void;
  onAddSection: () => void;
  groups?: Map<string, GroupBundle>;
  selectedGroupId?: string | null;
  onSelectGroup?: (id: string, questionId?: string) => void;
  onCreateGroup?: () => void;
  onRemoveGroup?: (id: string) => void;
  onRemoveSection?: (index: number) => void;
}

/** OutlineTree edits test sections and whole question or shared-context units in reading order. */
export function OutlineTree({
  sections,
  questions,
  selectedId,
  creating,
  onSelect,
  onChange,
  onCreateQuestion,
  onPickFromBank,
  onAddSection,
  groups = new Map(),
  selectedGroupId,
  onSelectGroup,
  onCreateGroup,
  onRemoveGroup,
  onRemoveSection,
}: Readonly<OutlineTreeProps>) {
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());
  // A section without a server id yet is keyed by a client key that travels
  // with it, so collapsing it and then moving it keeps the state on the section.
  const [clientKeys, setClientKeys] = useState<string[]>([]);
  if (clientKeys.length < sections.length) {
    setClientKeys([
      ...clientKeys,
      ...Array.from({ length: sections.length - clientKeys.length }, clientKey),
    ]);
  }
  const keyFor = (section: OutlineSection, index: number): string =>
    section.clientId ?? section.id ?? clientKeys[index] ?? `new-${index}`;
  const [renaming, setRenaming] = useState<number | null>(null);
  const [instructing, setInstructing] = useState<number | null>(null);
  const [removing, setRemoving] = useState<number | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<{ id: string; after: boolean } | null>(
    null,
  );
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: outlineKeyboardCoordinates }),
  );

  function trackDrop(event: DragMoveEvent) {
    const over = event.over;
    if (!over) {
      setDropTarget(null);
      return;
    }
    const rect = event.active.rect.current.translated;
    const after =
      rect !== null &&
      rect.top + rect.height / 2 > over.rect.top + over.rect.height / 2;
    const id = String(over.id);
    setDropTarget((current) =>
      current?.id === id && current.after === after ? current : { id, after },
    );
  }

  function dropSection(event: DragEndEvent, keyboard: boolean) {
    const from = event.active.data.current?.["sectionIndex"];
    const target = event.over?.data.current?.["sectionIndex"];
    if (typeof from !== "number" || typeof target !== "number") return;
    let to = target;
    if (!keyboard) {
      to += isAfterDrop(event) ? 1 : 0;
      if (from < to) to -= 1;
    }
    setClientKeys(moveSection(clientKeys, from, to));
    onChange(moveSection(sections, from, to));
  }

  function dropUnit(event: DragEndEvent, keyboard: boolean) {
    const from = findUnit(sections, String(event.active.id));
    const sectionIndex = event.over?.data.current?.["sectionIndex"];
    const to =
      typeof sectionIndex === "number"
        ? { sectionIndex, index: 0 }
        : findUnit(sections, String(event.over?.id));
    if (!from || !to) return;
    if (!keyboard && typeof sectionIndex !== "number") {
      to.index += isAfterDrop(event) ? 1 : 0;
      if (from.sectionIndex === to.sectionIndex && from.index < to.index) to.index -= 1;
    }
    if (typeof sectionIndex === "number") {
      const destination = sections[sectionIndex];
      if (destination)
        setCollapsed((current) => {
          const next = new Set(current);
          next.delete(keyFor(destination, sectionIndex));
          return next;
        });
    }
    onChange(moveUnit(sections, from, to));
  }

  function onDragEnd(event: DragEndEvent) {
    setDragging(null);
    setDropTarget(null);
    if (!event.over || event.over.id === event.active.id) return;
    const keyboard = event.activatorEvent instanceof KeyboardEvent;
    if (event.active.data.current?.["kind"] === "section") dropSection(event, keyboard);
    else dropUnit(event, keyboard);
  }

  function drop(questionId: string) {
    onChange(removeUnit(sections, questionId));
  }

  function step(questionId: string, direction: -1 | 1) {
    const from = findUnit(sections, questionId);
    if (!from) return;
    const to = stepUnit(sections, from, direction);
    if (!to) return;
    onChange(moveUnit(sections, from, to));
  }

  function rename(index: number, title: string | null) {
    setRenaming(null);
    if (title === null || title === sections[index]?.title) return;
    onChange(sections.map((s, i) => (i === index ? { ...s, title } : s)));
  }

  function saveInstructions(index: number, instructions: string) {
    setInstructing(null);
    const value = instructions.trim();
    onChange(
      sections.map((s, i) =>
        i === index ? { ...s, instructions: value === "" ? null : value } : s,
      ),
    );
  }

  function remove(index: number) {
    setRemoving(null);
    setClientKeys(clientKeys.filter((_, i) => i !== index));
    onChange(sections.filter((_, i) => i !== index));
  }

  function move(index: number, direction: -1 | 1) {
    setClientKeys(moveSection(clientKeys, index, index + direction));
    onChange(moveSection(sections, index, index + direction));
  }

  const numbering = numberQuestions(sections, groups);

  const settled = sections.every((section) =>
    unitsOf(section).every((unit) =>
      unit.kind === "question" ? questions.has(unit.id) : groups.has(unit.id),
    ),
  );

  const editing = instructing === null ? null : (sections[instructing] ?? null);
  const doomed = removing === null ? null : (sections[removing] ?? null);

  return (
    <div className="bg-card shadow-card flex h-full min-h-0 w-full flex-col overflow-hidden rounded-xl border">
      <div className="flex shrink-0 items-center gap-2 border-b px-3.5 py-3">
        <p className="text-sm font-semibold">{t("builder.outline")}</p>
        <p className="text-muted-foreground text-caption ml-auto shrink-0 tabular-nums">
          {settled
            ? t("builder.outlineSummary", {
                questions: numbering.size,
                points: totalPoints(sections, questions, groups),
              })
            : t("common.loading")}
        </p>
      </div>

      <DndContext
        accessibility={{
          screenReaderInstructions: { draggable: t("builder.dragInstructions") },
          announcements: {
            onDragStart: () => t("builder.dragStarted"),
            onDragOver: ({ over }) => (over ? t("builder.dragOver") : undefined),
            onDragEnd: () => t("builder.dragDropped"),
            onDragCancel: () => t("builder.dragCancelled"),
          },
        }}
        sensors={sensors}
        collisionDetection={(args) =>
          closestCenter({
            ...args,
            droppableContainers: args.droppableContainers.filter((container) =>
              args.active.data.current?.["kind"] === "section"
                ? container.data.current?.["kind"] === "section"
                : container.data.current?.["kind"] !== "section",
            ),
          })
        }
        onDragStart={(event) => setDragging(String(event.active.id))}
        onDragMove={trackDrop}
        onDragOver={trackDrop}
        onDragCancel={() => {
          setDragging(null);
          setDropTarget(null);
        }}
        onDragEnd={onDragEnd}
      >
        <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-2">
          <SortableContext
            items={sections.map(
              (section, index) => `section:${keyFor(section, index)}`,
            )}
            strategy={verticalListSortingStrategy}
          >
            {sections.map((section, sectionIndex) => {
              const key = keyFor(section, sectionIndex);
              const open = !collapsed.has(key);
              return (
                <div
                  key={key}
                  data-outline-section={section.id ?? undefined}
                  className={dragging === `section:${key}` ? "opacity-45" : undefined}
                >
                  <SectionHeader
                    id={`section:${key}`}
                    sectionIndex={sectionIndex}
                    dragging={dragging !== null}
                    drop={dropPosition(dropTarget, `section:${key}`)}
                    title={section.title}
                    summary={
                      settled
                        ? t("builder.sectionSummary", {
                            questions: sectionQuestionIds(section, groups).length,
                            points: sectionPoints(section, questions, groups),
                          })
                        : t("common.loading")
                    }
                    open={open}
                    renaming={renaming === sectionIndex}
                    first={sectionIndex === 0}
                    last={sectionIndex === sections.length - 1}
                    onToggle={() => setCollapsed((current) => toggle(current, key))}
                    onStartRename={() => setRenaming(sectionIndex)}
                    onRenamed={(next) => rename(sectionIndex, next)}
                    onInstructions={() => setInstructing(sectionIndex)}
                    onMove={(direction) => move(sectionIndex, direction)}
                    onRemove={() => {
                      const units = unitsOf(section);
                      if (units.length === 0) remove(sectionIndex);
                      else if (
                        onRemoveSection &&
                        units.some((unit) => unit.kind === "group")
                      )
                        onRemoveSection(sectionIndex);
                      else setRemoving(sectionIndex);
                    }}
                  >
                    {open && section.instructions ? (
                      <button
                        type="button"
                        className="text-muted-foreground hover:text-foreground mb-1 block w-full truncate px-9 text-left text-xs"
                        onClick={() => setInstructing(sectionIndex)}
                      >
                        {section.instructions}
                      </button>
                    ) : null}
                    {open ? (
                      <SortableContext
                        items={unitsOf(section).map(unitKey)}
                        strategy={verticalListSortingStrategy}
                      >
                        <div className="ml-3.5 space-y-px border-l py-1 pl-1">
                          {unitsOf(section).map((unit) =>
                            unit.kind === "group" ? (
                              <OutlineGroupRow
                                key={unitKey(unit)}
                                id={unit.id}
                                group={groups.get(unit.id)}
                                numbering={numbering}
                                drop={dropPosition(dropTarget, unitKey(unit))}
                                selected={selectedGroupId === unit.id}
                                selectedQuestionId={selectedId}
                                onSelect={(questionId) =>
                                  onSelectGroup?.(unit.id, questionId)
                                }
                                onStep={(direction) => step(unitKey(unit), direction)}
                                onRemove={() => onRemoveGroup?.(unit.id)}
                              />
                            ) : (
                              <OutlineRow
                                key={unitKey(unit)}
                                number={numbering.get(unit.id) ?? 0}
                                question={questions.get(unit.id)}
                                questionId={unit.id}
                                drop={dropPosition(dropTarget, unitKey(unit))}
                                canUp={
                                  stepUnit(
                                    sections,
                                    findUnit(sections, unitKey(unit))!,
                                    -1,
                                  ) !== null
                                }
                                canDown={
                                  stepUnit(
                                    sections,
                                    findUnit(sections, unitKey(unit))!,
                                    1,
                                  ) !== null
                                }
                                selected={!selectedGroupId && unit.id === selectedId}
                                onSelect={() => onSelect(unit.id)}
                                onStep={(direction) => step(unitKey(unit), direction)}
                                onDrop={() => drop(unitKey(unit))}
                              />
                            ),
                          )}
                          {unitsOf(section).length === 0 ? (
                            <EmptySectionDrop
                              id={`section-${key}`}
                              sectionIndex={sectionIndex}
                            />
                          ) : null}
                        </div>
                      </SortableContext>
                    ) : null}
                  </SectionHeader>
                </div>
              );
            })}
          </SortableContext>
        </div>
      </DndContext>

      <div className="shrink-0 space-y-1 border-t p-2">
        <Button
          variant="outline"
          size="sm"
          className="w-full justify-start in-data-[scale=deck]:h-8"
          disabled={creating || sections.length === 0}
          onClick={onCreateQuestion}
        >
          <Plus aria-hidden="true" />
          <span className="min-w-0 flex-1 text-left">
            <span className="block">
              {t(
                selectedGroupId
                  ? "builder.addStandaloneQuestion"
                  : "builder.addQuestion",
              )}
            </span>
            {sections.length === 0 ? null : (
              <span className="text-muted-foreground text-caption block truncate font-normal">
                {t("builder.addQuestionTo", {
                  title: abbreviatedTarget(sections, selectedGroupId, selectedId),
                })}
              </span>
            )}
          </span>
        </Button>
        {onCreateGroup ? (
          <Button
            variant="outline"
            size="sm"
            className="w-full justify-start in-data-[scale=deck]:h-8"
            disabled={creating || sections.length === 0}
            onClick={onCreateGroup}
          >
            <Layers aria-hidden="true" />
            {t("builder.addSharedGroup")}
          </Button>
        ) : null}
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground w-full justify-start"
          disabled={sections.length === 0}
          onClick={onPickFromBank}
        >
          <Library aria-hidden="true" />
          {t("builder.fromBank")}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground w-full justify-start"
          onClick={onAddSection}
        >
          <Plus aria-hidden="true" />
          {t("builder.addSection")}
        </Button>
      </div>

      {editing === null || instructing === null ? null : (
        <SectionInstructionsDialog
          key={editing.id ?? instructing}
          sectionTitle={editing.title}
          instructions={editing.instructions ?? ""}
          onCancel={() => setInstructing(null)}
          onSave={(value) => saveInstructions(instructing, value)}
        />
      )}

      <ConfirmDialog
        open={doomed !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={t("builder.removeSectionTitle", { title: doomed?.title ?? "" })}
        description={t("builder.removeSectionBody", {
          count: doomed ? sectionQuestionIds(doomed, groups).length : 0,
        })}
        confirmLabel={t("builder.removeSection")}
        destructive
        onConfirm={() => removing !== null && remove(removing)}
      />
    </div>
  );
}

const KEY = { enter: "↵", escape: "esc" } as const;

function SectionHeader({
  children,
  id,
  sectionIndex,
  dragging,
  drop,
  title,
  summary,
  open,
  renaming,
  first,
  last,
  onToggle,
  onStartRename,
  onRenamed,
  onInstructions,
  onMove,
  onRemove,
}: Readonly<{
  children: ReactNode;
  id: string;
  sectionIndex: number;
  dragging: boolean;
  drop: DropPosition;
  title: string;
  summary: string;
  open: boolean;
  renaming: boolean;
  first: boolean;
  last: boolean;
  onToggle: () => void;
  onStartRename: () => void;
  onRenamed: (title: string | null) => void;
  onInstructions: () => void;
  onMove: (direction: -1 | 1) => void;
  onRemove: () => void;
}>) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef } = useSortable({
    id,
    data: { kind: "section", sectionIndex },
    disabled: renaming,
    transition: null,
  });
  const { setNodeRef: setHeaderNodeRef, isOver: headerIsOver } = useDroppable({
    id: `head:${id}`,
    data: { sectionIndex },
  });
  const trigger = useRef<HTMLButtonElement>(null);
  const wasRenaming = useRef(renaming);
  const Chevron = open ? ChevronDown : ChevronRight;

  useEffect(() => {
    if (wasRenaming.current && !renaming && document.activeElement === document.body) {
      trigger.current?.focus();
    }
    wasRenaming.current = renaming;
  }, [renaming]);

  return (
    <div ref={setNodeRef} className="relative">
      {drop ? (
        <span
          aria-hidden="true"
          data-outline-drop={drop}
          className={cn(
            "bg-primary pointer-events-none absolute inset-x-0 h-0.5",
            drop === "before" ? "top-0" : "bottom-0",
          )}
        />
      ) : null}
      {renaming ? (
        <div className="flex items-start gap-1.5 px-1.5 py-1">
          <Chevron
            className="text-muted-foreground mt-1.5 size-3.5 shrink-0"
            aria-hidden="true"
          />
          <SectionTitleInput title={title} onDone={onRenamed} />
        </div>
      ) : (
        <div
          ref={setHeaderNodeRef}
          className={cn(
            "group/section hover:bg-hover has-data-[state=open]:bg-hover relative flex items-center gap-1 rounded-lg px-1.5 py-1 transition-colors duration-120 ease-[cubic-bezier(.25,.1,.25,1)] motion-reduce:transition-none",
            headerIsOver && "bg-accent-soft",
          )}
        >
          <button
            type="button"
            aria-label={t("builder.reorderSection", { title })}
            className="text-muted-foreground flex h-6 w-5 shrink-0 cursor-grab touch-none items-center justify-center active:cursor-grabbing"
            {...attributes}
            {...listeners}
          >
            <GripVertical aria-hidden="true" className="size-3.5" />
          </button>
          <button
            type="button"
            className="group/section-title flex min-w-0 flex-1 items-center gap-1.5 text-left"
            aria-expanded={open}
            onClick={onToggle}
            onDoubleClick={onStartRename}
          >
            <Chevron
              className="text-muted-foreground size-3.5 shrink-0"
              aria-hidden="true"
            />
            <MarqueeText
              text={title}
              minSeconds={6}
              gapPx={28}
              maskPx={10}
              className={cn(
                "text-meta min-w-0 flex-1 font-semibold group-focus-within/section:[&_.qz-marquee-track]:[animation-play-state:paused]! group-hover/section:[&_.qz-marquee-track]:[animation-play-state:paused]!",
                dragging && "[&_.qz-marquee-track]:[animation:none]!",
              )}
            />{" "}
            <span className="text-muted-foreground text-caption ml-auto shrink-0 tabular-nums">
              {summary}
            </span>
          </button>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                ref={trigger}
                variant="ghost"
                size="icon-xs"
                aria-label={t("builder.sectionActions")}
                className="text-muted-foreground shrink-0 opacity-100 group-focus-within/section:opacity-100 group-hover/section:opacity-100 data-[state=open]:opacity-100 min-[768px]:opacity-0"
              >
                <Ellipsis aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              className="w-48 data-[state=closed]:animate-none! data-[state=open]:animate-none!"
            >
              <DropdownMenuItem onSelect={onStartRename}>
                <Pencil aria-hidden="true" />
                {t("builder.renameSection")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={onInstructions}>
                <ScrollText aria-hidden="true" />
                {t("builder.sectionInstructions")}
              </DropdownMenuItem>
              <DropdownMenuItem disabled={first} onSelect={() => onMove(-1)}>
                <ChevronUp aria-hidden="true" />
                {t("builder.moveSectionUp")}
              </DropdownMenuItem>
              <DropdownMenuItem disabled={last} onSelect={() => onMove(1)}>
                <ChevronDown aria-hidden="true" />
                {t("builder.moveSectionDown")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={onRemove}>
                <Trash2 aria-hidden="true" />
                {t("builder.removeSection")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}
      {children}
    </div>
  );
}

function EmptySectionDrop({
  id,
  sectionIndex,
}: Readonly<{ id: string; sectionIndex: number }>) {
  const { t } = useTranslation();
  const { setNodeRef, isOver } = useDroppable({ id, data: { sectionIndex } });
  return (
    <div
      ref={setNodeRef}
      className={cn(
        "text-muted-foreground min-h-12 rounded-md border border-dashed px-2 py-3 text-xs",
        isOver && "bg-accent border-foreground",
      )}
    >
      {t("builder.sectionDropHint")}
    </div>
  );
}

function SectionTitleInput({
  title,
  onDone,
}: Readonly<{
  title: string;
  onDone: (title: string | null) => void;
}>) {
  const { t } = useTranslation();
  const [value, setValue] = useState(title);
  const field = useRef<HTMLInputElement>(null);
  const settled = useRef(false);

  useEffect(() => {
    field.current?.focus();
    field.current?.select();
  }, []);

  function finish(next: string | null) {
    if (settled.current) return;
    settled.current = true;
    onDone(next);
  }

  return (
    <div className="min-w-0 flex-1">
      <Input
        ref={field}
        value={value}
        aria-label={t("builder.sectionNameLabel")}
        className="h-7 px-2 text-xs font-semibold"
        onChange={(event) => setValue(event.target.value)}
        onBlur={() => finish(value.trim() === "" ? null : value.trim())}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            event.preventDefault();
            finish(value.trim() === "" ? null : value.trim());
          }
          if (event.key === "Escape") {
            event.preventDefault();
            finish(null);
          }
        }}
      />
      <p className="text-muted-foreground mt-1.5 flex items-center gap-1 text-xs">
        <Kbd>{KEY.enter}</Kbd> {t("builder.renameCommit")} · <Kbd>{KEY.escape}</Kbd>{" "}
        {t("builder.renameDiscard")}
      </p>
    </div>
  );
}

function SectionInstructionsDialog({
  sectionTitle,
  instructions,
  onCancel,
  onSave,
}: Readonly<{
  sectionTitle: string;
  instructions: string;
  onCancel: () => void;
  onSave: (instructions: string) => void;
}>) {
  const { t } = useTranslation();
  const [value, setValue] = useState(instructions);

  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => !open && onCancel()}
      title={t("builder.sectionInstructionsTitle", { title: sectionTitle })}
      description={t("builder.sectionInstructionsHint")}
      confirmLabel={t("common.save")}
      onConfirm={() => onSave(value)}
    >
      <Textarea
        // eslint-disable-next-line jsx-a11y/no-autofocus
        autoFocus
        value={value}
        aria-label={t("builder.sectionInstructions")}
        placeholder={t("builder.sectionInstructionsPlaceholder")}
        onChange={(event) => setValue(event.target.value)}
      />
    </ConfirmDialog>
  );
}

function OutlineRow({
  number,
  question,
  questionId,
  drop,
  canUp,
  canDown,
  selected,
  onSelect,
  onStep,
  onDrop,
}: Readonly<{
  number: number;
  question: OutlineQuestion | undefined;
  questionId: string;
  drop: DropPosition;
  canUp: boolean;
  canDown: boolean;
  selected: boolean;
  onSelect: () => void;
  onStep: (direction: -1 | 1) => void;
  onDrop: () => void;
}>) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef, isDragging } = useSortable({
    id: `question:${questionId}`,
    transition: null,
  });

  const loading = question === undefined;
  const label = question?.prompt.trim() ?? "";
  const Icon = question?.hasAudio
    ? Headphones
    : {
        single_choice: CircleDot,
        multiple_choice: ListChecks,
        true_false: ToggleLeft,
        fill_blank: TextCursorInput,
        short_answer: PenLine,
      }[question?.type ?? "single_choice"];

  return (
    <div
      ref={setNodeRef}
      data-outline-row=""
      className={cn(
        "group/row hover:bg-hover text-meta relative flex min-h-8 items-center gap-1.5 rounded-md px-1.5 py-1",
        selected ? "bg-secondary text-foreground font-medium" : "text-muted-foreground",
        question?.problem ? "text-destructive-ink" : "",
        isDragging ? "opacity-40" : "",
      )}
    >
      {drop ? (
        <span
          aria-hidden="true"
          data-outline-drop={drop}
          className={cn(
            "bg-primary pointer-events-none absolute inset-x-0 h-0.5",
            drop === "before" ? "top-0" : "bottom-0",
          )}
        />
      ) : null}
      <button
        type="button"
        className="text-muted-foreground flex h-6 w-5 shrink-0 cursor-grab touch-none items-center justify-center"
        aria-label={t("builder.reorder", { number })}
        {...attributes}
        {...listeners}
      >
        <GripVertical className="size-3.5 opacity-40" aria-hidden="true" />
      </button>

      <span className="w-4 shrink-0 tabular-nums">{number}</span>
      {loading ? null : (
        <Icon className="text-muted-foreground size-3.5 shrink-0" aria-hidden="true" />
      )}
      {question?.problem ? (
        <CircleAlert className="size-3.5 shrink-0" aria-hidden="true" />
      ) : null}

      <button
        type="button"
        className="flex-1 truncate text-left"
        disabled={loading}
        onClick={onSelect}
      >
        {loading ? (
          <span className="bg-muted inline-block h-3 w-full animate-pulse rounded" />
        ) : (
          (question.problem ?? titleOf(label, t))
        )}
      </button>

      <span className="text-muted-foreground text-caption shrink-0 tabular-nums group-focus-within/row:hidden group-hover/row:hidden">
        {loading ? "" : t("builder.points", { points: question.points })}
      </span>

      <span className="pointer-events-none flex w-0 shrink-0 overflow-hidden opacity-0 group-focus-within/row:pointer-events-auto group-focus-within/row:w-auto group-focus-within/row:overflow-visible group-focus-within/row:opacity-100 group-hover/row:pointer-events-auto group-hover/row:w-auto group-hover/row:overflow-visible group-hover/row:opacity-100 max-[767px]:pointer-events-auto max-[767px]:w-auto max-[767px]:overflow-visible max-[767px]:opacity-100">
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={t("builder.moveUp", { number })}
          disabled={!canUp}
          onClick={() => onStep(-1)}
        >
          <ChevronUp aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={t("builder.moveDown", { number })}
          disabled={!canDown}
          onClick={() => onStep(1)}
        >
          <ChevronDown aria-hidden="true" />
        </Button>
        <Tooltip
          label={`${t("builder.dropFromTest", { number })} · ${t("builder.staysInBank")}`}
        >
          <Button
            variant="ghost"
            size="icon-xs"
            data-drop=""
            aria-label={t("builder.dropFromTest", { number })}
            onClick={(event) => {
              const row = event.currentTarget.closest("[data-outline-row]");
              const neighbour = row?.nextElementSibling ?? row?.previousElementSibling;
              const target =
                neighbour?.querySelector<HTMLElement>("[data-drop]") ?? null;
              onDrop();
              requestAnimationFrame(() => target?.focus());
            }}
          >
            <X aria-hidden="true" />
          </Button>
        </Tooltip>
      </span>
    </div>
  );
}

function numberQuestions(
  sections: OutlineSection[],
  groups: Map<string, GroupBundle>,
): Map<string, number> {
  const numbers = new Map<string, number>();
  let n = 0;
  for (const section of sections) {
    for (const id of sectionQuestionIds(section, groups)) {
      n += 1;
      numbers.set(id, n);
    }
  }
  return numbers;
}

function sectionPoints(
  section: OutlineSection,
  questions: Map<string, OutlineQuestion>,
  groups: Map<string, GroupBundle>,
): number {
  return unitsOf(section).reduce(
    (sum, unit) =>
      sum +
      (unit.kind === "question"
        ? (questions.get(unit.id)?.points ?? 0)
        : (groups
            .get(unit.id)
            ?.questions.reduce(
              (points, question) => points + question.input.points,
              0,
            ) ?? 0)),
    0,
  );
}

function totalPoints(
  sections: OutlineSection[],
  questions: Map<string, OutlineQuestion>,
  groups: Map<string, GroupBundle>,
): number {
  return sections.reduce(
    (sum, section) => sum + sectionPoints(section, questions, groups),
    0,
  );
}

function clientKey(): string {
  return `new-${crypto.randomUUID()}`;
}

function toggle(set: ReadonlySet<string>, key: string): ReadonlySet<string> {
  const next = new Set(set);
  if (!next.delete(key)) next.add(key);
  return next;
}

function titleOf(label: string, t: TFunction): string {
  return label === "" ? t("builder.untitledQuestion") : label;
}

function abbreviatedTarget(
  sections: OutlineSection[],
  selectedGroupId: string | null | undefined,
  selectedId: string | null,
): string {
  const selected =
    sections.find((section) =>
      unitsOf(section).some((unit) =>
        unit.kind === "group" ? unit.id === selectedGroupId : unit.id === selectedId,
      ),
    ) ?? sections.at(-1);
  const title = selected?.title ?? "";
  return title.length > 16 ? `${title.slice(0, 15)}…` : title;
}

function dropPosition(
  target: { id: string; after: boolean } | null,
  id: string,
): DropPosition {
  if (target?.id !== id) return undefined;
  return target.after ? "after" : "before";
}

const outlineKeyboardCoordinates: KeyboardCoordinateGetter = (event, args) => {
  const { active, over, droppableContainers, droppableRects } = args.context;
  if (active?.data.current?.["kind"] !== "section")
    return sortableKeyboardCoordinates(event, args);
  if (event.code !== "ArrowUp" && event.code !== "ArrowDown") return;
  event.preventDefault();
  const current =
    over?.data.current?.["sectionIndex"] ?? active.data.current["sectionIndex"];
  if (typeof current !== "number") return;
  const next = current + (event.code === "ArrowDown" ? 1 : -1);
  const target = droppableContainers
    .getEnabled()
    .find(
      (container) =>
        container.data.current?.["kind"] === "section" &&
        container.data.current["sectionIndex"] === next,
    );
  const rect = target && droppableRects.get(target.id);
  if (rect) return { x: rect.left, y: rect.top };
};

function isAfterDrop(event: DragEndEvent): boolean {
  const rect = event.active.rect.current.translated;
  const over = event.over;
  return (
    rect !== null &&
    over !== null &&
    rect.top + rect.height / 2 > over.rect.top + over.rect.height / 2
  );
}
