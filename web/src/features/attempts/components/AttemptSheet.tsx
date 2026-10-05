import { useLayoutEffect, useSyncExternalStore, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { Clock, Flag } from "lucide-react";
import { Sheet } from "@/components/shared/Sheet";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { Assignment } from "@/features/assignments/api";
import {
  elapsedMinutes,
  reviewMatches,
} from "@/features/assignments/pages/teacher/assignmentDetail";
import { scoreText } from "@/features/assignments/studentTime";
import { useCan, useWorkspace } from "@/features/auth/permissions";
import { Timeline } from "@/features/integrity/components/Timeline";
import { useTick } from "@/hooks/useTick";
import { ApiError } from "@/lib/api/errors";
import { countdown } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import {
  getAttemptForReview,
  isHandedIn,
  type AttemptReview,
  type MonitorRow,
} from "../api";
import { reviewKey } from "../keys";
import type { SheetNoteController } from "./sheetNotes";

type Props = Readonly<{
  assignment: Assignment;
  attemptId: string;
  row: MonitorRow | undefined;
  questionCount: number;
  serverTime: string;
  receivedAt: number;
  notes: SheetNoteController;
  onClose: () => void;
  recovery?: ReactNode;
}>;

/** AttemptSheet validates an authorized review before showing its compact timeline and blur-saved teacher-only note. */
export function AttemptSheet(props: Props) {
  const { assignment, attemptId, notes, onClose, recovery } = props;
  const { t } = useTranslation();
  const workspace = useWorkspace("teacher");
  const grade = useCan("teaching.grading");
  const intervene = useCan("teaching.attempts.intervene");
  const allowed = workspace && (grade || intervene);
  const review = useQuery({
    queryKey: reviewKey(attemptId),
    queryFn: ({ signal }) => getAttemptForReview(attemptId, signal),
    enabled: allowed && attemptId !== "",
  });
  const valid = allowed && reviewMatches(review.data, assignment.id, attemptId);
  const data = valid ? review.data : undefined;
  const savedNote = data?.teacherNote;
  useLayoutEffect(() => {
    if (savedNote !== undefined) notes.accept(attemptId, savedNote);
  }, [attemptId, savedNote, notes]);
  useLayoutEffect(() => {
    if (document.activeElement === document.body)
      document.querySelector<HTMLElement>("main")?.focus();
  }, [attemptId]);
  let content: ReactNode = (
    <p role="status" className="text-muted-fg text-sm">
      {t("assignmentDetail.sheet.unavailable")}
    </p>
  );
  if (allowed) {
    if (review.isPending) content = <ListSkeleton rows={4} />;
    else if (review.isError)
      content = (
        <LoadError error={review.error} onRetry={() => void review.refetch()}>
          {t("assignmentDetail.sheet.unavailable")}
        </LoadError>
      );
    else if (data !== undefined) content = <AttemptBody {...props} data={data} />;
  }
  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      width={420}
      title={data?.student.fullName ?? t("assignmentDetail.sheet.unavailableTitle")}
      subtitle={data === undefined ? undefined : assignment.testTitle}
      leading={
        data === undefined ? undefined : (
          <Avatar name={data.student.fullName} className="size-10 text-sm" />
        )
      }
      footer={data === undefined ? undefined : <AttemptFooter data={data} />}
    >
      {recovery}
      {content}
    </Sheet>
  );
}

function AttemptFooter({ data }: Readonly<{ data: AttemptReview }>) {
  const { t } = useTranslation();
  const grade = useCan("teaching.grading");
  const pending =
    isHandedIn(data.attempt.status) && (data.attempt.score?.pendingManual ?? 0) > 0;
  return (
    <>
      <Button variant="outline" asChild className="h-9.5 min-w-0 flex-1">
        <Link to={`/teacher/attempts/${data.attempt.id}`}>
          {t("assignmentDetail.sheet.openFull")}
        </Link>
      </Button>
      {grade && pending && (
        <Button asChild className="h-9.5 min-w-0 flex-1">
          <Link to={`/teacher/attempts/${data.attempt.id}`}>
            {t("assignmentDetail.sheet.grade")}
          </Link>
        </Button>
      )}
    </>
  );
}

