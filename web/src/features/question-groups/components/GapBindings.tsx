import { useTranslation } from "react-i18next";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
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

export function GapBindings({
  material,
  bundle,
  onChange,
}: Readonly<{
  material: GroupMaterial;
  bundle: GroupBundle;
  onChange: (material: GroupMaterial) => void;
}>) {
  const { t } = useTranslation();
  const gaps = materialGaps(material);
  if (!gaps.length) return null;
  const questions = new Map(
    bundle.questions.map((question) => [question.id, question.input]),
  );
  const targets = bundle.group.members.flatMap<{
    key: string;
    binding:
      | Omit<Extract<GroupGapBinding, { kind: "question" }>, "gapId">
      | Omit<Extract<GroupGapBinding, { kind: "blank" }>, "gapId">;
    label: string;
  }>((member, index) => {
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
  const keyFor = (binding: GroupGapBinding) =>
    binding.kind === "blank"
      ? `${binding.questionId}:${binding.blankGapId}`
      : binding.questionId;
  return (
    <FieldGroup>
      <p className="text-sm font-medium">{t("groups.gapBindings")}</p>
      <FieldDescription>{t("groups.gapHint")}</FieldDescription>
      {gaps.map((gap) => {
        const binding = material.gaps.find((item) => item.gapId === gap.id);
        const unavailable = new Set(
          bundle.group.stimuli.flatMap((item) =>
            item.gaps
              .filter((target) => item.id !== material.id || target.gapId !== gap.id)
              .map(keyFor),
          ),
        );
        return (
          <Field key={gap.id} data-invalid={!binding}>
            <FieldLabel htmlFor={`group-gap-${gap.id}`}>
              {t("groups.gapLabel", { label: gap.label })}
            </FieldLabel>
            <Select
              value={binding ? keyFor(binding) : ""}
              onValueChange={(key) => {
                if (key === "unbound") {
                  onChange({
                    ...material,
                    gaps: material.gaps.filter((item) => item.gapId !== gap.id),
                  });
                  return;
                }
                const target = targets.find((candidate) => candidate.key === key);
                if (target)
                  onChange({
                    ...material,
                    gaps: [
                      ...material.gaps.filter((item) => item.gapId !== gap.id),
                      { ...target.binding, gapId: gap.id },
                    ],
                  });
              }}
            >
              <SelectTrigger id={`group-gap-${gap.id}`} aria-invalid={!binding}>
                <SelectValue placeholder={t("groups.chooseTarget")} />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="unbound">{t("groups.removeBinding")}</SelectItem>
                  {targets.map((target) => (
                    <SelectItem
                      key={target.key}
                      value={target.key}
                      disabled={unavailable.has(target.key)}
                    >
                      {target.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
        );
      })}
    </FieldGroup>
  );
}
