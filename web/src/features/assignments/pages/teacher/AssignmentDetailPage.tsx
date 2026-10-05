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
  Check,
  ExternalLink,
  FileText,
  GraduationCap,
  Lock,
  Pencil,
  Send,
  SquarePen,
  X,
} from "lucide-react";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { RowMenu } from "@/components/shared/RowMenu";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { StatStrip } from "@/components/shared/stats/StatStrip";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { can, hasWorkspace, useCan, useWorkspace } from "@/features/auth/permissions";
import {
  getMonitor,
  setAttemptNote,
  type AttemptReview,
} from "@/features/attempts/api";
import { AttemptSheet } from "@/features/attempts/components/AttemptSheet";
import { Monitor } from "@/features/attempts/components/Monitor";
import { SheetNoteController } from "@/features/attempts/components/sheetNotes";
import { POLL_MS, monitorKey, reviewKey } from "@/features/attempts/keys";
import { listVersions, previewTest, type TestVersion } from "@/features/tests/api";
import { StudentPreviewPane } from "@/features/tests/components/StudentPreviewPane";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { PageHead } from "@/layouts/shell/PageHead";
import { useCrumbs } from "@/layouts/shell/crumbs";
import { ApiError } from "@/lib/api/errors";
import { formatMoment } from "@/lib/i18n/datetime";
import { useAuthStore } from "@/stores/auth";
import { getAssignment, updateAssignment, type Assignment } from "../../api";
import { CloseEarlyDialog } from "../../components/CloseEarlyDialog";
import { ReopenDialog } from "../../components/ReopenDialog";
import { ReopenMenu, type ReopenChoice } from "../../components/ReopenMenu";
import { TargetsLine } from "../../components/TargetsLine";
import { toInput } from "../../input";
import { statusAt } from "../../status";
import { assignmentStats, detailTab, firstPendingPaper } from "./assignmentDetail";
import { assignmentDetailLocation } from "./assignmentDetailUrl";

