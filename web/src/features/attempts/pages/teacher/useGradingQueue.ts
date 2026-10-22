import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
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
import { gradeFieldMessage } from "../../gradeErrors";
import {
  markFields,
  markOf,
  markPoints,
  markSignature,
  markStarted,
  type EssayMark,
} from "../../essayMark";
import {
  gradingItemKey,
  groupReadyPapers,
  mergeQueueOrder,
  nextOpenItem,
  scanGradingCandidates,
  scoreOptions,
} from "./gradingRecovery";

/**
 * GradingDraft retains the acknowledged mark separately from the current
 * comment and score intent; `filled` marks a score laid by `fill` rather than
 * by a score button, which writes at once. An essay's `mark` holds its band or
 * its criteria; `points` is the score that mark derives, null until it is
 * complete, and `acknowledged.mark` the saved mark's signature.
 */
export interface GradingDraft {
  item: GradingQueueItem;
  points: number | null;
  comment: string;
  acknowledged: { points: number; comment: string; mark?: string | undefined } | null;
  filled?: boolean;
  mark?: EssayMark;
}

/** FieldError is a refused save's message for the marks of one answer, by its item key. */
export interface FieldError {
  key: string;
  message: string;
}

const essay = (item: GradingQueueItem) => item.type === "essay";
const rubricOf = (item: GradingQueueItem) => item.rubric ?? [];
const signature = (item: GradingQueueItem, draft: GradingDraft) =>
  essay(item) && draft.mark ? markSignature(rubricOf(item), draft.mark) : undefined;
const halfMarked = (item: GradingQueueItem, draft: GradingDraft | undefined) =>
  essay(item) && draft?.mark !== undefined && markStarted(rubricOf(item), draft.mark);

/** FinishState retains explicit completion recovery independently of pending queue membership. */
export interface FinishState {
  item: Pick<
    GradingQueueItem,
    "attemptId" | "assignmentId" | "assignmentTitle" | "studentId" | "studentName"
  >;
  error: string;
}

/** CheckFinish is what a Finish from a ready paper's check ended with: the paper whose check opens next, or null after the last. */
export interface CheckFinish {
  next: AttemptListRow | null;
}

/** markFirst is what Save & next says when the answer has no mark yet. */
function markFirst(item: GradingQueueItem, t: TFunction) {
  if (essay(item))
    return t(
      rubricOf(item).length ? "grading.pickCriteriaFirst" : "grading.pickBandFirst",
    );
  const count = scoreOptions(item.points).length;
  return t(count ? "grading.pickFirstKeys" : "grading.pickFirst", { n: count });
}

