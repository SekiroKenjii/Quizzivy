import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  ChevronDown,
  ChevronRight,
  ChevronUp,
  CircleAlert,
  Ellipsis,
  GripVertical,
  Headphones,
  Library,
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
import { SideColumn } from "@/components/shared/SideColumn";
import { cn } from "@/lib/utils";
import {
  findQuestion,
  moveQuestion,
  removeQuestion,
  moveSection,
  stepQuestion,
  type OutlineSection,
} from "@/features/tests/outline";

export interface OutlineQuestion {
  id: string;
  prompt: string;
  points: number;
  hasAudio: boolean;
  /** A publish violation anchored here, so the outline carries validity (A-04). */
  problem: string | null;
}

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
}

/**
 * The deck's A-04 outline: what is in this test, in reading order.
 *
 * It carries validity as well as structure — a question with a publish problem
 * is red here, while the teacher is authoring, so publish confirms rather than
 * surprises.
 */
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
}: OutlineTreeProps) {
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
    section.id ?? clientKeys[index] ?? `new-${index}`;
  const [renaming, setRenaming] = useState<number | null>(null);
  const [instructing, setInstructing] = useState<number | null>(null);
  const [removing, setRemoving] = useState<number | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  function onDragEnd(event: DragEndEvent) {
    const activeId = String(event.active.id);
    const overId = event.over ? String(event.over.id) : null;
    if (overId === null || overId === activeId) return;

    const from = findQuestion(sections, activeId);
    const to = findQuestion(sections, overId);
    if (!from || !to) return;
    onChange(moveQuestion(sections, from, to));
  }

  function drop(questionId: string) {
    const at = findQuestion(sections, questionId);
    if (at !== null) onChange(removeQuestion(sections, at));
  }

  function step(questionId: string, direction: -1 | 1) {
    const from = findQuestion(sections, questionId);
    if (!from) return;
    const to = stepQuestion(sections, from, direction);
    if (!to) return;
    onChange(moveQuestion(sections, from, to));
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

  const numbering = numberQuestions(sections);

  const settled = [...numbering.keys()].every((id) => questions.has(id));

  const editing = instructing === null ? null : (sections[instructing] ?? null);
  const doomed = removing === null ? null : (sections[removing] ?? null);

  return (
    <SideColumn
      as="div"
      column="outline"
      side="left"
      className="flex h-full flex-col overflow-hidden border-r"
    >
      <div className="flex shrink-0 items-center gap-2 border-b p-3">
        <p className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
          {t("builder.outline")}
        </p>
        <p className="text-muted-foreground ml-auto text-xs tabular-nums">
          {settled
            ? t("builder.outlineSummary", {
                questions: numbering.size,
                points: totalPoints(sections, questions),
              })
            : t("builder.outlineQuestionsOnly", { questions: numbering.size })}
        </p>
      </div>

      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={onDragEnd}
      >
        <div className="flex-1 space-y-3 overflow-y-auto p-2">
          {sections.map((section, sectionIndex) => {
            const key = keyFor(section, sectionIndex);
            const open = !collapsed.has(key);
            return (
              <div key={key}>
                <SectionHeader
                  title={section.title}
                  summary={
                    settled
                      ? t("builder.sectionSummary", {
                          questions: section.questionIds.length,
                          points: sectionPoints(section, questions),
                        })
                      : String(section.questionIds.length)
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
                  onRemove={() =>
                    section.questionIds.length === 0
                      ? remove(sectionIndex)
                      : setRemoving(sectionIndex)
                  }
                />

                {open ? (
                  <SortableContext
                    items={section.questionIds}
                    strategy={verticalListSortingStrategy}
                  >
                    <div className="space-y-0.5 pl-1">
                      {section.questionIds.map((questionId) => (
                        <OutlineRow
                          key={questionId}
                          number={numbering.get(questionId) ?? 0}
                          question={questions.get(questionId)}
                          questionId={questionId}
                          selected={questionId === selectedId}
                          onSelect={() => onSelect(questionId)}
                          onStep={(direction) => step(questionId, direction)}
                          onDrop={() => drop(questionId)}
                        />
                      ))}
                      {section.questionIds.length === 0 ? (
                        <p className="text-muted-foreground px-2 py-1.5 text-xs">
                          {t("builder.sectionEmpty")}
                        </p>
                      ) : null}
                    </div>
                  </SortableContext>
                ) : null}
              </div>
            );
          })}
        </div>
      </DndContext>

      <div className="shrink-0 space-y-1 border-t p-2">
        <Button
          variant="outline"
          size="sm"
          className="w-full justify-start"
          disabled={creating || sections.length === 0}
          onClick={onCreateQuestion}
        >
          <Plus aria-hidden="true" />
          {t("builder.addQuestion")}
        </Button>
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
          count: doomed?.questionIds.length ?? 0,
        })}
        confirmLabel={t("builder.removeSection")}
        destructive
        onConfirm={() => removing !== null && remove(removing)}
      />
    </SideColumn>
  );
}

/** The key caps A-04a prints under the rename field; not translated — they are the keys. */
const KEY = { enter: "↵", escape: "esc" } as const;