/** AssignmentDetailPage owns the assignment monitor and preserves authorized actions and note drafts across its URL-controlled tabs and sheet. */
export default function AssignmentDetailPage() {
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
  const [failure, setFailure] = useState<string | null>(null);
  const { notes, recoveryPanel } = useSheetDrafts();

  const { assignment, a, now, status, monitor, version, preview } = useAssignmentReads(
    id,
    workspace,
    tab,
  );
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
  if (!workspace) return <EmptyState>{t("assignmentDetail.unavailable")}</EmptyState>;
  if (assignment.isPending) return <ListSkeleton rows={8} />;
  if (assignment.isError)
    return (
      <LoadError error={assignment.error} onRetry={() => void assignment.refetch()}>
        {t("assignments.detail.loadFailed")}
      </LoadError>
    );
  if (a === undefined || status === null)
    return (
      <EmptyState
        action={
          <Button variant="outline" asChild>
            <Link to="/teacher/assignments">{t("assignments.detail.backToList")}</Link>
          </Button>
        }
      >
        {t("assignments.detail.notFound")}
      </EmptyState>
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
          <TestCard a={a} version={version} closed={status === "closed"} />
          <QuestionsPanel preview={preview} a={a} />
        </TabsContent>
        <TabsContent value="settings" className="space-y-4 pt-4.5">
          <AssignmentSettings a={a} version={version} status={status} />
        </TabsContent>
      </Tabs>
      <StateDialogs
        a={a}
        status={status}
        closing={closing}
        reopening={reopening}
        pending={close.isPending}
        failed={close.isError}
        onClosing={setClosing}
        onReopening={setReopening}
        onClose={() => {
          if (write && status === "open") close.mutate(a);
        }}
        onRefresh={refresh}
      />
      {attemptId && (
        <AttemptSheet
          assignment={a}
          attemptId={attemptId}
          row={monitor.data?.rows.find((row) => row.attemptId === attemptId)}
          questionCount={monitor.data?.questionCount ?? 0}
          serverTime={
            monitor.data?.serverTime ?? new Date(monitor.dataUpdatedAt).toISOString()
          }
          receivedAt={monitor.dataUpdatedAt}
          notes={notes}
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

function AssignmentSettings({
  a,
  version,
  status,
}: Readonly<{
  a: Assignment;
  version: TestVersion | undefined;
  status: ReturnType<typeof statusAt>;
}>) {
  const { t } = useTranslation();
  const write = useCan("teaching.assignments.write");
  return (
    <>
      <TestCard a={a} version={version} closed={status === "closed"} />
      <TargetsLine assignment={a} />
      <TargetsCard a={a} />
      <TimeCard a={a} />
      <RulesCard a={a} />
      <ReviewCard a={a} />
      {write && status !== "closed" && (
        <Button variant="outline" asChild>
          <Link to={`/teacher/assignments/${a.id}/edit`}>
            <Pencil aria-hidden="true" />
            {t("assignments.detail.edit")}
          </Link>
        </Button>
      )}
    </>
  );
}

function useSheetDrafts() {
  const client = useQueryClient();
  const [recovery, setRecovery] = useState(false);
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
  return { notes, recoveryPanel };
}

function StateDialogs({
  a,
  status,
  closing,
  reopening,
  pending,
  failed,
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
  onClosing: (value: boolean) => void;
  onReopening: (value: ReopenChoice | null) => void;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}>) {
  const write = useCan("teaching.assignments.write");
  if (!write) return null;
  if (status === "open")
    return (
      <CloseEarlyDialog
        assignment={a}
        open={closing}
        pending={pending}
        failed={failed}
        onOpenChange={onClosing}
        onConfirm={onClose}
      />
    );
  if (status === "closed" && reopening !== null)
    return (
      <ReopenDialog
        assignment={a}
        choice={reopening}
        open
        onOpenChange={(open) => {
          if (!open) onReopening(null);
        }}
        onDone={onRefresh}
      />
    );
  return null;
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
}: Readonly<{
  a: Assignment;
  status: ReturnType<typeof statusAt>;
  now: Date;
  pending: boolean;
  onPublish: () => void;
  onClose: () => void;
  onReopen: (choice: ReopenChoice) => void;
}>) {
  const { t } = useTranslation();
  const write = useCan("teaching.assignments.write");
  if (!write) return null;
  if (status === "closed")
    return (
      <ReopenMenu
        count={a.targetCount ?? 0}
        todayPossible={now.getHours() < 21}
        onChoose={(choice) => {
          if (write && status === "closed") onReopen(choice);
        }}
      />
    );
  return (
    <>
      <RowMenu>
        <DropdownMenuItem asChild>
          <Link to={`/teacher/assignments/${a.id}/edit`}>
            <Pencil aria-hidden="true" />
            {t("assignments.detail.edit")}
          </Link>
        </DropdownMenuItem>
        {status === "open" && (
          <DropdownMenuItem
            variant="destructive"
            onSelect={() => {
              if (write && status === "open") onClose();
            }}
          >
            <Lock aria-hidden="true" />
            {t("assignments.detail.closeEarly")}
          </DropdownMenuItem>
        )}
      </RowMenu>
      {status === "draft" && (
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
          tone: (stats?.flagged ?? 0) > 0 ? "danger" : "default",
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

function TargetsCard({ a }: Readonly<{ a: Assignment }>) {
  const { t } = useTranslation();
  const { classes, students } = a.targets;
  const reached = classes.reduce((sum, c) => sum + c.studentCount, 0) + students.length;
  const overlap = Math.max(0, reached - (a.targetCount ?? reached));
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("assignments.detail.targets")}</CardTitle>
        <CardDescription>
          {t("assignments.detail.targetCount", { count: a.targetCount ?? 0 })}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 pt-1">
        {classes.length === 0 && students.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {t("assignments.detail.noTargets")}
          </p>
        ) : null}
        {classes.length > 0 && (
          <div>
            <p className="text-muted-foreground mb-1 text-xs">
              {t("assignments.detail.classes")}
            </p>
            <div className="flex flex-wrap items-center gap-1.5">
              {classes.map((c) => (
                <Badge key={c.id} variant="secondary">
                  <GraduationCap aria-hidden="true" />
                  {t("assignments.detail.classChip", { name: c.name })}
                </Badge>
              ))}
            </div>
          </div>
        )}
        {students.length > 0 && (
          <div>
            <p className="text-muted-foreground mb-1 text-xs">
              {t("assignments.detail.students")}
            </p>
            <div className="flex flex-wrap items-center gap-1.5">
              {students.map((s) => (
                <Badge key={s.id} variant="secondary">
                  {s.name}
                </Badge>
              ))}
            </div>
            {overlap > 0 && (
              <p className="text-muted-foreground mt-1.5 text-xs">
                {t("assignments.detail.overlap", { count: overlap })}
              </p>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function TimeCard({ a }: Readonly<{ a: Assignment }>) {
  const { t } = useTranslation();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("assignments.detail.time")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 pt-1 text-sm">
        <Line
          label={t("assignments.detail.opens")}
          value={formatMoment(a.window.opensAt)}
        />
        <Line
          label={t("assignments.detail.closes")}
          value={formatMoment(a.window.closesAt)}
        />
        <Line
          label={t("assignments.detail.duration")}
          value={t("assignments.minutes", { count: a.durationMinutes })}
        />
        <Line
          label={t("assignments.detail.attempts")}
          value={
            a.maxAttempts === 1
              ? t("assignments.detail.attemptsOne")
              : t("assignments.detail.attemptsMany", { count: a.maxAttempts })
          }
        />
      </CardContent>
    </Card>
  );
}

function RulesCard({ a }: Readonly<{ a: Assignment }>) {
  const { t } = useTranslation();
  const { integrity } = a;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("assignments.detail.rules")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 pt-1 text-sm">
        <Line
          label={t("assignments.detail.fullscreen")}
          value={t(
            integrity.requireFullscreen
              ? "assignments.detail.required"
              : "assignments.detail.notRequired",
          )}
        />
        <Line
          label={t("assignments.detail.copyPaste")}
          value={t(
            integrity.blockCopyPaste
              ? "assignments.detail.blocked"
              : "assignments.detail.allowed",
          )}
        />
        <Line
          label={t("assignments.detail.focusLoss")}
          value={
            integrity.maxFocusLoss === 0
              ? t("assignments.detail.focusUnlimited")
              : t(`assignments.detail.focusLimit.${integrity.onLimitExceeded}`, {
                  count: Math.max(0, integrity.maxFocusLoss),
                })
          }
        />
        <Line
          label={t("assignments.detail.shuffleQuestions")}
          value={t(
            a.shuffleQuestions
              ? "assignments.detail.yesWithinSections"
              : "assignments.detail.no",
          )}
        />
        <Line
          label={t("assignments.detail.shuffleOptions")}
          value={t(
            a.shuffleOptions ? "assignments.detail.yes" : "assignments.detail.no",
          )}
        />
      </CardContent>
    </Card>
  );
}

function ReviewCard({ a }: Readonly<{ a: Assignment }>) {
  const { t } = useTranslation();
  const { review } = a;
  let hint = "assignments.detail.reuseHint";
  if (review.showCorrectAnswers) hint = "assignments.detail.reviewWhileOpen";
  if (statusAt(a, new Date()) === "closed")
    hint = "assignments.detail.reviewAfterClose";
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("assignments.detail.review")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 pt-1 text-sm">
        <Flag on={review.showScore}>{t("assignments.showScore")}</Flag>
        <Flag on={review.showCorrectAnswers}>
          {t("assignments.showCorrectAnswers")}
        </Flag>
        <Flag on={review.showExplanations}>{t("assignments.showExplanations")}</Flag>
        <p className="text-muted-foreground text-xs leading-relaxed">{t(hint)}</p>
      </CardContent>
    </Card>
  );
}

function Flag({ on, children }: Readonly<{ on: boolean; children: string }>) {
  return (
    <div
      className={
        on ? "flex items-center gap-2" : "text-muted-foreground flex items-center gap-2"
      }
    >
      {on ? (
        <Check className="text-muted-foreground size-4" aria-hidden="true" />
      ) : (
        <X className="size-4" aria-hidden="true" />
      )}
      {children}
    </div>
  );
}

function Line({ label, value }: Readonly<{ label: string; value: string }>) {
  return (
    <div className="flex justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium tabular-nums">{value}</span>
    </div>
  );
}
