import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSortable } from "@dnd-kit/sortable";

import {
  AudioLines,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  Ellipsis,
  FileText,
  GripVertical,
  Layers,
  PanelLeft,
  Pencil,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { GroupBundle } from "@/features/question-groups/api";
import { QUESTION_TYPE_ICONS } from "@/features/question-bank/components/questionTypeIcons";
import { sharedContentLine, type SharedContentLine } from "../sharedContentLine";

const SHARED_ICONS = {
  passage: FileText,
  audio: AudioLines,
  materials: FileText,
} as const;

/**
 * OutlineGroupRow is a shared-context group in the builder's outline, drawn
 * in the outline's own rows: a header row with its grip, disclosure, title,
 * question count and points, and its "…" menu (Rename, Shared content, Move
 * up, Move down, Remove group); while open, the shared-content line under
 * it and its members as numbered question rows with the deck's type icons
 * and points, indented under a rule. From 768px the menu's trigger shows on
 * hover, focus and while open, as a section's does; below it the trigger
 * shows at 44px only while the group or one of its members is selected, as
 * a question row's actions do (DG-135). It keeps the members together while
 * selecting, moving or removing it. `fileNames` maps an audio asset id to
 * the file name the line prints. Rename and Shared content only ask the
 * builder to act: the group's title is written by its editor, never here.
 * Rename and Shared content ask once the menu has closed, so the focus they
 * move is not taken back by the menu; Shared content leaves focus on the
 * trigger until its dialog opens. Shared content, from the menu or from the
 * line, and Remove group hand over the control they were asked from, the
 * menu's trigger or the line, where focus returns when their dialog closes.
 */
export function OutlineGroupRow({
  id,
  group,
  numbering,
  fileNames,
  drop,
  selected,
  selectedQuestionId,
  canUp,
  canDown,
  onSelect,
  onRename,
  onSharedContent,
  onStep,
  onRemove,
}: Readonly<{
  id: string;
  group: GroupBundle | undefined;
  numbering: Map<string, number>;
  fileNames: ReadonlyMap<string, string>;
  drop?: "before" | "after" | undefined;
  selected: boolean;
  selectedQuestionId: string | null;
  canUp: boolean;
  canDown: boolean;
  onSelect: (questionId?: string) => void;
  onRename: () => void;
  onSharedContent: (opener: HTMLElement | null) => void;
  onStep: (direction: -1 | 1) => void;
  onRemove: (opener: HTMLElement | null) => void;
}>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(true);
  const { attributes, listeners, setNodeRef, isDragging } = useSortable({
    id: `group:${id}`,
    transition: null,
  });
  const title = group?.group.title || t("groups.newGroup");
  const shared = group ? sharedContentLine(group, fileNames, t) : null;
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
      <div>
        <div
          className={cn(
            "group/unit hover:bg-hover has-data-[state=open]:bg-hover text-fg flex min-h-8 items-center gap-1 rounded-[7px] py-0.75 pr-1 pl-0.5 text-[13px]",
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
          <GroupHeading
            group={group}
            title={title}
            selected={selected}
            onSelect={() => onSelect()}
          />
          <GroupActions
            title={title}
            disabled={!group}
            selected={selected}
            canUp={canUp}
            canDown={canDown}
            onRename={onRename}
            onSharedContent={onSharedContent}
            onStep={onStep}
            onRemove={onRemove}
          />
        </div>
        {open && shared ? (
          <SharedContentButton line={shared} onClick={onSharedContent} />
        ) : null}
      </div>
      {open && group ? (
        <GroupMembers
          group={group}
          numbering={numbering}
          selectedQuestionId={selected ? selectedQuestionId : null}
          onSelect={onSelect}
        />
      ) : null}
    </div>
  );
}

function GroupActions({
  title,
  disabled,
  selected,
  canUp,
  canDown,
  onRename,
  onSharedContent,
  onStep,
  onRemove,
}: Readonly<{
  title: string;
  disabled: boolean;
  selected: boolean;
  canUp: boolean;
  canDown: boolean;
  onRename: () => void;
  onSharedContent: (opener: HTMLElement | null) => void;
  onStep: (direction: -1 | 1) => void;
  onRemove: (opener: HTMLElement | null) => void;
}>) {
  const { t } = useTranslation();
  const trigger = useRef<HTMLButtonElement>(null);
  const afterClose = useRef<{ run: () => void; focusTrigger: boolean } | null>(null);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          ref={trigger}
          variant="ghost"
          size="icon-xs"
          aria-label={t("builder.groupActions", { title })}
          disabled={disabled}
          className={cn(
            "text-muted-fg shrink-0 rounded-[6px] min-[768px]:opacity-0 min-[768px]:group-focus-within/unit:opacity-100 min-[768px]:group-hover/unit:opacity-100 min-[768px]:data-[state=open]:opacity-100",
            selected
              ? "max-[767px]:size-11 max-[767px]:in-data-[scale=deck]:size-11"
              : "max-[767px]:hidden",
          )}
        >
          <Ellipsis aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="w-48 data-[state=closed]:animate-none! data-[state=open]:animate-none!"
        onCloseAutoFocus={(event) => {
          const next = afterClose.current;
          afterClose.current = null;
          if (!next) return;
          if (!next.focusTrigger) event.preventDefault();
          next.run();
        }}
      >
        <DropdownMenuItem
          onSelect={() => {
            afterClose.current = { run: onRename, focusTrigger: false };
          }}
        >
          <Pencil aria-hidden="true" />
          {t("builder.renameGroup")}
        </DropdownMenuItem>
        <DropdownMenuItem
          onSelect={() => {
            afterClose.current = {
              run: () => onSharedContent(trigger.current),
              focusTrigger: true,
            };
          }}
        >
          <PanelLeft aria-hidden="true" />
          {t("builder.sharedContent")}
        </DropdownMenuItem>
        <DropdownMenuItem disabled={!canUp} onSelect={() => onStep(-1)}>
          <ChevronUp aria-hidden="true" />
          {t("builder.groupMoveUp")}
        </DropdownMenuItem>
        <DropdownMenuItem disabled={!canDown} onSelect={() => onStep(1)}>
          <ChevronDown aria-hidden="true" />
          {t("builder.groupMoveDown")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          onSelect={() => onRemove(trigger.current)}
        >
          <Trash2 aria-hidden="true" />
          {t("builder.groupRemove")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function GroupMembers({
  group,
  numbering,
  selectedQuestionId,
  onSelect,
}: Readonly<{
  group: GroupBundle;
  numbering: Map<string, number>;
  selectedQuestionId: string | null;
  onSelect: (questionId: string) => void;
}>) {
  const { t } = useTranslation();
  const questions = new Map(
    group.questions.map((question) => [question.id, question.input]),
  );
  const members = group.group.members;
  return (
    <div className="ml-4.5 flex flex-col gap-0.5 border-l pl-1.5">
      {members.map((member) => {
        const input = questions.get(member.questionId);
        const Icon = QUESTION_TYPE_ICONS[input?.type ?? "single_choice"];
        const current = selectedQuestionId === member.questionId;
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
  );
}

function SharedContentButton({
  line,
  onClick,
}: Readonly<{
  line: SharedContentLine;
  onClick: (opener: HTMLElement) => void;
}>) {
  const Icon = SHARED_ICONS[line.kind];
  return (
    <button
      type="button"
      className="text-muted-fg hover:bg-muted mb-0.5 ml-6.5 flex w-[calc(100%-1.625rem)] items-center gap-1.5 rounded-[6px] px-1.5 py-1 text-left text-xs leading-[1.45]"
      onClick={(event) => onClick(event.currentTarget)}
    >
      <Icon aria-hidden="true" className="size-3.25 shrink-0" />
      <span className="min-w-0 truncate leading-[18px]">{line.text}</span>
    </button>
  );
}

function groupPoints(group: GroupBundle): number {
  return group.questions.reduce((sum, question) => sum + question.input.points, 0);
}

function GroupHeading({
  group,
  title,
  selected,
  onSelect,
}: Readonly<{
  group: GroupBundle | undefined;
  title: string;
  selected: boolean;
  onSelect: () => void;
}>) {
  const { t } = useTranslation();
  const name = "min-w-0 flex-1 truncate px-1 py-0.75 text-left leading-4 font-medium";
  if (!group)
    return (
      <button type="button" className={name} disabled>
        <Skeleton className="h-3 w-full" />
      </button>
    );
  return (
    <>
      <button type="button" className={name} onClick={onSelect}>
        {title}
      </button>
      <span
        className={cn(
          "text-muted-fg shrink-0 text-xs whitespace-nowrap tabular-nums",
          selected && "max-[767px]:hidden",
        )}
      >
        {t("builder.sectionSummary", {
          questions: group.group.members.length,
          points: groupPoints(group),
        })}
      </span>
    </>
  );
}
