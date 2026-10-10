import { useId } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Link } from "react-router";
import { LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useLocale } from "@/lib/i18n/useLocale";
import { nfc } from "@/lib/nfc";
import type {
  ImportDraftQuestion,
  ImportDraftSection,
  ImportReview,
  ImportStatus,
} from "../../api";
import { questionPlaces } from "../../draft";
import type { CommitState } from "../../useImportCommit";
import { statusLine } from "./statusLine";

interface Row {
  label: string;
  value: string;
  sub: string[];
  blocks: boolean;
  link?: { label: string; search: string };
}

function includedOf(sections: readonly ImportDraftSection[]): ImportDraftQuestion[] {
  return questionPlaces(sections)
    .map((place) => place.question)
    .filter((question) => question.excluded === undefined);
}

function pointsRow(
  review: ImportReview,
  format: (value: number) => string,
  t: TFunction,
): Row {
  const label = t("imports.confirm.rowPoints");
  const included = includedOf(review.draft.sections);
  if (included.length === 0)
    return { label, value: t("imports.confirm.pointsNone"), sub: [], blocks: false };
  const values = new Set(included.map((question) => Number(question.points)));
  const origins = new Set(included.map((question) => question.origins.points));
  const each = values.size === 1 ? format([...values][0] ?? 0) : null;
  const totalLabel = t("imports.confirm.pointsTotal", {
    points: format(Number(review.summary.totalPoints)),
  });
  if (origins.has("defaulted")) {
    const listed = review.findings.some(
      (finding) => finding.severity === "informational" && finding.field === "points",
    );
    return {
      label,
      value: t("imports.confirm.pointsNotConfirmed"),
      sub: [
        each === null
          ? totalLabel
          : t("imports.confirm.pointsDefault", { points: each }),
      ],
      blocks: false,
      ...(listed
        ? { link: { label: t("imports.confirm.confirm"), search: "?filter=info" } }
        : {}),
    };
  }
  let sub = t("imports.confirm.pointsMixed");
  if (each !== null && origins.size === 1 && origins.has("teacher_entered"))
    sub = t("imports.confirm.pointsYou", { points: each });
  else if (each !== null && origins.size === 1 && origins.has("source_explicit"))
    sub = t("imports.confirm.pointsFile", { points: each });
  else if (each !== null) sub = t("imports.confirm.pointsDefault", { points: each });
  return { label, value: totalLabel, sub: [sub], blocks: false };
}

function answersRow(review: ImportReview, t: TFunction): Row {
  const { summary } = review;
  const row: Row = {
    label: t("imports.confirm.rowAnswers"),
    value: t("imports.confirm.ofTotal", {
      value: summary.answersKnown,
      total: summary.included,
    }),
    sub: [],
    blocks: review.findings.some(
      (finding) => finding.severity === "blocking" && finding.field === "answer",
    ),
  };
  if (row.blocks) {
    row.link = { label: t("imports.confirm.fix"), search: "?filter=blocking" };
    return row;
  }
  const unset = includedOf(review.draft.sections).find(
    (question) => question.answer.state !== "known",
  );
  if (unset !== undefined)
    row.link = {
      label: t("imports.confirm.goToQuestion", { label: unset.label }),
      search: `?question=${encodeURIComponent(unset.id)}`,
    };
  return row;
}

function passagesOf(sections: readonly ImportDraftSection[], t: TFunction): string[] {
  return sections.flatMap((section) =>
    section.items.flatMap((item) => {
      const group = item.group;
      if (group?.stimulus === undefined) return [];
      const kept = group.questions.filter(
        (question) => question.excluded === undefined,
      );
      if (kept.length === 0) return [];
      const label = group.label ?? kept[0]?.label ?? "";
      return [
        group.gaps.length > 0
          ? t("imports.confirm.passageGaps", { label, count: group.gaps.length })
          : t("imports.confirm.passageNote", { label }),
      ];
    }),
  );
}

