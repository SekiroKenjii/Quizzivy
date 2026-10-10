import {
  useCallback,
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { useTranslation } from "react-i18next";
import {
  Link,
  useBeforeUnload,
  useBlocker,
  useLocation,
  useNavigate,
  useParams,
} from "react-router";
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  CircleStop,
  ExternalLink,
  FileText,
  Pencil,
  RotateCcw,
  Send,
  SquarePen,
} from "lucide-react";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { RowMenu } from "@/components/shared/RowMenu";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { StatStrip } from "@/components/shared/stats/StatStrip";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { can, hasWorkspace, useCan, useWorkspace } from "@/features/auth/permissions";
import {
  getMonitor,
  setAttemptNote,
  type AttemptReview,
  type MonitorRow,
} from "@/features/attempts/api";
import { AttemptSheet } from "@/features/attempts/components/AttemptSheet";
import { Monitor } from "@/features/attempts/components/Monitor";
import { SheetNoteController } from "@/features/attempts/components/sheetNotes";
import { POLL_MS, monitorKey, reviewKey } from "@/features/attempts/keys";
import { FLAGGED } from "@/features/integrity/tones";
import { listVersions, previewTest, type TestVersion } from "@/features/tests/api";
import { StudentPreviewPane } from "@/features/tests/components/StudentPreviewPane";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { PageHead } from "@/layouts/shell/PageHead";
import { useCrumbs } from "@/layouts/shell/crumbs";
import { ApiError } from "@/lib/api/errors";
import { formatMoment, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useAuthStore } from "@/stores/auth";
import { getAssignment, updateAssignment, type Assignment } from "../../api";
import { CloseEarlyDialog } from "../../components/CloseEarlyDialog";
import { ReopenDialog } from "../../components/ReopenDialog";
import { ReopenMenu, type ReopenChoice } from "../../components/ReopenMenu";
import { ItemAnalysis } from "../../components/ItemAnalysis";
import { ReopenStudentDialog } from "../../components/ReopenStudentDialog";
import { SettingsGroups } from "../../components/SettingsGroups";
import { keepsTimeAfter } from "../../overrides";
import { toInput } from "../../input";
import { statusAt } from "../../status";
import { assignmentStats, detailTab, firstPendingPaper } from "./assignmentDetail";
import { assignmentDetailLocation } from "./assignmentDetailUrl";