function AttemptFacts({
  data,
  row,
  questionCount,
  serverTime,
  receivedAt,
}: Props & Readonly<{ data: AttemptReview }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const live = data.attempt.status === "in_progress";
  const tick = useTick(live);
  const remaining =
    row?.deadlineAt == null
      ? null
      : new Date(row.deadlineAt).getTime() -
        (tick * 1000 + new Date(serverTime).getTime() - receivedAt);
  const spent = elapsedMinutes(data.attempt);
  let time = "—";
  if (spent !== null)
    time =
      spent < 1
        ? t("papers.tookUnderMinute")
        : t("papers.tookMinutes", { count: spent });
  const score = data.attempt.score;
  return (
    <>
      <dl className="grid grid-cols-2 gap-2.5">
        <div className="rounded-[10px] border p-3">
          <dt className="text-muted-fg text-xs">{t("monitor.score")}</dt>
          <dd className="text-xl font-semibold tabular-nums">
            {score == null ? "—" : scoreText(score.earned, score.total, locale, t)}
          </dd>
        </div>
        <div className="rounded-[10px] border p-3">
          <dt className="text-muted-fg text-xs">
            {t("assignmentDetail.sheet.timeSpent")}
          </dt>
          <dd className="text-xl font-semibold tabular-nums">{time}</dd>
        </div>
      </dl>
      <div className="text-muted-fg flex flex-wrap gap-x-3 gap-y-1 text-xs">
        <span>
          {t("papers.attemptOf", { no: data.attempt.attemptNo, max: data.maxAttempts })}
        </span>
        {row?.answeredCount != null && (
          <span>
            {t("monitor.answeredOf", {
              answered: row.answeredCount,
              total: questionCount,
            })}
          </span>
        )}
        {live && remaining !== null && (
          <span className="inline-flex items-center gap-1 tabular-nums">
            <Clock aria-hidden="true" className="size-3.5" />
            {t("monitor.remaining")}: {countdown(remaining)}
          </span>
        )}
        {data.attempt.integrity?.flagged && (
          <span className="text-danger-ink inline-flex items-center gap-1">
            <Flag aria-hidden="true" className="size-3.5" />
            {t("status.attention.flagged")}
          </span>
        )}
      </div>
    </>
  );
}

function AttemptBody(props: Props & Readonly<{ data: AttemptReview }>) {
  const { data, notes } = props;
  const navigate = useNavigate();
  return (
    <>
      <AttemptFacts {...props} />
      <Timeline
        key={data.attempt.id}
        attemptId={data.attempt.id}
        questions={data.questions}
        live={data.attempt.status === "in_progress"}
        note={data.teacherNote}
        presentation="compact"
        onViewPaper={() => void navigate(`/teacher/attempts/${data.attempt.id}`)}
      />
      <AttemptNote data={data} notes={notes} />
    </>
  );
}

function AttemptNote({
  data,
  notes,
}: Readonly<{ data: AttemptReview; notes: SheetNoteController }>) {
  const { t } = useTranslation();
  const grade = useCan("teaching.grading");
  const attemptId = data.attempt.id;
  useSyncExternalStore(notes.subscribe, notes.snapshot, notes.snapshot);
  const draft = notes.get(attemptId);
  let status = t("assignmentDetail.sheet.saved");
  if (draft?.pending) status = t("assignmentDetail.sheet.saving");
  else if (draft && (draft.value.trim() === "" ? null : draft.value) !== draft.saved)
    status = t("assignmentDetail.sheet.unsaved");
  return (
    <div className="space-y-1.5">
      <Label htmlFor="attempt-sheet-note" className="text-sm font-semibold">
        {t("assignmentDetail.sheet.note")}
      </Label>
      <p id="attempt-sheet-note-help" className="text-muted-fg text-xs">
        {t("assignmentDetail.sheet.notePrivacy")}
      </p>
      {grade ? (
        <>
          <Textarea
            id="attempt-sheet-note"
            rows={3}
            aria-describedby="attempt-sheet-note-help"
            aria-invalid={draft?.error != null || (draft?.value.length ?? 0) > 2000}
            value={draft?.value ?? data.teacherNote ?? ""}
            placeholder={t("assignmentDetail.sheet.notePrivacy")}
            className="bg-bg rounded-ctl min-h-0 px-3 py-2.5"
            onChange={(event) => {
              if (grade) notes.change(attemptId, event.target.value);
            }}
            onBlur={() => {
              if (grade) void notes.flush(attemptId);
            }}
          />
          {(draft?.value.length ?? 0) > 2000 && (
            <p role="alert" className="text-danger-ink text-xs">
              {t("assignmentDetail.sheet.noteLimit")}
            </p>
          )}
          {draft?.error != null && (
            <div role="alert" className="space-y-1 text-xs">
              <p className="text-danger-ink">
                {draft.error instanceof ApiError
                  ? draft.error.message
                  : t("assignmentDetail.sheet.noteFailed")}
              </p>
              <Button
                variant="outline"
                size="sm"
                disabled={draft.pending}
                onClick={() => {
                  if (grade) void notes.flush(attemptId);
                }}
              >
                {t("common.retry")}
              </Button>
            </div>
          )}
          <p role="status" className="text-muted-fg text-xs">
            {status}
          </p>
        </>
      ) : (
        <p className="text-sm whitespace-pre-wrap">{data.teacherNote ?? "—"}</p>
      )}
    </div>
  );
}
