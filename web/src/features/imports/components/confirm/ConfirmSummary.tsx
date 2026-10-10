import { useId } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Link } from "react-router";
import { LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useLocale } from "@/lib/i18n/useLocale";
import { nfc } from "@/lib/nfc";
import type { ImportDraftSection, ImportReview } from "../../api";
import { questionPlaces } from "../../draft";
import type { FindingFilter } from "../../findings";
import type { CommitState } from "../../useImportCommit";

interface Row {
  label: string;
  value: string;
  sub: string[];
  link?: { label: string; filter: FindingFilter };
}

function pointsRow(
  sections: readonly ImportDraftSection[],
  total: string,
  format: (value: number) => string,
  t: TFunction,
): Row {
  const included = questionPlaces(sections)
    .map((place) => place.question)
    .filter((question) => question.excluded === undefined);
  const values = new Set(included.map((question) => Number(question.points)));
  const origins = new Set(included.map((question) => question.origins.points));
  const each = values.size === 1 ? format([...values][0] ?? 0) : null;
  const totalLabel = t("imports.confirm.pointsTotal", {
    points: format(Number(total)),
  });
  if (origins.has("defaulted"))
    return {
      label: t("imports.confirm.rowPoints"),
      value: t("imports.confirm.pointsNotConfirmed"),
      sub: [
        each === null
          ? totalLabel
          : t("imports.confirm.pointsDefault", { points: each }),
      ],
      link: { label: t("imports.confirm.confirm"), filter: "info" },
    };
  let sub = t("imports.confirm.pointsMixed");
  if (each !== null && origins.size === 1 && origins.has("teacher_entered"))
    sub = t("imports.confirm.pointsYou", { points: each });
  else if (each !== null && origins.size === 1 && origins.has("source_explicit"))
    sub = t("imports.confirm.pointsFile", { points: each });
  else if (each !== null) sub = t("imports.confirm.pointsDefault", { points: each });
  return { label: t("imports.confirm.rowPoints"), value: totalLabel, sub: [sub] };
}

function rowsOf(
  review: ImportReview,
  format: (value: number) => string,
  t: TFunction,
): Row[] {
  const { summary, draft } = review;
  const places = questionPlaces(draft.sections);
  const excluded = places
    .map((place) => place.question)
    .filter((question) => question.excluded !== undefined)
    .map((question) =>
      t("imports.confirm.excludedNote", {
        label: question.label,
        reason: nfc(question.excluded?.reason ?? ""),
      }),
    );
  const passages = draft.sections.flatMap((section) =>
    section.items.flatMap((item) => {
      const group = item.group;
      if (group?.stimulus === undefined) return [];
      const label = group.label ?? group.questions[0]?.label ?? "";
      return [
        group.gaps.length > 0
          ? t("imports.confirm.passageGaps", { label, count: group.gaps.length })
          : t("imports.confirm.passageNote", { label }),
      ];
    }),
  );
  const open = summary.blocking + summary.needsDecision;
  const answers: Row = {
    label: t("imports.confirm.rowAnswers"),
    value: t("imports.confirm.ofTotal", {
      value: summary.answersKnown,
      total: summary.included,
    }),
    sub: [],
  };
  if (summary.answersKnown < summary.included)
    answers.link = { label: t("imports.confirm.fix"), filter: "blocking" };
  const still: Row = {
    label: t("imports.confirm.rowOpen"),
    value: String(open),
    sub: [],
  };
  if (open > 0) still.link = { label: t("imports.confirm.review"), filter: "all" };
  return [
    {
      label: t("imports.confirm.rowIncluded"),
      value: t("imports.confirm.ofTotal", {
        value: summary.included,
        total: summary.questions,
      }),
      sub: excluded,
    },
    {
      label: t("imports.confirm.rowStructure"),
      value: `${t("imports.review.sectionsCount", { count: summary.sections })} · ${t(
        "imports.review.groupsCount",
        { count: summary.groups },
      )}`,
      sub: passages,
    },
    answers,
    pointsRow(draft.sections, summary.totalPoints, format, t),
    {
      label: t("imports.confirm.rowMedia"),
      value: t("imports.confirm.mediaNone"),
      sub: [t("imports.confirm.mediaNote")],
    },
    still,
  ];
}

