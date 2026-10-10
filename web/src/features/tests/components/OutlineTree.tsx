import { Tooltip } from "@/components/shared/Tooltip";
import { toast } from "@/components/ui/sonner";
import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import {
  DndContext,
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
import { OutlineKeyboardSensor } from "./OutlineKeyboardSensor";
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
  onPickFromBank: (opener: HTMLElement) => void;
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
  const [renaming, setRenaming] = useState<string | null>(null);
  const [renameNew, setRenameNew] = useState<number | null>(null);
  if (renameNew !== null && sections.length > renameNew) {
    setRenameNew(null);
    setRenaming(keyFor(sections[renameNew]!, renameNew));
  }
  const [instructing, setInstructing] = useState<number | null>(null);
  const instructionsOpener = useRef<HTMLElement | null>(null);
  const openInstructions = (sectionIndex: number, opener: HTMLElement | null) => {
    instructionsOpener.current = opener;
    setInstructing(sectionIndex);
  };
  const [removing, setRemoving] = useState<number | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<{ id: string; after: boolean } | null>(
    null,
  );
  const keyboardDisposers = useRef(new Set<() => void>());
  const registerKeyboard = useCallback((cancel: () => void) => {
    keyboardDisposers.current.add(cancel);
    return () => {
      keyboardDisposers.current.delete(cancel);
    };
  }, []);
  useEffect(() => {
    const disposers = keyboardDisposers.current;
    return () => {
      for (const cancel of [...disposers]) cancel();
    };
  }, []);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(OutlineKeyboardSensor, {
      coordinateGetter: outlineKeyboardCoordinates,
      register: registerKeyboard,
    }),
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
    toast(t("builder.removedFromTest"));
  }

  function step(questionId: string, direction: -1 | 1) {
    const from = findUnit(sections, questionId);
    if (!from) return;
    const to = stepUnit(sections, from, direction);
    if (!to) return;
    onChange(moveUnit(sections, from, to));
  }

  function rename(key: string, title: string | null) {
    setRenaming(null);
    const index = sections.findIndex((section, i) => keyFor(section, i) === key);
    if (index < 0 || title === null || title === sections[index]!.title) return;
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
        <p className="text-[13px] font-semibold">{t("builder.outline")}</p>
        <p className="text-muted-foreground ml-auto shrink-0 text-xs leading-[18px] tabular-nums">
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
        <div className="min-h-0 flex-1 space-y-1.5 overflow-y-auto p-1.5">
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
                  className={cn(
                    "pb-0.5",
                    dragging === `section:${key}` && "opacity-45",
                  )}
                >
                  <SectionHeader
                    id={`section:${key}`}
                    sectionIndex={sectionIndex}
                    dragging={dragging !== null}
                    drop={
                      dropTarget?.id === `head:section:${key}`
                        ? undefined
                        : dropPosition(dropTarget, `section:${key}`)
                    }
                    dropInto={dropTarget?.id === `head:section:${key}`}
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
                    renaming={renaming === key}
                    first={sectionIndex === 0}
                    last={sectionIndex === sections.length - 1}
                    onToggle={() => setCollapsed((current) => toggle(current, key))}
                    onStartRename={() => setRenaming(key)}
                    onRenamed={(next) => rename(key, next)}
                    onInstructions={(opener) => openInstructions(sectionIndex, opener)}
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
                        className="text-muted-fg hover:bg-muted mb-1 ml-6.5 flex w-[calc(100%-1.625rem)] items-center gap-1.5 rounded-[6px] px-1.5 py-1 text-left text-xs leading-[1.45]"
                        onClick={(event) =>
                          openInstructions(sectionIndex, event.currentTarget)
                        }
                      >
                        <ScrollText aria-hidden="true" className="size-3.25 shrink-0" />
                        <span className="min-w-0 truncate leading-[18px]">
                          {section.instructions}
                        </span>
                      </button>
                    ) : null}
                    {open ? (
                      <SortableContext
                        items={unitsOf(section).map(unitKey)}
                        strategy={verticalListSortingStrategy}
                      >
                        <div className="ml-3.5 space-y-px border-l pl-1">
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

      <div className="flex shrink-0 flex-col gap-0.5 border-t p-2">
        <Button
          variant="outline"
          size="sm"
          className="shadow-card w-full justify-start gap-2 px-2.5 in-data-[scale=deck]:h-8 in-data-[scale=deck]:gap-2 in-data-[scale=deck]:px-2.5"
          disabled={creating || sections.length === 0}
          onClick={onCreateQuestion}
        >
          <Plus aria-hidden="true" />
          <span className="flex-1 text-left whitespace-nowrap">
            {t(
              selectedGroupId ? "builder.addStandaloneQuestion" : "builder.addQuestion",
            )}
          </span>{" "}
          {sections.length === 0 ? null : (
            <span className="text-muted-foreground min-w-0 truncate text-[11.5px] font-normal">
              {t("builder.addQuestionTo", {
                title: abbreviatedTarget(sections, selectedGroupId, selectedId),
              })}
            </span>
          )}
        </Button>
        {onCreateGroup ? (
          <Button
            variant="outline"
            size="sm"
            className="shadow-card w-full justify-start gap-2 px-2.5 in-data-[scale=deck]:h-8 in-data-[scale=deck]:gap-2 in-data-[scale=deck]:px-2.5"
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
          className="text-muted-foreground w-full justify-start gap-2 px-2.5 in-data-[scale=deck]:h-8 in-data-[scale=deck]:gap-2 in-data-[scale=deck]:px-2.5"
          disabled={sections.length === 0}
          onClick={(event) => onPickFromBank(event.currentTarget)}
        >
          <Library aria-hidden="true" />
          {t("builder.fromQuestionBank")}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground w-full justify-start gap-2 px-2.5 in-data-[scale=deck]:h-8 in-data-[scale=deck]:gap-2 in-data-[scale=deck]:px-2.5"
          onClick={() => {
            onAddSection();
            setRenameNew(sections.length);
          }}
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
          returnFocus={instructionsOpener}
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
  dropInto,
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
  dropInto: boolean;
  title: string;
  summary: string;
  open: boolean;
  renaming: boolean;
  first: boolean;
  last: boolean;
  onToggle: () => void;
  onStartRename: () => void;
  onRenamed: (title: string | null) => void;
  onInstructions: (opener: HTMLElement | null) => void;
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
  const { setNodeRef: setHeaderNodeRef } = useDroppable({
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
          data-outline-drop-into={dropInto ? "" : undefined}
          className={cn(
            "group/section relative flex items-center gap-0.5 rounded-[7px] py-0.5 pr-1 pl-0.5 transition-colors duration-120 ease-[cubic-bezier(.25,.1,.25,1)] motion-reduce:transition-none",
            dropInto
              ? "bg-brand-soft"
              : "hover:bg-hover has-data-[state=open]:bg-hover",
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
            className="group/section-title flex min-w-0 flex-1 items-center gap-[5px] px-0.5 py-1 text-left leading-4"
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
                "text-meta min-w-0 flex-1 leading-4 font-semibold group-focus-within/section:[&_.qz-marquee-track]:[animation-play-state:paused]! group-hover/section:[&_.qz-marquee-track]:[animation-play-state:paused]!",
                dragging && "[&_.qz-marquee-track]:[animation:none]!",
              )}
            />{" "}
            <span className="text-muted-foreground ml-auto shrink-0 pl-1.5 text-xs leading-[15px] font-medium tabular-nums">
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
              <DropdownMenuItem onSelect={() => onInstructions(trigger.current)}>
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
        "text-muted-foreground mx-1 mt-0.5 mb-1 flex min-h-11 items-center rounded-[7px] border border-dashed px-2.5 text-xs",
        isOver && "bg-brand-soft border-foreground",
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
  const pressing = useRef(false);
  const deferred = useRef<(() => void) | null>(null);
  const done = useRef(onDone);

  useEffect(() => {
    done.current = onDone;
  });

  useEffect(() => {
    field.current?.focus();
    field.current?.select();
    let timer: number | undefined;
    const press = () => {
      pressing.current = true;
    };
    const release = () => {
      window.clearTimeout(timer);
      pressing.current = false;
      const commit = deferred.current;
      deferred.current = null;
      commit?.();
    };
    const releaseSoon = () => {
      timer = window.setTimeout(release, 0);
    };
    document.addEventListener("pointerdown", press, true);
    window.addEventListener("pointerup", releaseSoon);
    window.addEventListener("click", release, true);
    window.addEventListener("pointercancel", release);
    return () => {
      document.removeEventListener("pointerdown", press, true);
      window.removeEventListener("pointerup", releaseSoon);
      window.removeEventListener("click", release, true);
      window.removeEventListener("pointercancel", release);
      release();
    };
  }, []);

  function finish(next: string | null) {
    if (settled.current) return;
    settled.current = true;
    done.current(next);
  }

  function blur() {
    const next = value.trim() === "" ? null : value.trim();
    if (pressing.current) deferred.current = () => finish(next);
    else finish(next);
  }

  return (
    <div className="min-w-0 flex-1">
      <Input
        ref={field}
        value={value}
        aria-label={t("builder.sectionNameLabel")}
        className="h-7 px-2 text-xs font-semibold"
        onChange={(event) => setValue(event.target.value)}
        onBlur={blur}
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
  returnFocus,
  onCancel,
  onSave,
}: Readonly<{
  sectionTitle: string;
  instructions: string;
  returnFocus: RefObject<HTMLElement | null>;
  onCancel: () => void;
  onSave: (instructions: string) => void;
}>) {
  const { t } = useTranslation();
  const [value, setValue] = useState(instructions);

  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => !open && onCancel()}
      returnFocus={returnFocus}
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
  const problemId = useId();
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
        "group/row hover:bg-hover text-fg relative flex min-h-8 items-center gap-1 rounded-[7px] py-0.75 pr-1 pl-0.5 text-[13px]",
        selected && "bg-secondary font-medium",
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

      <span className="text-muted-foreground w-4.5 shrink-0 text-xs tabular-nums">
        {number}
      </span>
      {loading ? null : (
        <Icon className="text-muted-foreground size-3.5 shrink-0" aria-hidden="true" />
      )}

      <button
        type="button"
        className="min-w-0 flex-1 truncate px-1 py-0.75 text-left leading-4"
        disabled={loading}
        aria-describedby={question?.problem ? problemId : undefined}
        onClick={onSelect}
      >
        {loading ? (
          <span className="bg-muted inline-block h-3 w-full animate-pulse rounded" />
        ) : (
          titleOf(label, t)
        )}
      </button>
      {question?.problem ? (
        <>
          <CircleAlert className="text-warning size-3.5 shrink-0" aria-hidden="true">
            <title>{question.problem}</title>
          </CircleAlert>
          <span id={problemId} className="sr-only">
            {question.problem}
          </span>
        </>
      ) : null}

      <span
        className={cn(
          "text-muted-foreground shrink-0 pr-1 text-xs whitespace-nowrap tabular-nums min-[768px]:group-hover/row:hidden min-[768px]:group-has-[:focus-visible]/row:hidden",
          selected && "max-[767px]:hidden",
        )}
      >
        {loading ? "" : t("builder.points", { count: question.points })}
      </span>

      <span
        className={cn(
          "pointer-events-none flex w-0 shrink-0 overflow-hidden opacity-0 min-[768px]:group-hover/row:pointer-events-auto min-[768px]:group-hover/row:w-auto min-[768px]:group-hover/row:overflow-visible min-[768px]:group-hover/row:opacity-100 min-[768px]:group-has-[:focus-visible]/row:pointer-events-auto min-[768px]:group-has-[:focus-visible]/row:w-auto min-[768px]:group-has-[:focus-visible]/row:overflow-visible min-[768px]:group-has-[:focus-visible]/row:opacity-100",
          selected &&
            "max-[767px]:pointer-events-auto max-[767px]:w-auto max-[767px]:overflow-visible max-[767px]:opacity-100 max-[767px]:[&_button]:size-11",
        )}
      >
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
  const { active, over, droppableContainers, droppableRects, collisionRect } =
    args.context;
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
  if (rect && collisionRect)
    return {
      x: rect.left + (rect.width - collisionRect.width) / 2,
      y: rect.top + (rect.height - collisionRect.height) / 2,
    };
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
