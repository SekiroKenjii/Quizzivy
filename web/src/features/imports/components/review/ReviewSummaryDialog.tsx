import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { ImportReviewSummary } from "../../api";
import type { AutosaveStatus } from "@/features/tests/useAutosave";
import type { FindingFilter } from "../../findings";
import type { CommitState } from "../../useImportCommit";

/**
 * ReviewSummaryDialog summarises what the draft test will contain and what is
 * still unresolved, each count opening its filtered set, and offers "Tạo bản
 * nháp đề" only for a saved, ready revision. A failed save can be retried from
 * here. After success it links to the new draft without navigating by itself.
 * `onCloseAutoFocus` decides where focus goes when it closes.
 */
export function ReviewSummaryDialog({
  open,
  onOpenChange,
  summary,
  ready,
  save,
  state,
  onCommit,
  onRetrySave,
  onShowFilter,
  onCloseAutoFocus,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  summary: ImportReviewSummary;
  ready: boolean;
  save: AutosaveStatus;
  state: CommitState;
  onCommit: () => void;
  onRetrySave: () => void;
  onShowFilter: (filter: FindingFilter) => void;
  onCloseAutoFocus: (event: Event) => void;
}>) {
  const { t } = useTranslation();
  const pending = state.phase === "pending";
  const unsaved = !SAVED.has(save.kind);
  const done = state.phase === "done" ? state.result : null;
  const rows: [string, string][] = [
    [t("imports.summary.included"), String(summary.included)],
    [t("imports.summary.excluded"), String(summary.excluded)],
    [t("imports.summary.sections"), String(summary.sections)],
    [t("imports.summary.groups"), String(summary.groups)],
    [t("imports.summary.points"), summary.totalPoints],
    [t("imports.summary.answersKnown"), String(summary.answersKnown)],
    [t("imports.summary.answersMissing"), String(summary.answersMissing)],
    [t("imports.summary.answersConflicting"), String(summary.answersConflicting)],
  ];

  return (
    <Dialog open={open} onOpenChange={(next) => !pending && onOpenChange(next)}>
      <DialogContent
        className="gap-4 p-5 sm:max-w-lg"
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle>
            {done === null
              ? t("imports.summary.title")
              : t("imports.summary.doneTitle")}
          </DialogTitle>
          <DialogDescription>
            {done === null ? t("imports.summary.body") : t("imports.summary.doneBody")}
          </DialogDescription>
        </DialogHeader>

        {done === null ? (
          <>
            <dl className="grid grid-cols-[1fr_auto] gap-x-4 gap-y-1.5 rounded-md border p-3 text-sm">
              {rows.map(([label, value]) => (
                <div key={label} className="contents">
                  <dt className="text-muted-foreground">{label}</dt>
                  <dd className="text-right tabular-nums">{value}</dd>
                </div>
              ))}
            </dl>

            {summary.blocking > 0 || summary.needsDecision > 0 ? (
              <div className="space-y-2">
                <p className="text-sm font-medium">{t("imports.summary.remaining")}</p>
                <div className="flex flex-wrap gap-2">
                  {summary.blocking > 0 ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => onShowFilter("blocking")}
                    >
                      {t("imports.summary.showBlocking", { count: summary.blocking })}
                    </Button>
                  ) : null}
                  {summary.needsDecision > 0 ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => onShowFilter("review")}
                    >
                      {t("imports.summary.showReview", {
                        count: summary.needsDecision,
                      })}
                    </Button>
                  ) : null}
                </div>
              </div>
            ) : null}

            <CommitNote
              ready={ready}
              save={save}
              state={state}
              onRetrySave={onRetrySave}
            />
          </>
        ) : null}

        <DialogFooter>
          {done === null ? (
            <>
              <Button
                type="button"
                variant="outline"
                className="flex-1"
                disabled={pending}
                onClick={() => onOpenChange(false)}
              >
                {t("imports.summary.keepReviewing")}
              </Button>
              <Button
                type="button"
                className="flex-1"
                disabled={!ready || unsaved || pending}
                onClick={onCommit}
              >
                {commitLabel(state, t)}
              </Button>
            </>
          ) : (
            <>
              <Button asChild variant="outline" className="flex-1">
                <Link to="/admin/imports">{t("imports.backToHistory")}</Link>
              </Button>
              <Button asChild className="flex-1">
                <Link to={`/admin/tests/${done.testId}/edit`}>
                  {t("imports.detail.openBuilder")}
                </Link>
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const SAVED: ReadonlySet<AutosaveStatus["kind"]> = new Set(["idle", "saved"]);

function commitLabel(state: CommitState, t: TFunction): string {
  if (state.phase === "pending") return t("imports.summary.committing");
  if (state.phase === "lost") return t("imports.summary.retryCommit");
  return t("imports.summary.commit");
}

function CommitNote({
  ready,
  save,
  state,
  onRetrySave,
}: Readonly<{
  ready: boolean;
  save: AutosaveStatus;
  state: CommitState;
  onRetrySave: () => void;
}>) {
  const { t } = useTranslation();
  if (state.phase === "pending")
    return (
      <p role="status" className="text-muted-foreground text-sm">
        {t("imports.summary.pendingNote")}
      </p>
    );
  if (state.phase === "lost")
    return (
      <p role="alert" className="text-sm">
        {t("imports.summary.lost")}
      </p>
    );
  if (state.phase === "unsaved")
    return (
      <p role="alert" className="text-sm">
        {t("imports.summary.saveFailed")}
      </p>
    );
  if (state.phase === "refused")
    return (
      <p role="alert" className="text-sm">
        {state.message}
      </p>
    );
  if (save.kind === "failed")
    return (
      <div role="alert" className="space-y-2 text-sm">
        <p>{t("imports.summary.saveFailed")}</p>
        {save.message === "" ? null : (
          <p className="text-muted-foreground text-xs">{save.message}</p>
        )}
        <Button type="button" variant="outline" size="xs" onClick={onRetrySave}>
          {t("common.retry")}
        </Button>
      </div>
    );
  if (save.kind === "stale")
    return (
      <p role="alert" className="text-sm">
        {t("imports.summary.stale")}
      </p>
    );
  if (save.kind === "dirty" || save.kind === "saving")
    return (
      <p className="text-muted-foreground text-sm">{t("imports.summary.unsaved")}</p>
    );
  if (!ready)
    return (
      <p className="text-muted-foreground text-sm">{t("imports.summary.notReady")}</p>
    );
  return (
    <p className="text-muted-foreground text-sm">{t("imports.summary.readyNote")}</p>
  );
}