function hintOf(
  review: ImportReview,
  underReview: boolean,
  state: CommitState,
  t: TFunction,
): string {
  if (state.phase === "lost") return t("imports.confirm.lost");
  if (!underReview) return t("imports.confirm.notUnderReview");
  const open = review.summary.blocking + review.summary.needsDecision;
  if (open > 0) return t("imports.confirm.hintOpen", { count: open });
  if (!review.ready) return t("imports.confirm.hintNotReady");
  return t("imports.confirm.hintReady");
}

function commitLabel(state: CommitState, t: TFunction): string {
  if (state.phase === "pending") return t("imports.confirm.committing");
  if (state.phase === "lost") return t("imports.confirm.retry");
  return t("imports.confirm.commit");
}

/**
 * ConfirmSummary is the "Summary" card beside the preview: what the draft
 * will hold, row by row, each value that still needs the teacher drawn in
 * danger ink with a link into the review filtered to it, and "Create draft
 * test", which stays off while anything is open, the review is not ready,
 * the import is not under review, or a commit is under way. A lost response
 * is retried with the same request.
 */
export function ConfirmSummary({
  importId,
  review,
  underReview,
  state,
  onCommit,
}: Readonly<{
  importId: string;
  review: ImportReview;
  underReview: boolean;
  state: CommitState;
  onCommit: () => void;
}>) {
  const { t } = useTranslation();
  const titleId = useId();
  const locale = useLocale();
  const format = (value: number) => new Intl.NumberFormat(locale).format(value);
  const rows = rowsOf(review, format, t);
  const open = review.summary.blocking + review.summary.needsDecision;
  const pending = state.phase === "pending";
  const enabled = underReview && review.ready && open === 0 && !pending;

  return (
    <section
      aria-labelledby={titleId}
      className="bg-card shadow-card overflow-hidden rounded-xl border"
    >
      <h2 id={titleId} className="border-b px-4 py-3.5 text-base font-semibold">
        {t("imports.confirm.summaryTitle")}
      </h2>
      <dl className="m-0">
        {rows.map((row) => (
          <div
            key={row.label}
            className="text-ui flex items-start gap-3 border-t px-4 py-2.5 first:border-t-0"
          >
            <dt className="min-w-0 flex-1">
              <span className="text-muted-fg block">{row.label}</span>
              {row.sub.map((line) => (
                <span key={line} className="text-muted-fg block text-xs leading-[1.45]">
                  {line}
                </span>
              ))}
            </dt>
            <dd className="m-0 flex flex-col items-end gap-0.5 text-right">
              <span
                className={
                  row.link
                    ? "text-danger-ink font-semibold tabular-nums"
                    : "font-semibold tabular-nums"
                }
              >
                {row.value}
              </span>
              {row.link ? (
                <Link
                  to={`/teacher/imports/${importId}/review?filter=${row.link.filter}`}
                  className="text-fg rounded-sm text-xs underline underline-offset-2"
                >
                  {row.link.label}
                </Link>
              ) : null}
            </dd>
          </div>
        ))}
      </dl>
      <div className="flex flex-col gap-2 border-t px-4 py-3.5">
        <Button
          type="button"
          className="text-body h-11 rounded-[10px] font-semibold"
          disabled={!enabled}
          onClick={onCommit}
        >
          {pending ? (
            <LoaderCircle
              aria-hidden="true"
              className="size-4 animate-spin motion-reduce:animate-none"
            />
          ) : null}
          {commitLabel(state, t)}
        </Button>
        {state.phase === "refused" ? (
          <p
            role="alert"
            className="text-danger-ink text-meta m-0 text-center leading-normal"
          >
            {state.message}
          </p>
        ) : null}
        <p
          role={state.phase === "lost" ? "alert" : undefined}
          className="text-muted-fg text-meta m-0 text-center leading-normal"
        >
          {hintOf(review, underReview, state, t)}
        </p>
      </div>
    </section>
  );
}
