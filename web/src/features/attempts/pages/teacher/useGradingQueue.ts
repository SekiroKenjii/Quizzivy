import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { authStore } from "@/stores/auth";
import { can, hasWorkspace } from "@/features/auth/permissions";
import { useAuthStore } from "@/stores/auth";
import { toast } from "@/components/ui/sonner";
import { failureMessage } from "@/lib/api/errors";
import {
  finishGrading,
  getAttemptForReview,
  gradeAttempt,
  listGradingQueue,
  type AttemptListRow,
  type GradingQueueItem,
  type GradingQueueParams,
} from "../../api";
import { gradingKey, monitorKey, reviewKey } from "../../keys";
import {
  gradingGroupKey,
  gradingItemKey,
  scanGradingCandidates,
  scoreOptions,
} from "./gradingRecovery";

/** GradingDraft retains the acknowledged mark separately from the current comment and score intent. */
export interface GradingDraft {
  item: GradingQueueItem;
  points: number | null;
  comment: string;
  acknowledged: { points: number; comment: string } | null;
}

/** FinishState retains explicit completion recovery independently of pending queue membership. */
export interface FinishState {
  item: Pick<GradingQueueItem, "attemptId" | "assignmentId" | "studentId">;
  error: string;
}

/** useGradingQueue serializes explicit marks and Finish actions under the mounted actor and filter owner. */
export function useGradingQueue(params: GradingQueueParams) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [actor] = useState(authStore.captureActor);
  const [controller] = useState(() => new AbortController());
  const lock = useRef(false);
  const scoreSave = useRef<{
    key: string;
    result: Promise<GradingDraft | null>;
  } | null>(null);
  const waitingNext = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<Record<string, GradingDraft>>({});
  const [finishes, setFinishes] = useState<Record<string, FinishState>>({});
  const [finishReady, setFinishReady] = useState<FinishState["item"] | null>(null);
  const [completed, setCompleted] = useState<ReadonlySet<string>>(new Set());
  const [picked, setPicked] = useState<string | null>(null);
  const [candidate, setCandidate] = useState<AttemptListRow | null>(null);
  const mode = params.mode ?? "student";
  const assignment = params.assignmentId ?? "";
  const student = params.studentId ?? "";
  const current = () => {
    const user = useAuthStore.getState().user;
    return (
      !controller.signal.aborted &&
      authStore.isCurrent(actor) &&
      can(user, "teaching.grading") &&
      hasWorkspace(user, "teacher")
    );
  };
  useEffect(() => () => controller.abort(), [controller]);
  const queue = useQuery({
    queryKey: gradingKey(actor.generation, mode, assignment, student),
    queryFn: ({ signal }) => listGradingQueue(params, signal),
    retry: false,
  });
  const recovery = useQuery({
    queryKey: ["teacher-grading-recovery", actor.generation, assignment, student],
    queryFn: ({ signal }) => scanGradingCandidates(signal, assignment, student),
    retry: false,
  });
  const candidateReview = useQuery({
    queryKey: ["teacher-grading-candidate", actor.generation, candidate?.id],
    queryFn: ({ signal }) => getAttemptForReview(candidate!.id, signal),
    enabled: candidate !== null,
    retry: false,
  });
  const items = useMemo(() => {
    const seen = new Map<string, GradingQueueItem>();
    for (const item of queue.data?.items ?? []) {
      if (!completed.has(item.attemptId)) seen.set(gradingItemKey(item), item);
    }
    for (const draft of Object.values(drafts)) {
      if (!completed.has(draft.item.attemptId) && !seen.has(gradingItemKey(draft.item)))
        seen.set(gradingItemKey(draft.item), draft.item);
    }
    const groups = new Map(
      (queue.data?.groups ?? []).map((group, index) => [group.key, index]),
    );
    return [...seen.values()].sort(
      (a, b) =>
        (groups.get(gradingGroupKey(a, mode)) ?? Number.MAX_SAFE_INTEGER) -
        (groups.get(gradingGroupKey(b, mode)) ?? Number.MAX_SAFE_INTEGER),
    );
  }, [queue.data, drafts, completed, mode]);
  const selected = items.find((item) => gradingItemKey(item) === picked) ?? items[0];
  const draft = selected ? drafts[gradingItemKey(selected)] : undefined;
  const notify = (cause: unknown) => failureMessage(cause, t("grading.saveFailed"));
  const invalidate = (item: FinishState["item"], recovery = true) => {
    if (recovery)
      void client.invalidateQueries({ queryKey: ["teacher-grading-recovery"] });
    for (const queryKey of [
      ["teacher-grading"],
      reviewKey(item.attemptId),
      monitorKey(item.assignmentId),
      ["admin-attempts"],
      ["admin-answers-by-question", item.assignmentId],
      ["admin-dashboard"],
    ])
      void client.invalidateQueries({ queryKey });
  };
  const acquire = () => {
    if (!current() || lock.current) return false;
    lock.current = true;
    setBusy(true);
    setError(null);
    return true;
  };
  const release = () => {
    lock.current = false;
    if (current()) setBusy(false);
  };
  const draftFor = (item: GradingQueueItem): GradingDraft =>
    drafts[gradingItemKey(item)] ?? {
      item,
      points: item.score,
      comment: item.comment ?? "",
      acknowledged:
        item.score === null
          ? null
          : { points: item.score, comment: item.comment ?? "" },
    };
  const write = async (item: GradingQueueItem, next: GradingDraft) => {
    if (next.points === null || !current()) return null;
    const score = await gradeAttempt(
      item.attemptId,
      [
        {
          questionId: item.questionId,
          points: next.points,
          comment: next.comment.trim() || null,
        },
      ],
      controller.signal,
    );
    if (!current()) return null;
    setDrafts((previous) => ({
      ...previous,
      [gradingItemKey(item)]: {
        ...next,
        acknowledged: { points: next.points!, comment: next.comment.trim() },
      },
    }));
    invalidate(item, false);
    return score;
  };
  const flush = async (item: GradingQueueItem, next = draftFor(item)) => {
    if (next.points === null) return null;
    const ack = next.acknowledged;
    if (ack?.points === next.points && ack.comment === next.comment.trim()) return null;
    return write(item, next);
  };
  const pickScore = async (item: GradingQueueItem, points: number) => {
    if (!Number.isFinite(points) || points < 0 || points > item.points || !acquire())
      return;
    setFinishReady(null);
    const next = { ...draftFor(item), points };
    setDrafts((previous) => ({ ...previous, [gradingItemKey(item)]: next }));
    setPicked(gradingItemKey(item));
    const result = (async () => {
      try {
        const saved = await write(item, next);
        return saved && current()
          ? { ...next, acknowledged: { points, comment: next.comment.trim() } }
          : null;
      } catch (cause) {
        if (current()) setError(notify(cause));
        return null;
      } finally {
        release();
      }
    })();
    scoreSave.current = { key: gradingItemKey(item), result };
    await result;
  };
  const comment = (item: GradingQueueItem, value: string) => {
    if (!current() || lock.current) return;
    setFinishReady(null);
    setDrafts((previous) => ({
      ...previous,
      [gradingItemKey(item)]: { ...draftFor(item), comment: value },
    }));
  };
  const select = async (item: GradingQueueItem) => {
    if (!acquire()) return;
    try {
      if (selected) await flush(selected);
      if (current()) {
        setPicked(gradingItemKey(item));
        setFinishReady(null);
        setCandidate(null);
      }
    } catch (cause) {
      if (current()) setError(notify(cause));
    } finally {
      release();
    }
  };
  const leave = async (action: () => void) => {
    if (!acquire()) return;
    try {
      if (selected) {
        const next = draftFor(selected);
        if (
          next.points === null &&
          next.comment.trim() !== (selected.comment ?? "").trim()
        ) {
          setError(t("grading.pickFirst"));
          return;
        }
        await flush(selected);
      }
      if (current()) action();
    } catch (cause) {
      if (current()) setError(notify(cause));
    } finally {
      release();
    }
  };
  const move = (delta: number) => {
    const index = selected
      ? items.findIndex((item) => gradingItemKey(item) === gradingItemKey(selected))
      : -1;
    const next = items[index + delta];
    if (next) void select(next);
  };
  const finish = async (item: FinishState["item"]) => {
    await finishGrading(item.attemptId, controller.signal);
    if (!current()) return;
    setCompleted((previous) => new Set([...previous, item.attemptId]));
    setFinishes((previous) => {
      const next = { ...previous };
      delete next[item.attemptId];
      return next;
    });
    setCandidate(null);
    invalidate(item);
  };
  const rememberFinish = (item: FinishState["item"], cause: unknown) => {
    if (current())
      setFinishes((previous) => ({
        ...previous,
        [item.attemptId]: { item, error: notify(cause) },
      }));
  };
  const pendingAfterSave = async (
    item: FinishState["item"],
    pending: number | undefined,
  ) => {
    if (pending !== undefined) return pending;
    const review = await getAttemptForReview(item.attemptId, controller.signal);
    if (
      !current() ||
      review.attempt.id !== item.attemptId ||
      review.attempt.assignmentId !== item.assignmentId ||
      review.attempt.studentId !== item.studentId
    )
      return undefined;
    return review.attempt.score?.pendingManual;
  };
  const waitForScore = async (item: GradingQueueItem, numericIntent: boolean) => {
    if (waitingNext.current || !current()) return false;
    if (!lock.current && !numericIntent) return undefined;
    const operation = scoreSave.current;
    if (!operation || operation.key !== gradingItemKey(item)) return false;
    waitingNext.current = true;
    try {
      const acknowledged = await operation.result;
      return acknowledged && current() ? acknowledged : false;
    } finally {
      waitingNext.current = false;
    }
  };
  const next = async (numericIntent = false) => {
    if (!selected) return;
    const item = selected;
    const savedDraft = await waitForScore(item, numericIntent);
    if (savedDraft === false || !acquire()) return;
    const index = items.findIndex(
      (row) => gradingItemKey(row) === gradingItemKey(item),
    );
    const following =
      items[index + 1] ??
      items.find(
        (row) =>
          gradingItemKey(row) !== gradingItemKey(item) &&
          queue.data?.items.some(
            (pending) => gradingItemKey(pending) === gradingItemKey(row),
          ),
      );
    try {
      const nextDraft = savedDraft ?? draftFor(item);
      if (nextDraft.points === null) {
        const count = scoreOptions(item.points).length;
        const message = t(count ? "grading.pickFirstKeys" : "grading.pickFirst", {
          n: count,
        });
        setError(message);
        toast.error(message);
        return;
      }
      const saved = await flush(item, nextDraft);
      if (!current()) return;
      const pending = await pendingAfterSave(item, saved?.pendingManual);
      if (!current()) return;
      if (pending === 0) {
        setFinishReady(item);
        return;
      } else if (pending === undefined) {
        setError(t("grading.pendingUnknown"));
        return;
      }
      if (current() && following) setPicked(gradingItemKey(following));
    } catch (cause) {
      if (current()) setError(notify(cause));
    } finally {
      release();
    }
  };
  const confirmFinish = async () => {
    if (!finishReady || !acquire()) return;
    const item = finishReady;
    try {
      const pending = await pendingAfterSave(item, undefined);
      if (!current()) return;
      setFinishReady(null);
      if (pending !== 0) {
        setError(t("grading.pendingUnknown"));
        return;
      }
      try {
        await finish(item);
      } catch (cause) {
        rememberFinish(item, cause);
      }
    } catch (cause) {
      if (current()) setError(notify(cause));
    } finally {
      release();
    }
  };
  const retryFinish = async (state: FinishState) => {
    if (!acquire()) return;
    try {
      await finish(state.item);
    } catch (cause) {
      rememberFinish(state.item, cause);
    } finally {
      release();
    }
  };
  const selectCandidate = async (row: AttemptListRow) => {
    if (!acquire()) return;
    try {
      if (selected) await flush(selected);
      if (current()) {
        setFinishReady(null);
        setCandidate(row);
      }
    } catch (cause) {
      if (current()) setError(notify(cause));
    } finally {
      release();
    }
  };
  const finishCandidate = async () => {
    if (!candidate || !acquire()) return;
    const item = {
      attemptId: candidate.id,
      assignmentId: candidate.assignmentId,
      studentId: candidate.studentId,
    };
    try {
      const fresh = await getAttemptForReview(item.attemptId, controller.signal);
      if (!current()) return;
      if (
        fresh.attempt.id !== item.attemptId ||
        fresh.attempt.assignmentId !== item.assignmentId ||
        fresh.attempt.studentId !== item.studentId ||
        fresh.attempt.score?.pendingManual !== 0 ||
        !["submitted", "timed_out"].includes(fresh.attempt.status)
      ) {
        setError(t("grading.pendingUnknown"));
        return;
      }
      try {
        await finish(item);
      } catch (cause) {
        rememberFinish(item, cause);
      }
    } catch (cause) {
      if (current()) setError(notify(cause));
    } finally {
      release();
    }
  };
  const savedCount = items.filter(
    (item) => item.score !== null || drafts[gradingItemKey(item)]?.acknowledged != null,
  ).length;
  return {
    queue,
    recovery,
    items,
    selected,
    draft,
    savedCount,
    busy,
    error,
    finishes,
    finishReady,
    completed,
    candidate,
    candidateReview,
    pickScore,
    comment,
    select,
    leave,
    move,
    next,
    confirmFinish,
    retryFinish,
    selectCandidate,
    finishCandidate,
  };
}