function rowsOf(
  review: ImportReview,
  format: (value: number) => string,
  t: TFunction,
): Row[] {
  const { summary, draft } = review;
  const excluded = questionPlaces(draft.sections)
    .map((place) => place.question)
    .filter((question) => question.excluded !== undefined)
    .map((question) =>
      t("imports.confirm.excludedNote", {
        label: question.label,
        reason: nfc(question.excluded?.reason ?? ""),
      }),
    );
  const open = summary.blocking + summary.needsDecision;
  const still: Row = {
    label: t("imports.confirm.rowOpen"),
    value: String(open),
    sub: [],
    blocks: open > 0,
  };
  if (open > 0)
    still.link = { label: t("imports.confirm.review"), search: "?filter=all" };
  return [
    {
      label: t("imports.confirm.rowIncluded"),
      value: t("imports.confirm.ofTotal", {
        value: summary.included,
        total: summary.questions,
      }),
      sub: excluded,
      blocks: false,
    },
    {
      label: t("imports.confirm.rowStructure"),
      value: `${t("imports.review.sectionsCount", { count: summary.sections })} · ${t(
        "imports.review.groupsCount",
        { count: summary.groups },
      )}`,
      sub: passagesOf(draft.sections, t),
      blocks: false,
    },
    answersRow(review, t),
    pointsRow(review, format, t),
    {
      label: t("imports.confirm.rowMedia"),
      value: t("imports.confirm.mediaNone"),
      sub: [t("imports.confirm.mediaNote")],
      blocks: false,
    },
    still,
  ];
}

function hintOf(
  review: ImportReview,
  status: ImportStatus,
  state: CommitState,
  t: TFunction,
): string {
  if (state.phase === "lost") return t("imports.confirm.lost");
  if (status !== "needs_review") return statusLine(status, t);
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
 * will hold, row by row. A row that blocks the draft is in danger ink and
 * links into the review filtered to what blocks it; a gap that does not
 * block reads in normal ink and links to where it can be filled, never to an
 * empty filter. "Create draft test" stays focusable while it is off, with
 * the reason under it as its description, and does nothing then: while
 * anything is open, the review is not ready, the import is not under review
 * or a commit is under way. A lost response is retried with the same request.
 */
export function ConfirmSummary({
  importId,
  review,
  status,
  state,
  onCommit,
}: Readonly<{
  importId: string;
  review: ImportReview;
  status: ImportStatus;
  state: CommitState;
  onCommit: () => void;
}>) {
  const { t } = useTranslation();
  const titleId = useId();
  const hintId = useId();
  const locale = useLocale();
  const format = (value: number) => new Intl.NumberFormat(locale).format(value);
  const rows = rowsOf(review, format, t);
  const open = review.summary.blocking + review.summary.needsDecision;
  const pending = state.phase === "pending";
  const enabled = status === "needs_review" && review.ready && open === 0 && !pending;
  const closed = status === "cancelled" || status === "failed";

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
            data-blocks={row.blocks || undefined}
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
                  row.blocks
                    ? "text-danger-ink font-semibold tabular-nums"
                    : "font-semibold tabular-nums"
                }
              >
                {row.value}
              </span>
              {row.link ? (
                <Link
                  to={`/teacher/imports/${importId}/review${row.link.search}`}
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
          aria-disabled={!enabled}
          aria-describedby={hintId}
          className="text-body in-data-[scale=deck]:text-body h-11 rounded-[10px] font-semibold aria-disabled:cursor-default aria-disabled:opacity-50"
          onClick={() => {
            if (enabled) onCommit();
          }}
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
          id={hintId}
          role={state.phase === "lost" ? "alert" : undefined}
          className="text-muted-fg text-meta m-0 text-center leading-normal"
        >
          {hintOf(review, status, state, t)}
        </p>
        {closed ? (
          <Link
            to={`/teacher/imports/${importId}`}
            className="text-fg text-meta self-center rounded-sm underline underline-offset-2"
          >
            {t("imports.review.viewImport")}
          </Link>
        ) : null}
      </div>
    </section>
  );
}
