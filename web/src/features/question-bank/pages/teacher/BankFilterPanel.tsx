import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Checkbox } from "@/components/ui/checkbox";
import type { QuestionType } from "@/features/question-bank/api";
import type { components } from "@/lib/api/schema";
import {
  BANK_LEVELS,
  BANK_SKILLS,
  BANK_TYPES,
  toggled,
  type BankFilters,
  type QuestionLevel,
  type QuestionSkill,
  type TagMatch,
} from "./bankFilters";
import { BankTagFilter } from "./BankTagFilter";

type Facets = components["schemas"]["QuestionTypeFacets"];

/** BankFilterChange names the one filter a control of the panel changes. */
export type BankFilterChange =
  | { readonly types: readonly QuestionType[] }
  | { readonly levels: readonly QuestionLevel[] }
  | { readonly skills: readonly QuestionSkill[] }
  | { readonly tags: readonly string[] }
  | { readonly tagMatch: TagMatch }
  | { readonly audio: boolean };

function Option({
  label,
  count,
  checked,
  onChange,
}: Readonly<{
  label: string;
  count: number | undefined;
  checked: boolean;
  onChange: () => void;
}>) {
  return (
    <label className="flex cursor-pointer items-center gap-2.25 px-0.5 py-1.25 text-sm leading-normal">
      <Checkbox checked={checked} onChange={onChange} />
      <span className="min-w-0 flex-1 break-words">{label}</span>
      {count === undefined ? null : (
        <span className="text-muted-fg text-xs tabular-nums">{count}</span>
      )}
    </label>
  );
}

function Group({
  legend,
  children,
}: Readonly<{ legend: string; children: ReactNode }>) {
  return (
    <fieldset className="min-w-0">
      <legend className="text-meta mb-1.5 leading-normal font-semibold">
        {legend}
      </legend>
      {children}
    </fieldset>
  );
}

/**
 * BankFilterPanel is the bank's filters as the deck's aside draws them: Type
 * (the stored types and Audio), Level and Skill as checkbox lists with the
 * server's counts, then the tag search. Audio has no count, because the
 * contract's facets carry none. Every change goes through `onChange`.
 */
export function BankFilterPanel({
  filters,
  facets,
  tags,
  onChange,
}: Readonly<{
  filters: BankFilters;
  facets: Facets | undefined;
  tags: readonly string[];
  onChange: (change: BankFilterChange) => void;
}>) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-4.5">
      <Group legend={t("bank.typeFilter")}>
        {BANK_TYPES.map((type) => (
          <Option
            key={type}
            label={t(`questionEditor.type.${type}`)}
            count={facets?.[type]}
            checked={filters.types.includes(type)}
            onChange={() => onChange({ types: toggled(filters.types, type) })}
          />
        ))}
        <Option
          label={t("bank.audio")}
          count={undefined}
          checked={filters.audio}
          onChange={() => onChange({ audio: !filters.audio })}
        />
      </Group>
      <Group legend={t("bank.levelFilter")}>
        {BANK_LEVELS.map((level) => (
          <Option
            key={level}
            label={t(`bank.level.${level}`)}
            count={facets?.levels[level]}
            checked={filters.levels.includes(level)}
            onChange={() => onChange({ levels: toggled(filters.levels, level) })}
          />
        ))}
      </Group>
      <Group legend={t("bank.skillFilter")}>
        {BANK_SKILLS.map((skill) => (
          <Option
            key={skill}
            label={t(`tests.skill.${skill}`)}
            count={facets?.skills[skill]}
            checked={filters.skills.includes(skill)}
            onChange={() => onChange({ skills: toggled(filters.skills, skill) })}
          />
        ))}
      </Group>
      <BankTagFilter
        options={tags}
        selected={filters.tags}
        match={filters.tagMatch}
        onSelected={(next) => onChange({ tags: next })}
        onMatch={(next) => onChange({ tagMatch: next })}
      />
    </div>
  );
}