/** AssignmentDetailPage owns the assignment monitor and preserves authorized actions and note drafts across its URL-controlled tabs and sheet. */
export default function AssignmentDetailPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const { id = "" } = useParams<{ id: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const params = new URLSearchParams(location.search);
  const tab = detailTab(params.get("tab"));
  const attemptId = params.get("attempt");
  const workspace = useWorkspace("teacher");
  const write = useCan("teaching.assignments.write");
  const client = useQueryClient();
  const [closing, setClosing] = useState(false);
  const [reopening, setReopening] = useState<ReopenChoice | null>(null);
  const [reopeningStudent, setReopeningStudent] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const { notes, recoveryPanel, noteFocusRequest } = useSheetDrafts();

  const { assignment, a, now, status, monitor, version, preview } = useAssignmentReads(
    id,
    workspace,
    tab,
  );
  const [sheetIdentity, setSheetIdentity] = useState<{
    assignmentId: string;
    attemptId: string;
  } | null>(null);
  const hostReady =
    workspace && assignment.isSuccess && a !== undefined && status !== null;
  if (!hostReady && sheetIdentity !== null) setSheetIdentity(null);
  else if (
    hostReady &&
    attemptId &&
    (sheetIdentity?.assignmentId !== id || sheetIdentity.attemptId !== attemptId)
  )
    setSheetIdentity({ assignmentId: id, attemptId });
  else if (!attemptId && sheetIdentity !== null && sheetIdentity.assignmentId !== id)
    setSheetIdentity(null);
  const sheetAttempt =
    attemptId || (sheetIdentity?.assignmentId === id ? sheetIdentity.attemptId : null);
  useCrumbs(a === undefined ? null : [{ label: a.testTitle }]);
  const refresh = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: ["admin-assignment", id] }),
      client.invalidateQueries({ queryKey: ["admin-assignments"] }),
      client.invalidateQueries({ queryKey: ["admin-dashboard"] }),
      client.invalidateQueries({ queryKey: monitorKey(id) }),
    ]);
  };
  const publish = useMutation({
    mutationFn: (value: Assignment) => {
      if (!write || value.id !== id)
        throw new Error("Assignment write is not permitted");
      return updateAssignment(value.id, { ...toInput(value), draft: false });
    },
    onSuccess: refresh,
    onError: (error) =>
      setFailure(
        error instanceof ApiError
          ? error.message
          : t("assignments.detail.publishFailed"),
      ),
  });
  const close = useMutation({
    mutationFn: (value: Assignment) => {
      if (!write || value.id !== id || statusAt(value, new Date()) !== "open")
        throw new Error("Assignment cannot close");
      return updateAssignment(value.id, {
        ...toInput(value),
        draft: false,
        closeNow: true,
      });
    },
    onSuccess: async () => {
      setClosing(false);
      await refresh();
    },
  });
  if (!workspace)
    return (
      <>
        {recoveryPanel}
        <EmptyState>{t("assignmentDetail.unavailable")}</EmptyState>
      </>
    );
  if (assignment.isPending)
    return (
      <>
        {recoveryPanel}
        <ListSkeleton rows={8} />
      </>
    );
  if (assignment.isError)
    return (
      <>
        {recoveryPanel}
        <LoadError error={assignment.error} onRetry={() => void assignment.refetch()}>
          {t("assignments.detail.loadFailed")}
        </LoadError>
      </>
    );
  if (a === undefined || status === null)
    return (
      <>
        {recoveryPanel}
        <EmptyState
          action={
            <Button variant="outline" asChild>
              <Link to="/teacher/assignments">
                {t("assignments.detail.backToList")}
              </Link>
            </Button>
          }
        >
          {t("assignments.detail.notFound")}
        </EmptyState>
      </>
    );
  const stats = monitor.isSuccess ? assignmentStats(monitor.data.rows) : null;
  const eligible = monitor.isSuccess ? firstPendingPaper(monitor.data.rows) : undefined;
  const openAttempt = (selected: string) =>
    navigate(assignmentDetailLocation(location, { attempt: selected }));
  return (
    <div className="flex min-w-0 flex-col gap-4.5">
      <PageHead
        title={a.testTitle}
        status={<StatusBadge kind="assignment" status={status} />}
        className="items-start! [&_h1]:text-balance"
        actions={
          <>
            <WriteActions
              a={a}
              status={status}
              now={now}
              pending={publish.isPending}
              onPublish={() => publish.mutate(a)}
              onClose={() => setClosing(true)}
              onReopen={setReopening}
              onEditSettings={() =>
                void navigate(assignmentDetailLocation(location, { tab: "settings" }), {
                  replace: true,
                })
              }
              onReopenStudent={() => setReopeningStudent(true)}
            />
            <GradeAction
              count={stats?.pending ?? 0}
              attemptId={eligible?.attemptId}
              onOpen={(selected) => void openAttempt(selected)}
            />
          </>
        }
      >
        <AssignmentMeta a={a} status={status} />
      </PageHead>
      {failure !== null && (
        <p role="alert" className="text-danger-ink text-sm">
          {failure}
        </p>
      )}
      <DraftHint a={a} status={status} />
      {recoveryPanel !== null && !attemptId && recoveryPanel}
      <DetailStats stats={stats} />
      <Tabs
        value={tab}
        onValueChange={(value) =>
          void navigate(assignmentDetailLocation(location, { tab: value }), {
            replace: true,
          })
        }
      >
        <TabsList aria-label={t("assignmentDetail.tabsLabel")}>
          {(["students", "questions", "settings"] as const).map((value) => (
            <TabsTrigger key={value} value={value}>
              {t(`assignmentDetail.tabs.${value}`)}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="students" className="pt-4.5">
          <StudentsPanel
            monitor={monitor}
            a={a}
            attemptId={attemptId}
            onOpen={(selected) => void openAttempt(selected)}
            onRefresh={refresh}
          />
        </TabsContent>
        <TabsContent value="questions" className="space-y-4 pt-4.5">
          <ItemAnalysis assignmentId={a.id} />
          <TestCard a={a} version={version} closed={status === "closed"} />
          <QuestionsPanel preview={preview} a={a} />
        </TabsContent>
        <TabsContent value="settings" className="pt-4.5">
          <SettingsGroups a={a} version={version} status={status} />
        </TabsContent>
      </Tabs>
      <StateDialogs
        a={a}
        status={status}
        closing={closing}
        reopening={reopening}
        pending={close.isPending}
        failed={close.isError}
        rows={monitor.data?.rows ?? null}
        now={now}
        reopeningStudent={reopeningStudent}
        onReopeningStudent={setReopeningStudent}
        onClosing={setClosing}
        onReopening={setReopening}
        onClose={() => {
          if (write && status === "open") close.mutate(a);
        }}
        onRefresh={refresh}
      />
      {sheetAttempt && (
        <AttemptSheet
          assignment={a}
          open={Boolean(attemptId)}
          attemptId={sheetAttempt}
          row={monitor.data?.rows.find((row) => row.attemptId === sheetAttempt)}
          questionCount={monitor.data?.questionCount ?? 0}
          serverTime={
            monitor.data?.serverTime ?? new Date(monitor.dataUpdatedAt).toISOString()
          }
          receivedAt={monitor.dataUpdatedAt}
          notes={notes}
          noteFocusRequest={noteFocusRequest}
          onClose={() => {
            void navigate(assignmentDetailLocation(location, { attempt: null }), {
              replace: true,
            });
          }}
          recovery={recoveryPanel}
        />
      )}
    </div>
  );
}

function useAssignmentReads(
  id: string,
  workspace: boolean,
  tab: ReturnType<typeof detailTab>,
) {
  const assignment = useQuery({
    queryKey: ["admin-assignment", id],
    queryFn: ({ signal }) => getAssignment(id, signal),
    enabled: workspace && id !== "",
  });
  const a = workspace && assignment.data?.id === id ? assignment.data : undefined;
  const now = new Date();
  const status = a === undefined ? null : statusAt(a, now);
  const live = status === "open" && assignment.isSuccess;
  const interval = useIdlePolling(POLL_MS, live);
  const monitor = useQuery({
    queryKey: monitorKey(id),
    queryFn: ({ signal }) => getMonitor(id, signal),
    enabled: a !== undefined && assignment.isSuccess,
    refetchInterval: (query) => (query.state.status === "error" ? false : interval),
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(monitor.refetch, live && monitor.isSuccess);
  const testId = a?.testId;
  const versions = useQuery({
    queryKey: ["admin-test-versions", testId],
    queryFn: ({ signal }) => listVersions(testId ?? "", signal),
    enabled: a !== undefined && testId !== undefined,
  });
  const version = versions.data?.items.find((v) => v.id === a?.testVersionId);
  const preview = useQuery({
    queryKey: ["admin-test-preview", testId, a?.testVersion],
    queryFn: ({ signal }) => previewTest(testId ?? "", a?.testVersion, signal),
    enabled: a !== undefined && tab === "questions",
  });
  return { assignment, a, now, status, monitor, version, preview };
}

function AssignmentMeta({
  a,
  status,
}: Readonly<{ a: Assignment; status: ReturnType<typeof statusAt> }>) {
  const { t } = useTranslation();
  return (
    <p className="text-muted-fg text-ui mt-1">
      {a.targets.classes.map((value) => value.name).join(", ")} ·{" "}
      {formatMoment(
        status === "scheduled"
          ? a.window.opensAt
          : (a.window.closedAt ?? a.window.closesAt),
      )}{" "}
      · {t("assignments.minutes", { count: a.durationMinutes })}
    </p>
  );
}

function DraftHint({
  a,
  status,
}: Readonly<{ a: Assignment; status: ReturnType<typeof statusAt> }>) {
  const { t } = useTranslation();
  if (status !== "draft") return null;
  return (
    <p className="text-muted-fg text-sm">
      {t("assignmentDetail.draftHint")}
      {(a.targetCount ?? 0) === 0 ? ` ${t("assignments.needTargets")}` : ""}
    </p>
  );
}

function useSheetDrafts() {
  const client = useQueryClient();
  const [recovery, setRecovery] = useState(false);
  const [noteFocusRequest, setNoteFocusRequest] = useState(0);
  const [resolution, setResolution] = useState<"save" | "discard">("save");
  const [retry, setRetry] = useState(0);
  const [notes] = useState(
    () =>
      new SheetNoteController(async (noteId, value) => {
        const user = useAuthStore.getState().user;
        if (!hasWorkspace(user, "teacher") || !can(user, "teaching.grading"))
          throw new Error("Note write is not permitted");
        const result = await setAttemptNote(noteId, value);
        client.setQueryData<AttemptReview>(reviewKey(noteId), (data) =>
          data?.attempt.id === noteId ? { ...data, teacherNote: value } : data,
        );
        return result;
      }),
  );
  useSyncExternalStore(notes.subscribe, notes.snapshot, notes.snapshot);
  const blocker = useBlocker(notes.unsettled);
  const blockedKey = blocker.state === "blocked" ? blocker.location.key : null;
  const departedKey = useRef<string | null>(null);
  const depart = useEffectEvent((key: string, saved: boolean) => {
    if (
      blocker.state === "blocked" &&
      blocker.location.key === key &&
      departedKey.current !== key
    ) {
      if (!saved) {
        setNoteFocusRequest((value) => value + 1);
        setRecovery(true);
        return;
      }
      departedKey.current = key;
      setRecovery(false);
      setResolution("save");
      blocker.proceed();
    }
  });
  useEffect(() => {
    let active = true;
    if (blockedKey === null) departedKey.current = null;
    if (blockedKey !== null)
      void (
        resolution === "discard"
          ? notes.discardAll().then(() => true)
          : notes.flushAll()
      ).then((saved) => {
        if (!active) return;
        depart(blockedKey, saved);
      });
    return () => {
      active = false;
    };
  }, [blockedKey, notes, resolution, retry]);
  useBeforeUnload(
    useCallback(
      (event) => {
        if (notes.unsettled) event.preventDefault();
      },
      [notes],
    ),
  );

  const recoveryPanel = recovery ? (
    <NoteRecovery
      onRetry={() => {
        setResolution("save");
        setRetry((value) => value + 1);
      }}
      onDiscard={() => {
        setResolution("discard");
        setRetry((value) => value + 1);
      }}
      onStay={() => {
        if (blocker.state === "blocked") {
          blocker.reset();
        }
        setRecovery(false);
        setResolution("save");
      }}
    />
  ) : null;
  return { notes, recoveryPanel, noteFocusRequest };
}

function StateDialogs({
  a,
  status,
  closing,
  reopening,
  pending,
  failed,
  rows,
  now,
  reopeningStudent,
  onReopeningStudent,
  onClosing,
  onReopening,
  onClose,
  onRefresh,
}: Readonly<{
  a: Assignment;
  status: ReturnType<typeof statusAt>;
  closing: boolean;
  reopening: ReopenChoice | null;
  pending: boolean;
  failed: boolean;
  rows: readonly MonitorRow[] | null;
  now: Date;
  reopeningStudent: boolean;
  onReopeningStudent: (value: boolean) => void;
  onClosing: (value: boolean) => void;
  onReopening: (value: ReopenChoice | null) => void;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}>) {
  const write = useCan("teaching.assignments.write");
  const intervene = useCan("teaching.attempts.intervene");
  return (
    <>
      {write && status === "open" && (
        <CloseEarlyDialog
          assignment={a}
          open={closing}
          pending={pending}
          failed={failed}
          keepTime={rows === null ? 0 : keepsTimeAfter(rows, now)}
          onOpenChange={onClosing}
          onConfirm={onClose}
        />
      )}
      {write && status === "closed" && reopening !== null && (
        <ReopenDialog
          assignment={a}
          choice={reopening}
          open
          onOpenChange={(open) => {
            if (!open) onReopening(null);
          }}
          onDone={onRefresh}
        />
      )}
      {intervene && status !== "draft" && rows !== null && (
        <ReopenStudentDialog
          assignment={a}
          rows={rows}
          open={reopeningStudent}
          onOpenChange={onReopeningStudent}
          onDone={onRefresh}
        />
      )}
    </>
  );
}

function NoteRecovery({
  onRetry,
  onDiscard,
  onStay,
}: Readonly<{ onRetry: () => void; onDiscard: () => void; onStay: () => void }>) {
  const { t } = useTranslation();
  return (
    <div role="alert" className="bg-danger-soft rounded-ctl space-y-2 p-3 text-sm">
      <p>{t("assignmentDetail.sheet.leaveFailed")}</p>
      <div className="flex flex-wrap gap-2">
        <Button size="sm" onClick={onRetry}>
          {t("common.retry")}
        </Button>
        <Button size="sm" variant="outline" onClick={onDiscard}>
          {t("assignmentDetail.sheet.discardLeave")}
        </Button>
        <Button size="sm" variant="ghost" onClick={onStay}>
          {t("assignmentDetail.sheet.stay")}
        </Button>
      </div>
    </div>
  );
}

function WriteActions({
  a,
  status,
  now,
  pending,
  onPublish,
  onClose,
  onReopen,
  onEditSettings,
  onReopenStudent,
}: Readonly<{
  a: Assignment;
  status: ReturnType<typeof statusAt>;
  now: Date;
  pending: boolean;
  onPublish: () => void;
  onClose: () => void;
  onReopen: (choice: ReopenChoice) => void;
  onEditSettings: () => void;
  onReopenStudent: () => void;
}>) {
  const { t } = useTranslation();
  const write = useCan("teaching.assignments.write");
  const intervene = useCan("teaching.attempts.intervene");
  const draft = status === "draft";
  if (!write && !(intervene && !draft)) return null;
  return (
    <>
      <RowMenu label={t("assignmentDetail.moreActions")}>
        {write && draft && (
          <DropdownMenuItem asChild>
            <Link to={`/teacher/assignments/${a.id}/edit`}>
              <Pencil aria-hidden="true" />
              {t("assignments.detail.edit")}
            </Link>
          </DropdownMenuItem>
        )}
        {write && !draft && (
          <DropdownMenuItem onSelect={onEditSettings}>
            <Pencil aria-hidden="true" />
            {t("assignmentDetail.editSettings")}
          </DropdownMenuItem>
        )}
        {intervene && !draft && (
          <DropdownMenuItem onSelect={onReopenStudent}>
            <RotateCcw aria-hidden="true" />
            {t("assignmentDetail.reopenStudent.title")}
          </DropdownMenuItem>
        )}
        {write && status === "open" && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              onSelect={() => {
                if (write && status === "open") onClose();
              }}
            >
              <CircleStop aria-hidden="true" />
              {t("assignments.detail.closeEarly")}
            </DropdownMenuItem>
          </>
        )}
      </RowMenu>
      {write && status === "closed" && (
        <ReopenMenu
          count={a.targetCount ?? 0}
          todayPossible={now.getHours() < 21}
          onChoose={(choice) => {
            if (write && status === "closed") onReopen(choice);
          }}
        />
      )}
      {write && draft && (
        <Button
          disabled={(a.targetCount ?? 0) === 0 || pending}
          onClick={() => {
            if (write && status === "draft") onPublish();
          }}
        >
          <Send aria-hidden="true" />
          {t("assignments.detail.publish")}
        </Button>
      )}
    </>
  );
}

function GradeAction({
  count,
  attemptId,
  onOpen,
}: Readonly<{
  count: number;
  attemptId: string | null | undefined;
  onOpen: (id: string) => void;
}>) {
  const { t } = useTranslation();
  const grade = useCan("teaching.grading");
  if (!grade || count === 0 || !attemptId) return null;
  return (
    <Button
      onClick={() => {
        if (grade && attemptId) onOpen(attemptId);
      }}
    >
      <SquarePen aria-hidden="true" />
      {t("assignmentDetail.gradeAnswers", { count })}
    </Button>
  );
}

function DetailStats({
  stats,
}: Readonly<{ stats: ReturnType<typeof assignmentStats> | null }>) {
  const { t } = useTranslation();
  return (
    <StatStrip
      items={[
        {
          label: t("assignmentDetail.submitted"),
          value: stats === null ? "—" : String(stats.submitted),
          suffix: stats === null ? undefined : ` / ${stats.total}`,
        },
        {
          label: t("monitor.cards.inProgress"),
          value: stats === null ? "—" : String(stats.inProgress),
        },
        {
          label: t("assignmentDetail.average"),
          value: stats?.average == null ? "—" : String(stats.average),
          suffix: stats?.average == null ? undefined : "%",
        },
        {
          label: t("assignments.detail.flagged"),
          value: stats === null ? "—" : String(stats.flagged),
          tone: (stats?.flagged ?? 0) > 0 ? FLAGGED.tone : "default",
        },
      ]}
    />
  );
}

function StudentsPanel({
  monitor,
  a,
  attemptId,
  onOpen,
  onRefresh,
}: Readonly<{
  monitor: UseQueryResult<Awaited<ReturnType<typeof getMonitor>>, Error>;
  a: Assignment;
  attemptId: string | null;
  onOpen: (id: string) => void;
  onRefresh: () => Promise<void>;
}>) {
  const { t } = useTranslation();
  if (monitor.isPending) return <ListSkeleton />;
  if (monitor.isError)
    return (
      <LoadError error={monitor.error} onRetry={() => void monitor.refetch()}>
        {t("monitor.loadFailed")}
      </LoadError>
    );
  return (
    <Monitor
      assignment={a}
      data={monitor.data}
      selectedAttempt={attemptId}
      onOpen={onOpen}
      onRefresh={onRefresh}
    />
  );
}

function QuestionsPanel({
  preview,
  a,
}: Readonly<{
  preview: UseQueryResult<Awaited<ReturnType<typeof previewTest>>, Error>;
  a: Assignment;
}>) {
  const { t } = useTranslation();
  if (preview.isPending) return <ListSkeleton />;
  if (preview.isError)
    return (
      <LoadError error={preview.error} onRetry={() => void preview.refetch()}>
        {t("assignmentDetail.questionsUnavailable")}
      </LoadError>
    );
  if (preview.data.version !== a.testVersion)
    return <EmptyState>{t("assignmentDetail.questionsUnavailable")}</EmptyState>;
  if (preview.data.questions.length === 0)
    return <EmptyState>{t("assignmentDetail.noQuestions")}</EmptyState>;
  return (
    <StudentPreviewPane
      questions={preview.data.questions}
      sections={preview.data.sections ?? []}
      groups={preview.data.groups ?? []}
      onRetryMedia={() => void preview.refetch()}
    />
  );
}

function TestCard({
  a,
  version,
  closed,
}: Readonly<{
  a: Assignment;
  version: TestVersion | undefined;
  closed: boolean;
}>) {
  const { t } = useTranslation();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("assignments.detail.test")}</CardTitle>
      </CardHeader>
      <CardContent className="pt-1">
        <div className="flex items-center gap-3 rounded-md border p-3">
          <FileText
            className="text-muted-foreground size-5 shrink-0"
            aria-hidden="true"
          />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{a.testTitle}</p>
            <p className="text-muted-foreground text-xs">
              {version === undefined
                ? t("assignments.detail.versionOnly", { version: a.testVersion })
                : t("assignments.detail.versionMeta", {
                    questions: version.questionCount,
                    points: version.totalPoints,
                    audio: version.audioCount,
                    manual: version.manualCount,
                    version: version.version,
                  })}
            </p>
          </div>
          <Button variant="ghost" size="sm" asChild>
            <Link to={`/teacher/tests/${a.testId}`}>
              <ExternalLink aria-hidden="true" />
              {t("assignments.detail.viewTest")}
            </Link>
          </Button>
        </div>
        <p className="text-muted-foreground mt-2 text-xs leading-relaxed">
          {t(closed ? "assignments.detail.pinnedClosed" : "assignments.detail.pinned", {
            version: a.testVersion,
          })}
        </p>
      </CardContent>
    </Card>
  );
}
