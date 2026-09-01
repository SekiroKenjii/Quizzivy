import { useState } from "react";
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
  GripVertical,
  Headphones,
  Library,
  Plus,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  findQuestion,
  moveQuestion,
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

  function step(questionId: string, direction: -1 | 1) {
    const from = findQuestion(sections, questionId);
    if (!from) return;
    const to = stepQuestion(sections, from, direction);
    if (!to) return;
    onChange(moveQuestion(sections, from, to));
  }

  const numbering = numberQuestions(sections);

  // The count comes from local state and is complete immediately; the points
  // come from per-question queries and fill in one at a time. Showing both
  // together rendered "6 câu · 0 điểm", then 2, then 6 -- a number that was
  // never true. The count alone is honest until the rest lands.
  const settled = [...numbering.keys()].every((id) => questions.has(id));

  return (
    <div className="flex h-full w-72 shrink-0 flex-col overflow-hidden border-r">
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
            const key = section.id ?? `new-${sectionIndex}`;
            const open = !collapsed.has(key);
            return (
              <div key={key}>
                <button
                  type="button"
                  className="flex w-full items-center gap-1.5 px-1.5 py-1 text-left"
                  aria-expanded={open}
                  onClick={() => setCollapsed(toggle(collapsed, key))}
                >
                  {open ? (
                    <ChevronDown
                      className="text-muted-foreground size-3.5"
                      aria-hidden="true"
                    />
                  ) : (
                    <ChevronRight
                      className="text-muted-foreground size-3.5"
                      aria-hidden="true"
                    />
                  )}
                  <span className="truncate text-xs font-semibold">
                    {section.title}
                  </span>
                  <span className="text-muted-foreground ml-auto text-xs tabular-nums">
                    {settled
                      ? t("builder.sectionSummary", {
                          questions: section.questionIds.length,
                          points: sectionPoints(section, questions),
                        })
                      : section.questionIds.length}
                  </span>
                </button>

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
    </div>
  );
}

function OutlineRow({
  number,
  question,
  questionId,
  selected,
  onSelect,
  onStep,
}: {
  number: number;
  question: OutlineQuestion | undefined;
  questionId: string;
  selected: boolean;
  onSelect: () => void;
  onStep: (direction: -1 | 1) => void;
}) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: questionId });

  // `undefined` is "the query has not resolved", which is not the same claim as
  // "this question has no text". Collapsing them made a six-question outline
  // read as six empty questions for the length of six in-flight requests.
  const loading = question === undefined;
  const label = question?.prompt.trim() ?? "";

  return (
    <div
      ref={setNodeRef}
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

      {/* The keyboard path, per §14. dnd-kit's keyboard sensor drives the same
          move; these make it discoverable without knowing to press space. */}
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

function toggle(set: ReadonlySet<string>, key: string): ReadonlySet<string> {
  const next = new Set(set);
  if (!next.delete(key)) next.add(key);
  return next;
}
