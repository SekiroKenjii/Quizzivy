import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSortable } from "@dnd-kit/sortable";

import {
  ChevronDown,
  ChevronRight,
  ChevronUp,
  GripVertical,
  Layers,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { GroupBundle } from "@/features/question-groups/api";
import { QUESTION_TYPE_ICONS } from "@/features/question-bank/components/questionTypeIcons";

const ACTION =
  "text-muted-fg size-6 rounded-[6px] in-data-[scale=deck]:size-6 in-data-[scale=deck]:rounded-[6px] [&_svg:not([class*='size-'])]:size-3.5";

/**
 * OutlineGroupRow is a shared-context group in the builder's outline, drawn
 * in the outline's own rows: a header row with its grip, disclosure, title,
 * question count and points, and its move and remove actions, then its
 * members as numbered question rows with the deck's type icons and points,
 * indented under a rule. From 768px the actions always show; below it they
 * show at 44px only while the group or one of its members is selected, as a
 * question row's do (DG-135). It keeps the members together while selecting,
 * moving or removing it.
 */
export function OutlineGroupRow({
  id,
  group,
  numbering,
  drop,
  selected,
  selectedQuestionId,
  onSelect,
  onStep,
  onRemove,
}: Readonly<{
  id: string;
  group: GroupBundle | undefined;
  numbering: Map<string, number>;
  drop?: "before" | "after" | undefined;
  selected: boolean;
  selectedQuestionId: string | null;
  onSelect: (questionId?: string) => void;
  onStep: (direction: -1 | 1) => void;
  onRemove: () => void;
}>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(true);
  const { attributes, listeners, setNodeRef, isDragging } = useSortable({
    id: `group:${id}`,
    transition: null,
  });
  const title = group?.group.title || t("groups.newGroup");
  const questions = new Map(
    group?.questions.map((question) => [question.id, question.input]),
  );
  const members = group?.group.members ?? [];
  const points = members.reduce(
    (sum, member) => sum + (questions.get(member.questionId)?.points ?? 0),
    0,
  );
  const Chevron = open ? ChevronDown : ChevronRight;
  return (
    <div
      ref={setNodeRef}
      data-outline-group={id}
      className={cn("relative flex flex-col gap-0.5", isDragging && "opacity-40")}
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
      <div
        className={cn(
          "hover:bg-hover text-fg flex min-h-8 items-center gap-1 rounded-[7px] py-0.75 pr-1 pl-0.5 text-[13px]",
          selected && !selectedQuestionId && "bg-secondary font-medium",
        )}
      >
        <button
          type="button"
          aria-label={t("builder.reorderGroup", { title })}
          {...attributes}
          {...listeners}
          className="text-muted-fg flex h-6 w-5 shrink-0 cursor-grab touch-none items-center justify-center"
        >
          <GripVertical className="size-3.5 opacity-40" aria-hidden="true" />
        </button>
        <button
          type="button"
          aria-label={title}
          aria-expanded={open}
          className="text-muted-fg grid size-5 shrink-0 place-items-center rounded-[5px]"
          onClick={() => setOpen(!open)}
        >
          <Chevron className="size-3.5" aria-hidden="true" />
        </button>
        <Layers className="text-muted-fg size-3.5 shrink-0" aria-hidden="true" />
        <button
          type="button"
          className="min-w-0 flex-1 truncate px-1 py-0.75 text-left leading-4 font-medium"
          disabled={!group}
          onClick={() => onSelect()}
        >
          {group ? title : <Skeleton className="h-3 w-full" />}
        </button>
        {group ? (
          <span
            className={cn(
              "text-muted-fg shrink-0 text-xs whitespace-nowrap tabular-nums",
              selected && "max-[767px]:hidden",
            )}
          >
            {t("builder.sectionSummary", { questions: members.length, points })}
          </span>
        ) : null}
        <span
          className={cn(
            "flex shrink-0",
            selected ? "max-[767px]:[&_button]:size-11" : "max-[767px]:hidden",
          )}
        >
          <Button
            variant="ghost"
            size="icon-xs"
            className={ACTION}
            aria-label={t("builder.moveGroupUp", { title })}
            onClick={() => onStep(-1)}
          >
            <ChevronUp aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon-xs"
            className={ACTION}
            aria-label={t("builder.moveGroupDown", { title })}
            onClick={() => onStep(1)}
          >
            <ChevronDown aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon-xs"
            className={cn(ACTION, "hover:text-danger-ink")}
            aria-label={t("builder.removeGroup", { title })}
            onClick={onRemove}
          >
            <Trash2 aria-hidden="true" />
          </Button>
        </span>
      </div>
      {open && group ? (
        <div className="ml-4.5 flex flex-col gap-0.5 border-l pl-1.5">
          {members.map((member) => {
            const input = questions.get(member.questionId);
            const Icon = QUESTION_TYPE_ICONS[input?.type ?? "single_choice"];
            const current = selected && selectedQuestionId === member.questionId;
            return (
              <button
                key={member.questionId}
                type="button"
                className={cn(
                  "hover:bg-hover text-fg flex min-h-8 min-w-0 items-center gap-1.5 rounded-[7px] py-0.75 pr-1 pl-1.5 text-left text-[13px]",
                  current && "bg-secondary font-medium",
                )}
                onClick={() => onSelect(member.questionId)}
              >
                <span className="text-muted-fg w-4.5 shrink-0 text-xs tabular-nums">
                  {numbering.get(member.questionId)}
                </span>
                <Icon className="text-muted-fg size-3.5 shrink-0" aria-hidden="true" />
                <span className="min-w-0 flex-1 truncate leading-4">
                  {input?.prompt || t("builder.untitledQuestion")}
                </span>
                {input ? (
                  <span className="text-muted-fg shrink-0 pr-1 text-xs whitespace-nowrap tabular-nums">
                    {t("builder.points", { count: input.points })}
                  </span>
                ) : null}
              </button>
            );
          })}
          {members.length === 0 ? (
            <p className="text-muted-fg px-1.5 py-1.5 text-xs leading-normal">
              {t("builder.emptySharedGroup")}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
