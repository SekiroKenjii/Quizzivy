import type { RefObject } from "react";
import { useTranslation } from "react-i18next";
import { CircleSlash, Hash } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import type { ImportDraftSection } from "../../api";
import { questionPlaces, validPoints } from "../../draft";

type ExcludeValues = { reason: string };
type PointsValues = { points: string };

const EXCLUDE_INITIAL: ExcludeValues = { reason: "" };

function normalizePoints(value: string): string {
  return value.trim().replace(",", ".");
}

/**
 * ExcludeDialog asks why a question is left out of the test; the reason is
 * required, at most 500 characters, and saved with the import. Closing it
 * returns focus to `returnFocus` when that is set and still on the page.
 */
export function ExcludeDialog({
  label,
  open,
  returnFocus,
  onOpenChange,
  onExclude,
}: Readonly<{
  label: string;
  open: boolean;
  returnFocus: RefObject<HTMLElement | null>;
  onOpenChange: (open: boolean) => void;
  onExclude: (reason: string) => void;
}>) {
  const { t } = useTranslation();
  const fields: readonly FormField<ExcludeValues>[] = [
    {
      name: "reason",
      kind: "area",
      label: t("imports.review.excludeReason"),
      hint: t("imports.review.excludeHint"),
      placeholder: t("imports.review.excludePlaceholder"),
      maxLength: 500,
      required: true,
    },
  ];
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      returnFocus={returnFocus}
      icon={CircleSlash}
      danger
      title={t("imports.review.excludeTitle", { label })}
      description={t("imports.review.excludeBody")}
      initial={EXCLUDE_INITIAL}
      fields={fields}
      validate={(values) =>
        values.reason.trim() === ""
          ? { reason: t("imports.review.excludeReasonRequired") }
          : null
      }
      submitLabel={t("imports.review.excludeConfirm")}
      onSubmit={(values) => onExclude(values.reason.trim())}
    />
  );
}

/**
 * PointsDialog sets one value for every question still in the test, with the
 * total and each section's share; excluded questions are named and left out.
 */
export function PointsDialog({
  sections,
  initialPoints,
  open,
  onOpenChange,
  onApply,
}: Readonly<{
  sections: readonly ImportDraftSection[];
  initialPoints: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onApply: (points: string) => void;
}>) {
  const { t, i18n } = useTranslation();
  const places = questionPlaces(sections);
  const included = places.filter((place) => place.question.excluded === undefined);
  const excluded = places.filter((place) => place.question.excluded !== undefined);
  const fields: readonly FormField<PointsValues>[] = [
    {
      name: "points",
      kind: "text",
      inputMode: "decimal",
      label: t("imports.review.pointsEach"),
      maxLength: 9,
      required: true,
      noOptional: true,
    },
  ];
  const total = (values: PointsValues) => {
    const points = normalizePoints(values.points);
    if (!validPoints(points)) return t("imports.review.pointsUnknown");
    return new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(
      Number(points) * included.length,
    );
  };
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      icon={Hash}
      width={520}
      title={t("imports.review.pointsTitle")}
      description={(values) => (
        <>
          {t("imports.review.pointsBody")}{" "}
          <span className="text-fg font-semibold">
            {t("imports.review.pointsTotal", { total: total(values) })}
          </span>
        </>
      )}
      initial={{ points: initialPoints }}
      fields={fields}
      validate={(values) =>
        validPoints(normalizePoints(values.points))
          ? null
          : { points: t("imports.review.pointsInvalid") }
      }
      submitLabel={(values) => {
        const points = normalizePoints(values.points);
        return validPoints(points)
          ? t("imports.review.pointsApply", { points, count: included.length })
          : t("imports.review.pointsApplyPlain");
      }}
      onSubmit={(values) => onApply(normalizePoints(values.points))}
    >
      <ul className="m-0 flex list-none flex-col overflow-hidden rounded-[10px] border p-0">
        {sections.map((section) => (
          <li
            key={section.id}
            className="flex justify-between gap-2.5 border-t px-3 py-2.5 text-sm first:border-t-0"
          >
            <span className="min-w-0 font-medium [overflow-wrap:anywhere]">
              {section.title}
            </span>
            <span className="text-muted-fg flex-none tabular-nums">
              {t("imports.review.questionsCount", {
                count: included.filter((place) => place.sectionId === section.id)
                  .length,
              })}
            </span>
          </li>
        ))}
      </ul>
      {excluded.length > 0 ? (
        <p className="text-muted-fg text-meta m-0">
          {t("imports.review.pointsExcluded", {
            labels: excluded.map((place) => place.question.label).join(", "),
          })}
        </p>
      ) : null}
    </FormDialog>
  );
}
