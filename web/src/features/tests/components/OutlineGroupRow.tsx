import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
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

export function OutlineGroupRow({
  id,
  group,
  numbering,
  selected,
  selectedQuestionId,
  onSelect,
  onStep,
  onRemove,
}: Readonly<{
  id: string;
  group: GroupBundle | undefined;
  numbering: Map<string, number>;
  selected: boolean;
  selectedQuestionId: string | null;
  onSelect: (questionId?: string) => void;
  onStep: (direction: -1 | 1) => void;
  onRemove: () => void;
}>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(true);
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: `group:${id}` });
  const title = group?.group.title || t("groups.newGroup");
  const questions = new Map(
    group?.questions.map((question) => [question.id, question.input]),
  );
  const Chevron = open ? ChevronDown : ChevronRight;
  return (
    <div
      ref={setNodeRef}
      data-outline-group={id}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn("rounded-md border", isDragging && "opacity-60")}
    >
      <div
        className={cn(
          "flex items-center gap-1 p-1",
          selected && !selectedQuestionId && "bg-secondary",
        )}
      >
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={t("builder.reorderGroup", { title })}
          {...attributes}
          {...listeners}
          className="cursor-grab touch-none"
        >
          <GripVertical />
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={title}
          aria-expanded={open}
          onClick={() => setOpen(!open)}
        >
          <Chevron />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="min-w-0 flex-1 justify-start"
          onClick={() => onSelect()}
          disabled={!group}
        >
          <Layers />
          {group ? (
            <span className="truncate">{title}</span>
          ) : (
            <Skeleton className="h-3 w-full" />
          )}
        </Button>
      </div>
      {open && group ? (
        <div className="flex flex-col gap-1 px-2 pb-2">
          {group.group.members.map((member) => (
            <Button
              key={member.questionId}
              variant={
                selected && selectedQuestionId === member.questionId
                  ? "secondary"
                  : "ghost"
              }
              size="sm"
              className="min-w-0 justify-start"
              onClick={() => onSelect(member.questionId)}
            >
              <span className="text-muted-foreground tabular-nums">
                {numbering.get(member.questionId)}
              </span>
              <span className="truncate">
                {questions.get(member.questionId)?.prompt ||
                  t("builder.untitledQuestion")}
              </span>
            </Button>
          ))}
          {group.group.members.length === 0 ? (
            <p className="text-muted-foreground p-2 text-xs">
              {t("builder.emptySharedGroup")}
            </p>
          ) : null}
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={t("builder.moveGroupUp", { title })}
              onClick={() => onStep(-1)}
            >
              <ChevronUp />
            </Button>
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={t("builder.moveGroupDown", { title })}
              onClick={() => onStep(1)}
            >
              <ChevronDown />
            </Button>
            <Button
              variant="ghost"
              size="icon-xs"
              className="ml-auto"
              aria-label={t("builder.removeGroup", { title })}
              onClick={onRemove}
            >
              <Trash2 />
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
