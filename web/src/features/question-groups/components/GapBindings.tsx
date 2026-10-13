import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { GroupBundle } from "../api";
import { materialGaps, type GroupMaterial, type GroupGapBinding } from "../model";

const UNBOUND = "unbound";

type GapTarget = {
  key: string;
  binding:
    | Omit<Extract<GroupGapBinding, { kind: "question" }>, "gapId">
    | Omit<Extract<GroupGapBinding, { kind: "blank" }>, "gapId">;
  label: string;
};

function targetKey(binding: GroupGapBinding) {
  return binding.kind === "blank"
    ? `${binding.questionId}:${binding.blankGapId}`
    : binding.questionId;
}

/**
 * GapBindings is a material's cloze hint and one row per gap in its text: the
 * gap's chip, the select that binds it to a choice member or to one blank of a
 * fill-in member, and "Remove gap", which calls `onRemove` with the gap's id
 * and is drawn only when the host passes it. A target bound to another gap is
 * disabled, and an unbound row says so in its border, in the select's text
 * and in its accessible name. A material stored as Markdown holds no gaps and
 * renders nothing.
 */
export function GapBindings({
  material,
  bundle,
  onChange,
  onRemove,
}: Readonly<{
  material: GroupMaterial;
  bundle: GroupBundle;
  onChange: (material: GroupMaterial) => void;
  onRemove?: ((gapId: string) => void) | undefined;
}>) {
  const { t } = useTranslation();
  if (material.content.format !== "semantic_v1") return null;
  const gaps = materialGaps(material);
  const questions = new Map(
    bundle.questions.map((question) => [question.id, question.input]),
  );
  const targets = bundle.group.members.flatMap<GapTarget>((member, index) => {
    const question = questions.get(member.questionId);
    if (!question) return [];
    const number = index + 1;
    if (
      question.type === "single_choice" ||
      question.type === "multiple_choice" ||
      question.type === "true_false"
    )
      return [
        {
          key: member.questionId,
          binding: { kind: "question" as const, questionId: member.questionId },
          label: t("groups.questionTarget", { number }),
        },
      ];
    return (question.blanks ?? []).flatMap((blank) =>
      blank.gapId
        ? [
            {
              key: `${member.questionId}:${blank.gapId}`,
              binding: {
                kind: "blank" as const,
                questionId: member.questionId,
                blankGapId: blank.gapId,
              },
              label: t("groups.blankTarget", { number, blank: blank.ordinal }),
            },
          ]
        : [],
    );
  });
  const targetsByKey = new Map(targets.map((target) => [target.key, target]));
  const bindingsByGap = new Map(
    material.gaps.map((binding) => [binding.gapId, binding]),
  );
  const holders = new Map<string, string[]>();
  for (const item of bundle.group.stimuli)
    for (const binding of item.gaps) {
      const key = targetKey(binding);
      holders.set(key, [...(holders.get(key) ?? []), `${item.id}:${binding.gapId}`]);
    }
  const bind = (gapId: string, key: string) => {
    const others = material.gaps.filter((item) => item.gapId !== gapId);
    if (key === UNBOUND) {
      onChange({ ...material, gaps: others });
      return;
    }
    const target = targetsByKey.get(key);
    if (target)
      onChange({ ...material, gaps: [...others, { ...target.binding, gapId }] });
  };
  return (
    <div className="flex flex-col gap-3">
      <p className="text-muted-fg text-xs">{t("groups.gapHint")}</p>
      {gaps.length > 0 && (
        <div
          className={cn(
            "grid gap-x-2 gap-y-1.5",
            onRemove
              ? "grid-cols-[auto_minmax(0,1fr)_auto]"
              : "grid-cols-[auto_minmax(0,1fr)]",
          )}
        >
          {gaps.map((gap) => {
            const binding = bindingsByGap.get(gap.id);
            const self = `${material.id}:${gap.id}`;
            return (
              <div
                key={gap.id}
                role="group"
                aria-label={t(binding ? "groups.gapLabel" : "groups.gapLabelUnbound", {
                  label: gap.label,
                })}
                className="col-span-full grid grid-cols-subgrid items-center"
              >
                <span
                  aria-hidden="true"
                  className="border-ring bg-muted text-muted-fg inline-flex h-6 min-w-13 items-center justify-center rounded-[6px] border-[1.5px] border-dashed px-2 text-xs font-semibold whitespace-nowrap"
                >
                  {t("groups.gapChip", { label: gap.label })}
                </span>
                <Select
                  value={binding ? targetKey(binding) : UNBOUND}
                  onValueChange={(key) => bind(gap.id, key)}
                >
                  <SelectTrigger
                    size="sm"
                    aria-label={t("groups.gapTarget", { label: gap.label })}
                    aria-invalid={!binding}
                    className="bg-card text-fg text-meta in-data-[scale=deck]:bg-card in-data-[scale=deck]:text-meta dark:bg-card dark:hover:bg-card dark:in-data-[scale=deck]:bg-card dark:in-data-[scale=deck]:hover:bg-card h-8 w-full min-w-0 rounded-[7px] px-2 shadow-none"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value={UNBOUND}>
                        {t("groups.chooseTarget")}
                      </SelectItem>
                      {targets.map((target) => (
                        <SelectItem
                          key={target.key}
                          value={target.key}
                          disabled={(holders.get(target.key) ?? []).some(
                            (holder) => holder !== self,
                          )}
                        >
                          {target.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                {onRemove && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t("groups.removeGap", { label: gap.label })}
                    className="text-muted-fg hover:bg-hover dark:hover:bg-hover size-7 rounded-[6px] in-data-[scale=deck]:size-7 in-data-[scale=deck]:rounded-[6px]"
                    onClick={() => onRemove(gap.id)}
                  >
                    <X className="size-3.5" aria-hidden="true" />
                  </Button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
