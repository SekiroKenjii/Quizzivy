import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { ProgressBar } from "@/components/shared/stats/ProgressBar";
import { percent, thresholdTone } from "@/components/shared/stats/progress";
import { useCan } from "@/features/auth/permissions";
import { getItemAnalysis, type ItemAnalysis as Analysis } from "../api";

const CARD = "bg-card shadow-card overflow-hidden rounded-xl border";

function itemAnalysisKey(assignmentId: string) {
  return ["admin-assignment", assignmentId, "item-analysis"] as const;
}

/**
 * ItemAnalysis is the Questions tab's card: every question of the pinned
 * version, hardest first, with its share of full marks as a bar in danger
 * below 40%, warning below 65% and success otherwise. It needs
 * `teaching.grading`; without it the card says so and reads nothing.
 */
export function ItemAnalysis({ assignmentId }: Readonly<{ assignmentId: string }>) {
  const { t } = useTranslation();
  const grading = useCan("teaching.grading");
  const analysis = useQuery({
    queryKey: itemAnalysisKey(assignmentId),
    queryFn: ({ signal }) => getItemAnalysis(assignmentId, signal),
    enabled: grading,
  });
  if (!grading)
    return (
      <section className={CARD} aria-label={t("assignmentDetail.analysis.title")}>
        <p className="text-muted-fg text-ui px-4 py-3.5">
          {t("assignmentDetail.analysis.noPermission")}
        </p>
      </section>
    );
  return (
    <section className={CARD} aria-label={t("assignmentDetail.analysis.title")}>
      <p className="text-muted-fg text-ui px-4 py-3.5">
        {t("assignmentDetail.analysis.intro")}
      </p>
      {analysis.isPending && (
        <div className="border-t p-4">
          <ListSkeleton rows={4} />
        </div>
      )}
      {analysis.isError && (
        <div className="border-t p-4">
          <LoadError error={analysis.error} onRetry={() => void analysis.refetch()}>
            {t("assignmentDetail.analysis.loadFailed")}
          </LoadError>
        </div>
      )}
      {analysis.isSuccess && <AnalysisRows data={analysis.data} />}
    </section>
  );
}

function AnalysisRows({ data }: Readonly<{ data: Analysis }>) {
  const { t } = useTranslation();
  if (data.handedIn === 0 || data.items.length === 0)
    return (
      <p className="text-muted-fg text-ui border-t px-4 py-3.5">
        {t(
          data.items.length === 0
            ? "assignmentDetail.noQuestions"
            : "assignmentDetail.analysis.empty",
        )}
      </p>
    );
  return (
    <ol aria-label={t("assignmentDetail.analysis.title")}>
      {data.items.map((item) => {
        const share = item.correctRate === null ? null : percent(item.correctRate, 1);
        return (
          <li
            key={item.questionId}
            className="flex flex-wrap items-center gap-x-3.5 gap-y-2 border-t px-4 py-3"
          >
            <span className="text-muted-fg text-meta w-8.5 flex-none tabular-nums">
              {t("assignmentDetail.analysis.number", { n: item.number })}
            </span>
            <span className="text-ui min-w-0 flex-[1_1_260px] [overflow-wrap:anywhere]">
              {item.promptExcerpt || t("assignmentDetail.analysis.noPrompt")}
              <span className="text-muted-fg block text-xs">
                {t(`questionEditor.type.${item.type}`, { defaultValue: item.type })}
              </span>
            </span>
            <span className="flex min-w-40 flex-[0_1_220px] items-center gap-2.5">
              <ProgressBar
                className="flex-1"
                value={share ?? 0}
                tone={share === null ? "accent" : thresholdTone(share)}
                label={
                  share === null
                    ? t("assignmentDetail.analysis.noMarks")
                    : t("assignmentDetail.analysis.correct", { n: share })
                }
              />
              <span className="text-meta w-9.5 text-right tabular-nums">
                {share === null ? "—" : `${share}%`}
              </span>
            </span>
          </li>
        );
      })}
    </ol>
  );
}
