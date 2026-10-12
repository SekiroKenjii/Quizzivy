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
  gradingItemKey,
  mergeQueueOrder,
  nextOpenItem,
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

function ungradedHere(draft: GradingDraft | undefined, item: GradingQueueItem) {
  return (
    !draft ||
    (draft.acknowledged === null &&
      draft.points === null &&
      draft.comment.trim() === (item.comment ?? "").trim())
  );
}

function vanishedOpen(
  open: GradingQueueItem | null,
  fresh: readonly GradingQueueItem[] | undefined,
  held: ReadonlyMap<string, GradingQueueItem>,
  drafts: Record<string, GradingDraft>,
) {
  if (!open || !fresh) return null;
  const key = gradingItemKey(open);
  if (held.has(key) || fresh.some((item) => gradingItemKey(item) === key)) return null;
  return ungradedHere(drafts[key], open) ? open : null;
}

function useOpenAnswerSync(
  generation: number,
  heldOpen: GradingQueueItem | null,
  fetchedAt: number,
  drafts: Record<string, GradingDraft>,
  setDrafts: (next: Record<string, GradingDraft>) => void,
) {
  const sync = useQuery({
    queryKey: [
      "teacher-grading-open",
      generation,
      heldOpen?.attemptId,
      heldOpen?.questionId,
      fetchedAt,
    ],
    queryFn: ({ signal }) => getAttemptForReview(heldOpen!.attemptId, signal),
    enabled: heldOpen !== null,
    retry: false,
  });
  if (!heldOpen || sync.data?.attempt.id !== heldOpen.attemptId) return;
  const points = sync.data.answers[heldOpen.questionId]?.manualScore;
  if (points == null) return;
  const remark = (sync.data.answers[heldOpen.questionId]?.graderComment ?? "").trim();
  setDrafts({
    ...drafts,
    [gradingItemKey(heldOpen)]: {
      item: heldOpen,
      points,
      comment: remark,
      acknowledged: { points, comment: remark },
    },
  });
}

function commentUnsettled(item: GradingQueueItem, draft: GradingDraft | undefined) {
  if (!draft) return false;
  const comment = draft.comment.trim();
  const saved = draft.acknowledged
    ? draft.acknowledged.comment
    : (item.comment ?? "").trim();
  return comment !== saved || (draft.points === null && comment !== "");
}

/**
 * useGradingQueue serializes explicit marks and Finish actions under the
 * mounted actor and filter owner, and reports through `unsettled` a comment
 * on the selected answer that no Grade has saved. It sets state during render
 * in three places, and each converges because its own write turns off the
 * condition it tests: new queue data is recorded in `order` (and a vanished
 * open answer added to `held`, which `vanishedOpen` then skips); `open`
 * stores the selected item it is compared with; and the open-answer sync
 * writes an acknowledged draft, after which `ungradedHere` is false. A change
 * to `ungradedHere` or `vanishedOpen` must keep that, or the render loops.
 */
export function useGradingQueue(params: GradingQueueParams) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [actor] = useState(authStore.captureActor);
  const controller = useRef<AbortController | null>(null);
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
  const [open, setOpen] = useState<GradingQueueItem | null>(null);
  const [held, setHeld] = useState<ReadonlyMap<string, GradingQueueItem>>(new Map());
  const mode = params.mode ?? "student";
  const assignment = params.assignmentId ?? "";
  const student = params.studentId ?? "";
  const signal = () => controller.current?.signal ?? AbortSignal.abort();
  const current = () => {
    const user = useAuthStore.getState().user;
    return (
      controller.current !== null &&
      !controller.current.signal.aborted &&
      authStore.isCurrent(actor) &&
      can(user, "teaching.grading") &&
      hasWorkspace(user, "teacher")
    );
  };
  useEffect(() => {
    const live = new AbortController();
    controller.current = live;
    return () => live.abort();
  }, []);
  const queue = useQuery({
    queryKey: gradingKey(actor.generation, mode, assignment, student),
    queryFn: ({ signal }) => listGradingQueue(params, signal),
    retry: false,
    refetchOnWindowFocus: "always",
  });
  const [order, setOrder] = useState<{
    data: typeof queue.data;
    keys: readonly string[];
  }>({ data: undefined, keys: [] });
  if (queue.data !== order.data) {
    setOrder({
      data: queue.data,
      keys: mergeQueueOrder(order.keys, (queue.data?.items ?? []).map(gradingItemKey)),
    });
    const vanished = vanishedOpen(open, queue.data?.items, held, drafts);
    if (vanished) setHeld(new Map(held).set(gradingItemKey(vanished), vanished));
  }
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
    const shown = new Map<string, GradingQueueItem>();
    for (const item of queue.data?.items ?? [])
      if (!completed.has(item.attemptId)) shown.set(gradingItemKey(item), item);
    for (const draft of Object.values(drafts)) {
      const key = gradingItemKey(draft.item);
      if (!completed.has(draft.item.attemptId) && !shown.has(key))
        shown.set(key, draft.item);
    }
    for (const [key, item] of held)
      if (!completed.has(item.attemptId) && !shown.has(key)) shown.set(key, item);
    return order.keys.flatMap((key) => {
      const item = shown.get(key);
      return item ? [item] : [];
    });
  }, [queue.data, drafts, held, completed, order.keys]);
  const selected = items.find((item) => gradingItemKey(item) === picked) ?? items[0];
  const selectedKey = selected ? gradingItemKey(selected) : null;
  if (selectedKey !== (open ? gradingItemKey(open) : null)) setOpen(selected ?? null);
  const heldOpen =
    selected &&
    held.has(gradingItemKey(selected)) &&
    ungradedHere(drafts[gradingItemKey(selected)], selected)
      ? selected
      : null;
  useOpenAnswerSync(actor.generation, heldOpen, queue.dataUpdatedAt, drafts, setDrafts);
  const draft = selected ? drafts[gradingItemKey(selected)] : undefined;
  const unsettled = selected ? commentUnsettled(selected, draft) : false;
  const notify = (cause: unknown) => failureMessage(cause, t("grading.saveFailed"));
  const ungraded = (item: GradingQueueItem) =>
    item.score === null && drafts[gradingItemKey(item)]?.acknowledged == null;
  const indexOf = (item: Pick<GradingQueueItem, "attemptId" | "questionId">) =>
    items.findIndex((row) => gradingItemKey(row) === gradingItemKey(item));
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
      signal(),
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
    await finishGrading(item.attemptId, signal());
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
    const review = await getAttemptForReview(item.attemptId, signal());
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
    const index = indexOf(item);
    const following =
      nextOpenItem(
        items,
        index,
        (row) => gradingItemKey(row) !== gradingItemKey(item) && ungraded(row),
      ) ??
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
    const after = nextOpenItem(
      items,
      selected ? indexOf(selected) : -1,
      (row) => row.attemptId !== item.attemptId && ungraded(row),
    );
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
        if (current() && after) setPicked(gradingItemKey(after));
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
      const fresh = await getAttemptForReview(item.attemptId, signal());
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
    unsettled,
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