function ungradedHere(draft: GradingDraft | undefined, item: GradingQueueItem) {
  return (
    !draft ||
    (draft.acknowledged === null &&
      draft.points === null &&
      !halfMarked(item, draft) &&
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
  const answer = sync.data.answers[heldOpen.questionId];
  const points = answer?.manualScore;
  if (points == null) return;
  const remark = (answer?.graderComment ?? "").trim();
  const draft: GradingDraft = {
    item: heldOpen,
    points,
    comment: remark,
    acknowledged: { points, comment: remark },
  };
  if (essay(heldOpen)) {
    draft.mark = markOf(answer ?? {});
    draft.acknowledged = { ...draft.acknowledged!, mark: signature(heldOpen, draft) };
  }
  setDrafts({ ...drafts, [gradingItemKey(heldOpen)]: draft });
}

function commentUnsettled(item: GradingQueueItem, draft: GradingDraft | undefined) {
  if (!draft) return false;
  const comment = draft.comment.trim();
  const saved = draft.acknowledged
    ? draft.acknowledged.comment
    : (item.comment ?? "").trim();
  return comment !== saved || (draft.points === null && comment !== "");
}

/** markUnsaved says an essay's complete mark is not the one its last save wrote, as after a refused save. */
function markUnsaved(item: GradingQueueItem, draft: GradingDraft | undefined) {
  if (!essay(item) || !draft?.mark || draft.points === null) return false;
  return signature(item, draft) !== draft.acknowledged?.mark;
}

function scoreUnsettled(item: GradingQueueItem, draft: GradingDraft | undefined) {
  if (halfMarked(item, draft) || markUnsaved(item, draft)) return true;
  if (!draft?.filled || draft.points === null) return false;
  return draft.points !== (draft.acknowledged?.points ?? item.score);
}

/**
 * useGradingQueue serializes explicit marks and Finish actions under the
 * mounted actor and filter owner, and reports through `unsettled` a comment,
 * or a score `fill` laid, on the selected answer that no Grade has saved, and
 * through `unsavedScore` whether that score is among it. `readyGroups` holds
 * the papers waiting for an explicit Finish in the order the card lists them;
 * `candidate` is the one whose check is open, and a Finish from it opens the
 * next one, toasts, and sets `announcement` for a polite live region;
 * `finished` lists the papers finished here, whose names outlive the reads
 * that held them. `fill`
 * lays a score and a comment over an answer's draft and saves nothing: Save &
 * next, or the flush a move makes, writes them as it writes a typed comment.
 * It sets state
 * during render in three places, and each converges because its own write
 * turns off the condition it tests: new queue data is recorded in `order`
 * (and a vanished open answer added to `held`, which `vanishedOpen` then
 * skips); `open` stores the selected item it is compared with; and the
 * open-answer sync writes an acknowledged draft, after which `ungradedHere`
 * is false. A change to `ungradedHere` or `vanishedOpen` must keep that, or
 * the render loops.
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
  const [fieldError, setFieldError] = useState<FieldError | null>(null);
  const queuedMark = useRef<{ item: GradingQueueItem; mark: EssayMark } | null>(null);
  const [heldLeave, setHeldLeave] = useState<(() => void) | null>(null);
  const [drafts, setDrafts] = useState<Record<string, GradingDraft>>({});
  const [finishes, setFinishes] = useState<Record<string, FinishState>>({});
  const [finishReady, setFinishReady] = useState<FinishState["item"] | null>(null);
  const [completed, setCompleted] = useState<ReadonlySet<string>>(new Set());
  const [finished, setFinished] = useState<readonly FinishState["item"][]>([]);
  const [picked, setPicked] = useState<string | null>(null);
  const [candidate, setCandidate] = useState<AttemptListRow | null>(null);
  const [checkFailure, setCheckFailure] = useState<string | null>(null);
  const [announcement, setAnnouncement] = useState("");
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
  const candidateKey = (id: string | undefined) => [
    "teacher-grading-candidate",
    actor.generation,
    id,
  ];
  const candidateReview = useQuery({
    queryKey: candidateKey(candidate?.id),
    queryFn: ({ signal }) => getAttemptForReview(candidate!.id, signal),
    enabled: candidate !== null,
    retry: false,
  });
  const readyGroups = useMemo(
    () =>
      groupReadyPapers((recovery.data ?? []).filter((row) => !completed.has(row.id))),
    [recovery.data, completed],
  );
  const readyPapers = useMemo(
    () => readyGroups.flatMap((group) => group.rows),
    [readyGroups],
  );
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
  const partial = Object.values(drafts).some((entry) => halfMarked(entry.item, entry));
  const unsavedScore = partial || (selected ? scoreUnsettled(selected, draft) : false);
  const unsettled =
    unsavedScore || (selected ? commentUnsettled(selected, draft) : false);
  const notify = (cause: unknown) => failureMessage(cause, t("grading.saveFailed"));
  const report = (cause: unknown, item: GradingQueueItem | undefined) => {
    const field = item ? gradeFieldMessage(cause, item.questionId, t) : null;
    if (field !== null && item)
      setFieldError({ key: gradingItemKey(item), message: field });
    else setError(notify(cause));
  };
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
      ["teacher-attempts"],
      ["teacher-answers-by-question", item.assignmentId],
      ["teacher-dashboard"],
    ])
      void client.invalidateQueries({ queryKey });
  };
  const acquire = () => {
    if (!current() || lock.current) return false;
    lock.current = true;
    setBusy(true);
    setError(null);
    setFieldError(null);
    return true;
  };
  const release = () => {
    lock.current = false;
    if (current()) setBusy(false);
    const queued = queuedMark.current;
    queuedMark.current = null;
    if (queued && current()) void pickBand(queued.item, queued.mark, false);
  };
  const draftFor = (item: GradingQueueItem): GradingDraft => {
    const saved = drafts[gradingItemKey(item)];
    if (saved) return saved;
    const draft: GradingDraft = {
      item,
      points: item.score,
      comment: item.comment ?? "",
      acknowledged:
        item.score === null
          ? null
          : { points: item.score, comment: item.comment ?? "" },
    };
    if (essay(item)) {
      draft.mark = markOf(item);
      if (draft.acknowledged)
        draft.acknowledged = { ...draft.acknowledged, mark: signature(item, draft) };
    }
    return draft;
  };
  const acknowledge = (item: GradingQueueItem, next: GradingDraft) => ({
    points: next.points!,
    comment: next.comment.trim(),
    ...(essay(item) ? { mark: signature(item, next) } : {}),
  });
  const write = async (item: GradingQueueItem, next: GradingDraft) => {
    if (next.points === null || !current()) return null;
    const marks =
      essay(item) && next.mark
        ? markFields(rubricOf(item), next.mark)
        : { points: next.points };
    const score = await gradeAttempt(
      item.attemptId,
      [
        {
          questionId: item.questionId,
          ...marks,
          comment: next.comment.trim() || null,
        },
      ],
      signal(),
    );
    if (!current()) return null;
    setDrafts((previous) => ({
      ...previous,
      [gradingItemKey(item)]: { ...next, acknowledged: acknowledge(item, next) },
    }));
    invalidate(item, false);
    return score;
  };
  const flush = async (item: GradingQueueItem, next = draftFor(item)) => {
    if (next.points === null) return null;
    const ack = next.acknowledged;
    if (
      ack?.points === next.points &&
      ack.comment === next.comment.trim() &&
      ack.mark === signature(item, next)
    )
      return null;
    return write(item, next);
  };
  const pickScore = async (item: GradingQueueItem, points: number) => {
    if (
      essay(item) ||
      !Number.isFinite(points) ||
      points < 0 ||
      points > item.points ||
      !acquire()
    )
      return;
    setFinishReady(null);
    const next = { ...draftFor(item), points, filled: false };
    setDrafts((previous) => ({ ...previous, [gradingItemKey(item)]: next }));
    setPicked(gradingItemKey(item));
    await save(item, next);
  };
  const save = async (item: GradingQueueItem, next: GradingDraft) => {
    const result = (async () => {
      try {
        const saved = await write(item, next);
        return saved && current()
          ? { ...next, acknowledged: acknowledge(item, next) }
          : null;
      } catch (cause) {
        if (current()) report(cause, item);
        return null;
      } finally {
        release();
      }
    })();
    scoreSave.current = { key: gradingItemKey(item), result };
    await result;
  };
  /**
   * pickBand sets an essay's band or one of its criteria. A complete mark
   * saves at once, as a score button does; a rubric still missing a criterion
   * waits in the draft, which `unsavedScore` reports, and saves nothing. A
   * mark given while a save is in flight shows at once and waits for that
   * save, and only the last one waiting is saved, so a quick correction is
   * kept. `follow` selects the answer; a waiting mark saved after a move
   * leaves the selection where the move put it.
   */
  const pickBand = async (item: GradingQueueItem, mark: EssayMark, follow = true) => {
    if (!essay(item) || !current()) return;
    const key = gradingItemKey(item);
    const points = markPoints(rubricOf(item), mark, item.points);
    const next = { ...draftFor(item), mark, points, filled: false };
    setDrafts((previous) => ({
      ...previous,
      [key]: { ...(previous[key] ?? next), mark, points, filled: false },
    }));
    if (lock.current) {
      queuedMark.current = { item, mark };
      return;
    }
    setFinishReady(null);
    setFieldError(null);
    if (follow) setPicked(key);
    if (points === null || !acquire()) return;
    await save(item, next);
  };
  const comment = (item: GradingQueueItem, value: string) => {
    if (!current() || lock.current) return;
    setFinishReady(null);
    setDrafts((previous) => ({
      ...previous,
      [gradingItemKey(item)]: { ...draftFor(item), comment: value },
    }));
  };
  const fill = (item: GradingQueueItem, points: number, value: string) => {
    if (
      essay(item) ||
      !Number.isFinite(points) ||
      points < 0 ||
      points > item.points ||
      !current() ||
      lock.current
    )
      return false;
    setFinishReady(null);
    setDrafts((previous) => ({
      ...previous,
      [gradingItemKey(item)]: {
        ...draftFor(item),
        points,
        comment: value,
        filled: true,
      },
    }));
    setPicked(gradingItemKey(item));
    return true;
  };
  const select = async (item: GradingQueueItem) => {
    if (!acquire()) return;
    try {
      if (selected) await flush(selected);
      if (current()) {
        setPicked(gradingItemKey(item));
        setFinishReady(null);
      }
    } catch (cause) {
      if (current()) report(cause, selected);
    } finally {
      release();
    }
  };
  /** commentWithoutScore is why an answer with a comment but no score cannot be saved, or null. */
  const commentWithoutScore = (item: GradingQueueItem) => {
    const next = draftFor(item);
    if (next.points !== null || next.comment.trim() === (item.comment ?? "").trim())
      return null;
    return t(essay(item) ? "grading.pickBandFirst" : "grading.pickFirst");
  };
  /**
   * leave saves the open answer and runs `action`. While any essay holds a
   * part-marked rubric, which no save can write, it holds `action` in
   * `heldLeave` instead, for the page to ask whether to `discard` those bands.
   */
  const leave = async (action: () => void) => {
    if (partial) {
      if (current() && !lock.current) setHeldLeave(() => action);
      return;
    }
    if (!acquire()) return;
    try {
      const unscored = selected ? commentWithoutScore(selected) : null;
      if (unscored !== null) {
        setError(unscored);
        return;
      }
      if (selected) await flush(selected);
      if (current()) action();
    } catch (cause) {
      if (current()) report(cause, selected);
    } finally {
      release();
    }
  };
  /** discard drops every part-marked rubric's bands, saves nothing, and runs the held leave and `action`. */
  const discard = (action?: () => void) => {
    if (!current() || lock.current) return;
    const held = heldLeave;
    setHeldLeave(null);
    setError(null);
    setDrafts((previous) =>
      Object.fromEntries(
        Object.entries(previous).filter(([, entry]) => !halfMarked(entry.item, entry)),
      ),
    );
    held?.();
    action?.();
  };
  /** keepMarking closes the leave question and keeps the part-marked rubric. */
  const keepMarking = () => setHeldLeave(null);
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
    setFinished((previous) => [...previous, item]);
    setFinishes((previous) => {
      const next = { ...previous };
      delete next[item.attemptId];
      return next;
    });
    setCandidate((open) => (open?.id === item.attemptId ? null : open));
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
        const message = markFirst(item, t);
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
      if (current()) report(cause, item);
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
  const openCheck = (row: AttemptListRow | null) => {
    if (!current()) return;
    setCheckFailure(null);
    setCandidate(row);
  };
  const finishCandidate = async (): Promise<CheckFinish | null> => {
    if (!candidate || !acquire()) return null;
    const row = candidate;
    const item = {
      attemptId: row.id,
      assignmentId: row.assignmentId,
      assignmentTitle: row.testTitle,
      studentId: row.studentId,
      studentName: row.studentName,
    };
    const index = readyPapers.findIndex((paper) => paper.id === row.id);
    const following = nextOpenItem(readyPapers, index, (paper) => paper.id !== row.id);
    setCheckFailure(null);
    try {
      const fresh = await getAttemptForReview(item.attemptId, signal());
      if (!current()) return null;
      if (
        fresh.attempt.id !== item.attemptId ||
        fresh.attempt.assignmentId !== item.assignmentId ||
        fresh.attempt.studentId !== item.studentId
      ) {
        setError(t("grading.pendingUnknown"));
        return null;
      }
      if (
        fresh.attempt.score?.pendingManual !== 0 ||
        !["submitted", "timed_out"].includes(fresh.attempt.status)
      ) {
        client.setQueryData(candidateKey(item.attemptId), fresh);
        return null;
      }
      await finish(item);
      if (!current()) return null;
      setCandidate(following);
      toast(t("grading.finishedToast", { name: row.studentName }));
      setAnnouncement(
        following
          ? t("grading.finishedNext", {
              name: row.studentName,
              next: following.studentName,
            })
          : t("grading.finishedToast", { name: row.studentName }),
      );
      return { next: following };
    } catch {
      if (current()) setCheckFailure(item.attemptId);
      return null;
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
    unsavedScore,
    partial,
    leaveHeld: heldLeave !== null,
    discard,
    keepMarking,
    savedCount,
    busy,
    error,
    fieldError,
    finishes,
    finishReady,
    completed,
    finished,
    candidate,
    candidateReview,
    checkFailure,
    announcement,
    readyGroups,
    readyPapers,
    pickScore,
    pickBand,
    comment,
    fill,
    select,
    leave,
    move,
    next,
    confirmFinish,
    retryFinish,
    openCheck,
    finishCandidate,
  };
}

/** GradingQueueModel is what useGradingQueue returns. */
export type GradingQueueModel = ReturnType<typeof useGradingQueue>;
