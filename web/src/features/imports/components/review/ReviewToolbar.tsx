import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronsUpDown, ChevronUp } from "lucide-react";
import { Segmented } from "@/components/ui/segmented";
import { cn } from "@/lib/utils";
import { FINDING_FILTERS, type FindingFilter } from "../../findings";

/** ReviewPane is the pane a narrow review shows: the source or the test. */
export type ReviewPane = "source" | "exam";

/** SectionChoice is one entry of the toolbar's section select. */
export interface SectionChoice {
  id: string;
  title: string;
}

const FILTER_LABEL: Record<FindingFilter, string> = {
  all: "imports.review.filterAll",
  blocking: "imports.review.filterBlocking",
  review: "imports.review.filterReview",
  info: "imports.review.filterInfo",
};

const NAV_BUTTON =
  "bg-card hover:bg-muted grid size-8 flex-none cursor-pointer place-items-center rounded-[7px] border disabled:cursor-default disabled:opacity-45 disabled:hover:bg-card";

/**
 * ReviewToolbar sits under the review's top bar: the section select, the
 * finding filter with its counts, the Source | Test switch on a narrow
 * review, and the navigator through the open items ("{i} of {n} open",
 * Previous and Next).
 */
export function ReviewToolbar({
  sections,
  section,
  onSection,
  filter,
  counts,
  onFilter,
  narrow,
  pane,
  onPane,
  position,
  canStep,
  onStep,
}: Readonly<{
  sections: readonly SectionChoice[];
  section: string;
  onSection: (section: string) => void;
  filter: FindingFilter;
  counts: Readonly<Record<FindingFilter, number>>;
  onFilter: (filter: FindingFilter) => void;
  narrow: boolean;
  pane: ReviewPane;
  onPane: (pane: ReviewPane) => void;
  position: string;
  canStep: boolean;
  onStep: (delta: 1 | -1) => void;
}>) {
  const { t } = useTranslation();
  return (
    <div className="bg-sidebar flex flex-none flex-wrap items-center gap-2 border-b px-4 py-2">
      <span className="relative inline-flex max-w-60 items-center">
        <select
          value={section}
          aria-label={t("imports.review.sectionLabel")}
          onChange={(event) => onSection(event.target.value)}
          className="bg-card h-8 max-w-60 min-w-0 cursor-pointer appearance-none truncate rounded-lg border pr-7.5 pl-2.5 text-sm"
        >
          <option value="all">{t("imports.review.allSections")}</option>
          {sections.map((choice) => (
            <option key={choice.id} value={choice.id}>
              {choice.title}
            </option>
          ))}
        </select>
        <ChevronsUpDown
          aria-hidden="true"
          className="text-muted-fg pointer-events-none absolute right-2.25 size-3.25"
        />
      </span>
      <div
        role="group"
        aria-label={t("imports.review.filterLabel")}
        className="bg-muted rounded-ctl flex max-w-full [scrollbar-width:none] gap-0.5 overflow-x-auto p-[0.1875rem]"
      >
        {FINDING_FILTERS.map((value) => {
          const on = value === filter;
          return (
            <button
              key={value}
              type="button"
              aria-pressed={on}
              onClick={() => onFilter(value)}
              className={cn(
                "rounded-seg text-meta flex h-7 flex-none cursor-pointer items-center gap-1.5 px-2.5 font-medium whitespace-nowrap",
                on ? "bg-card text-fg shadow-card ring-border ring-1" : "text-muted-fg",
              )}
            >
              {t(FILTER_LABEL[value])}{" "}
              <span className="tabular-nums opacity-70">{counts[value]}</span>
            </button>
          );
        })}
      </div>
      <span className="flex-1" />
      {narrow ? (
        <Segmented
          size="sm"
          label={t("imports.review.viewLabel")}
          value={pane}
          options={[
            { value: "source", label: t("imports.review.viewSource") },
            { value: "exam", label: t("imports.review.viewExam") },
          ]}
          onChange={(next) => onPane(next === "source" ? "source" : "exam")}
        />
      ) : null}
      <div className="flex items-center gap-1">
        <button
          type="button"
          aria-label={t("imports.review.previousFinding")}
          title={t("imports.review.previousFinding")}
          disabled={!canStep}
          onClick={() => onStep(-1)}
          className={NAV_BUTTON}
        >
          <ChevronUp aria-hidden="true" className="size-3.75" />
        </button>
        <span
          role="status"
          aria-live="polite"
          className="text-muted-fg text-meta min-w-23 text-center tabular-nums"
        >
          {position}
        </span>
        <button
          type="button"
          disabled={!canStep}
          onClick={() => onStep(1)}
          className={cn(NAV_BUTTON, "text-meta flex w-auto gap-1 px-2.5 font-medium")}
        >
          {t("imports.review.nextFinding")}
          <ChevronDown aria-hidden="true" className="size-3.75" />
        </button>
      </div>
    </div>
  );
}
