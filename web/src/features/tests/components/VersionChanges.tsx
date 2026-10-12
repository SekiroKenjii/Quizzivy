import type { UseQueryResult } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import type { DiffChange, TestVersionDiff } from "@/features/tests/api";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { nfc } from "@/lib/nfc";
import { cn } from "@/lib/utils";

const CHIP: Record<DiffChange["kind"], string> = {
  added: "bg-success-soft text-success-ink",
  removed: "bg-danger-soft text-danger-ink",
  changed: "bg-warning-soft text-warning-ink",
  answer: "bg-info-soft text-info-ink",
  points: "bg-muted text-muted-fg",
};

function points(value: number | undefined, locale: Locale) {
  return new Intl.NumberFormat(locale).format(value ?? 0);
}

function changeText(
  change: DiffChange,
  previous: number,
  t: TFunction,
  locale: Locale,
): string {
  const n = change.questionNumber;
  const { params } = change;
  switch (change.kind) {
    case "added":
      return t("tests.detail.compare.added", { n, prompt: nfc(params.prompt ?? "") });
    case "removed":
      return t("tests.detail.compare.removed", {
        n,
        version: previous,
        prompt: nfc(params.prompt ?? ""),
      });
    case "changed":
      return t("tests.detail.compare.changed", {
        n,
        fields: new Intl.ListFormat(locale, { type: "unit" }).format(
          (params.fields ?? []).map((field) =>
            t(`tests.detail.compare.field.${field}`),
          ),
        ),
      });
    case "answer":
      return params.answerFrom && params.answerTo
        ? t("tests.detail.compare.answer", {
            n,
            from: params.answerFrom.join(", "),
            to: params.answerTo.join(", "),
          })
        : t("tests.detail.compare.answerKey", { n });
    case "points":
      return t("tests.detail.compare.points", {
        from: points(params.pointsFrom, locale),
        to: points(params.pointsTo, locale),
      });
  }
}

/**
 * VersionChanges is the band under the preview's header that lists what
 * changed from version `previous` to the version shown: one row per change
 * of `query`'s diff, with the deck's chip for its kind (Added, Removed,
 * Changed, Answer, Points) and a line naming the question. It shows a
 * skeleton while the diff loads, a retry when it fails, and a line when
 * nothing changed.
 */
export function VersionChanges({
  query,
  previous,
}: Readonly<{
  query: UseQueryResult<TestVersionDiff>;
  previous: number;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  return (
    <div
      role="region"
      aria-label={t("tests.detail.compare.toggle", { n: previous })}
      className="bg-sidebar flex flex-col gap-1.5 border-b px-4 py-3"
    >
      {query.isPending ? (
        <>
          <Skeleton className="h-5.5 w-3/4" />
          <Skeleton className="h-5.5 w-2/3" />
        </>
      ) : null}
      {query.isError ? (
        <p role="alert" className="flex flex-wrap items-center gap-2 text-sm">
          {t("tests.detail.compare.failed")}
          <Button size="xs" variant="outline" onClick={() => void query.refetch()}>
            {t("common.retry")}
          </Button>
        </p>
      ) : null}
      {query.isSuccess && query.data.changes.length === 0 ? (
        <p className="text-muted-fg text-sm">
          {t("tests.detail.compare.none", { n: previous })}
        </p>
      ) : null}
      {query.isSuccess && query.data.changes.length > 0 ? (
        <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
          {query.data.changes.map((change) => (
            <li
              key={`${change.kind}-${change.questionId ?? "total"}`}
              className="flex items-center gap-2.5 text-sm"
            >
              <span
                className={cn(
                  "text-caption inline-flex h-5.5 min-w-[70px] flex-none items-center justify-center rounded-[6px] px-2 font-semibold whitespace-nowrap",
                  CHIP[change.kind],
                )}
              >
                {t(`tests.detail.compare.kind.${change.kind}`)}
              </span>
              <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">
                {changeText(change, previous, t, locale)}
              </span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