function SectionHeader({
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
}: {
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
}) {
  const { t } = useTranslation();
  const trigger = useRef<HTMLButtonElement>(null);
  const wasRenaming = useRef(renaming);
  const Chevron = open ? ChevronDown : ChevronRight;

  useEffect(() => {
    if (wasRenaming.current && !renaming && document.activeElement === document.body) {
      trigger.current?.focus();
    }
    wasRenaming.current = renaming;
  }, [renaming]);

  if (renaming) {
    return (
      <div className="flex items-start gap-1.5 px-1.5 py-1">
        <Chevron
          className="text-muted-foreground mt-1.5 size-3.5 shrink-0"
          aria-hidden="true"
        />
        <SectionTitleInput title={title} onDone={onRenamed} />
      </div>
    );
  }

  return (
    <div className="group has-data-[state=open]:bg-accent flex items-center gap-1.5 rounded-md px-1.5 py-1">
      <button
        type="button"
        className="flex min-w-0 flex-1 items-center gap-1.5 text-left"
        aria-expanded={open}
        onClick={onToggle}
      >
        <Chevron
          className="text-muted-foreground size-3.5 shrink-0"
          aria-hidden="true"
        />
        <span className="truncate text-xs font-semibold">{title}</span>
        <span className="text-muted-foreground ml-auto text-xs tabular-nums">
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
            className="text-muted-foreground shrink-0 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 data-[state=open]:opacity-100"
          >
            <Ellipsis aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
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
  );
}

function SectionTitleInput({
  title,
  onDone,
}: {
  title: string;
  onDone: (title: string | null) => void;
}) {
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
}: {
  sectionTitle: string;
  instructions: string;
  onCancel: () => void;
  onSave: (instructions: string) => void;
}) {
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
  selected,
  onSelect,
  onStep,
  onDrop,
}: {
  number: number;
  question: OutlineQuestion | undefined;
  questionId: string;
  selected: boolean;
  onSelect: () => void;
  onStep: (direction: -1 | 1) => void;
  onDrop: () => void;
}) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: questionId });

  const loading = question === undefined;
  const label = question?.prompt.trim() ?? "";

  return (
    <div
      ref={setNodeRef}
      data-outline-row=""
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        "group flex items-center gap-2 rounded-md px-2 py-1.5 text-xs",
        selected ? "bg-secondary text-foreground font-medium" : "text-muted-foreground",
        question?.problem ? "text-destructive-ink" : "",
        isDragging ? "opacity-60" : "",
      )}
    >
      <button
        type="button"
        className="cursor-grab touch-none"
        aria-label={t("builder.reorder", { number })}
        {...attributes}
        {...listeners}
      >
        <GripVertical className="size-3.5 opacity-40" aria-hidden="true" />
      </button>

      <span className="w-4 shrink-0 tabular-nums">{number}</span>
      {question?.hasAudio ? (
        <Headphones className="size-3.5 shrink-0" aria-hidden="true" />
      ) : null}
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
          (question.problem ?? (label === "" ? t("builder.untitledQuestion") : label))
        )}
      </button>

      <span className="shrink-0 tabular-nums">
        {loading ? "" : t("builder.points", { points: question.points })}
      </span>

      {/* The keyboard path, per §14. */}
      <span className="flex shrink-0 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100">
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={t("builder.moveUp", { number })}
          onClick={() => onStep(-1)}
        >
          <ChevronUp aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={t("builder.moveDown", { number })}
          onClick={() => onStep(1)}
        >
          <ChevronDown aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          data-drop=""
          aria-label={t("builder.dropFromTest", { number })}
          onClick={(event) => {
            const row = event.currentTarget.closest("[data-outline-row]");
            const neighbour = row?.nextElementSibling ?? row?.previousElementSibling;
            const target = neighbour?.querySelector<HTMLElement>("[data-drop]") ?? null;
            onDrop();
            requestAnimationFrame(() => target?.focus());
          }}
        >
          <X aria-hidden="true" />
        </Button>
      </span>
    </div>
  );
}

/** Numbering runs across sections, which is how a student counts them. */
function numberQuestions(sections: OutlineSection[]): Map<string, number> {
  const numbers = new Map<string, number>();
  let n = 0;
  for (const section of sections) {
    for (const id of section.questionIds) {
      n += 1;
      numbers.set(id, n);
    }
  }
  return numbers;
}

function sectionPoints(
  section: OutlineSection,
  questions: Map<string, OutlineQuestion>,
): number {
  return section.questionIds.reduce(
    (sum, id) => sum + (questions.get(id)?.points ?? 0),
    0,
  );
}

function totalPoints(
  sections: OutlineSection[],
  questions: Map<string, OutlineQuestion>,
): number {
  return sections.reduce((sum, section) => sum + sectionPoints(section, questions), 0);
}

function clientKey(): string {
  return `new-${Math.random().toString(36).slice(2)}`;
}

function toggle(set: ReadonlySet<string>, key: string): ReadonlySet<string> {
  const next = new Set(set);
  if (!next.delete(key)) next.add(key);
  return next;
}
