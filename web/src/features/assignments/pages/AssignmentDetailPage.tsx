import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  getAssignment,
  updateAssignment,
  type Assignment,
  type AssignmentStatus,
} from "@/features/assignments/api";
import { CloseEarlyDialog } from "@/features/assignments/components/CloseEarlyDialog";
import { ReopenDialog } from "@/features/assignments/components/ReopenDialog";
import {
  ReopenMenu,
  type ReopenChoice,
} from "@/features/assignments/components/ReopenMenu";
import { StudentPanel } from "@/features/assignments/components/StudentPanel";
import { TargetsLine } from "@/features/assignments/components/TargetsLine";
import { getMonitor } from "@/features/attempts/api";
import { Monitor } from "@/features/attempts/components/Monitor";
import { monitorKey } from "@/features/attempts/keys";
import { toInput } from "@/features/assignments/input";
import { statusAt } from "@/features/assignments/status";
import { listVersions, type TestVersion } from "@/features/tests/api";
import { ApiError } from "@/lib/api/errors";
import { formatDate, formatMoment, formatTime } from "@/lib/i18n/datetime";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import {
  ArrowRight,
  CalendarClock,
  Check,
  ExternalLink,
  Eye,
  EyeOff,
  FileText,
  GraduationCap,
  Lock,
  Pencil,
  RefreshCw,
  Send,
  X,
} from "lucide-react";
import { useState, type ReactNode } from "react";
import { Trans, useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";

/** G-09: one route, four states. The bar is the state machine; the summary is what G-01 saved. */
export default function AssignmentDetailPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const [closing, setClosing] = useState(false);
  const [reopening, setReopening] = useState<ReopenChoice | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  const assignment = useQuery({
    queryKey: ["admin-assignment", id],
    queryFn: ({ signal }) => getAssignment(id, signal),
  });
  const testId = assignment.data?.testId;
  const versions = useQuery({
    queryKey: ["admin-test-versions", testId],
    queryFn: ({ signal }) => listVersions(testId ?? "", signal),
    enabled: testId !== undefined,
  });

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: ["admin-assignment", id] });
    await queryClient.invalidateQueries({ queryKey: ["admin-assignments"] });
    await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
    await queryClient.invalidateQueries({ queryKey: monitorKey(id) });
  };
  const publish = useMutation({
    mutationFn: (a: Assignment) =>
      updateAssignment(a.id, { ...toInput(a), draft: false }),
    onSuccess: refresh,
    onError: (cause) =>
      setFailure(
        cause instanceof ApiError
          ? cause.message
          : t("assignments.detail.publishFailed"),
      ),
  });
  const close = useMutation({
    mutationFn: (a: Assignment) =>
      updateAssignment(a.id, { ...toInput(a), draft: false, closeNow: true }),
    onSuccess: async () => {
      setClosing(false);
      await refresh();
    },
  });

  if (assignment.isPending) {
    return <ListSkeleton rows={8} />;
  }
  if (assignment.isError) {
    const missing =
      assignment.error instanceof ApiError && assignment.error.status === 404;
    return missing ? (
      <EmptyState
        action={
          <Button variant="outline" size="sm" asChild>
            <Link to="/admin/assignments">{t("assignments.detail.backToList")}</Link>
          </Button>
        }
      >
        {t("assignments.detail.notFound")}
      </EmptyState>
    ) : (
      <LoadError error={assignment.error} onRetry={() => void assignment.refetch()}>
        {t("assignments.detail.loadFailed")}
      </LoadError>
    );
  }

  const a = assignment.data;
  const now = new Date();
  const status = statusAt(a, now);
  const version = versions.data?.items.find((v) => v.id === a.testVersionId);
  const hasTargets = a.targets.classes.length > 0 || a.targets.students.length > 0;
  const targetCount = a.targetCount ?? 0;

  return (
    <>
      <PageHeader
        title={a.testTitle}
        backTo="/admin/assignments"
        meta={
          <>
            <StatusBadge kind="assignment" status={status} />
            <span className="text-muted-foreground text-xs">
              {barMeta(a, status, now, t)}
            </span>
          </>
        }
        actions={
          <Actions
            status={status}
            hasTargets={hasTargets}
            targetCount={targetCount}
            editHref={`/admin/assignments/${a.id}/edit`}
            attemptsHref={`/admin/assignments/${a.id}/attempts`}
            publishing={publish.isPending}
            now={now}
            onPublish={() => {
              setFailure(null);
              publish.mutate(a);
            }}
            onClose={() => setClosing(true)}
            onReopen={setReopening}
            onRefresh={() => void refresh()}
          />
        }
      />

      <div className="space-y-4">
        {failure === null ? null : (
          <p role="alert" className="text-destructive text-sm">
            {failure}
          </p>
        )}
        {status === "draft" && (
          <Note
            icon={
              <EyeOff
                className="text-muted-foreground mt-0.5 size-4 shrink-0"
                aria-hidden="true"
              />
            }
          >
            <Trans
              i18nKey="assignments.detail.draftNote"
              values={{ count: targetCount }}
              components={{ strong: <strong /> }}
            />
          </Note>
        )}
        {status === "scheduled" && (
          <Note
            icon={
              <CalendarClock
                className="text-muted-foreground mt-0.5 size-4 shrink-0"
                aria-hidden="true"
              />
            }
          >
            <Trans
              i18nKey="assignments.detail.scheduledNote"
              values={{
                when: formatMoment(a.window.opensAt),
                left: timeLeft(new Date(a.window.opensAt).getTime() - now.getTime(), t),
                count: targetCount,
              }}
              components={{ strong: <strong /> }}
            />
          </Note>
        )}
        {status === "closed" && (
          <ResultsStrip
            a={a}
            version={version}
            panelOpen={panelOpen}
            onTogglePanel={() => setPanelOpen((open) => !open)}
          />
        )}
        {/* G-09: open shows the live table (G-02); closed shows the numbers, and the papers live on G-11. */}
        {status === "open" && (
          <>
            <TargetsLine assignment={a} />
            <Monitor assignment={a} live />
          </>
        )}

        {status !== "open" && (
          <div className="grid gap-4 md:grid-cols-2">
            <TestCard a={a} version={version} closed={status === "closed"} />
            <TargetsCard a={a} />
            <TimeCard a={a} />
            <RulesCard a={a} />
            <ReviewCard a={a} />
          </div>
        )}
      </div>

      <CloseEarlyDialog
        assignment={a}
        open={closing}
        pending={close.isPending}
        failed={close.isError}
        onOpenChange={setClosing}
        onConfirm={() => close.mutate(a)}
      />
      {reopening !== null && (
        <ReopenDialog
          assignment={a}
          choice={reopening}
          open
          onOpenChange={(open) => {
            if (!open) setReopening(null);
          }}
          onDone={refresh}
        />
      )}
      {status === "closed" && panelOpen && (
        <StudentPanel assignment={a} open onOpenChange={setPanelOpen} />
      )}
    </>
  );
}

function barMeta(
  a: Assignment,
  status: AssignmentStatus,
  now: Date,
  t: TFunction,
): string {
  switch (status) {
    case "draft":
      return formatDate(a.updatedAt) === formatDate(now)
        ? t("assignments.detail.savedToday", { time: formatTime(a.updatedAt) })
        : t("assignments.detail.savedOn", {
            time: formatTime(a.updatedAt),
            date: formatDate(a.updatedAt),
          });
    case "scheduled":
      return t("assignments.detail.opensAt", { when: formatMoment(a.window.opensAt) });
    case "open":
      return t("assignments.detail.closesAt", {
        when: formatMoment(a.window.closesAt),
      });
    case "closed":
      return t("assignments.detail.closedAt", {
        when: formatMoment(a.window.closedAt ?? a.window.closesAt),
      });
  }
}

function timeLeft(ms: number, t: TFunction): string {
  const hours = Math.floor(ms / 3_600_000);
  const days = Math.floor(hours / 24);
  if (days > 0)
    return t("assignments.detail.left.daysHours", { days, hours: hours - days * 24 });
  if (hours > 0) return t("assignments.detail.left.hours", { hours });
  return t("assignments.detail.left.minutes", {
    minutes: Math.max(1, Math.floor(ms / 60_000)),
  });
}

function Actions({
  status,
  hasTargets,
  targetCount,
  editHref,
  attemptsHref,
  publishing,
  now,
  onPublish,
  onClose,
  onReopen,
  onRefresh,
}: {
  status: AssignmentStatus;
  hasTargets: boolean;
  targetCount: number;
  editHref: string;
  attemptsHref: string;
  publishing: boolean;
  now: Date;
  onPublish: () => void;
  onClose: () => void;
  onReopen: (choice: ReopenChoice) => void;
  onRefresh: () => void;
}) {
  const { t } = useTranslation();
  const edit = (
    <Button variant="outline" size="sm" asChild>
      <Link to={editHref}>
        <Pencil aria-hidden="true" />
        {t("assignments.detail.edit")}
      </Link>
    </Button>
  );
  switch (status) {
    case "draft":
      return (
        <>
          {hasTargets ? null : (
            <span className="text-muted-foreground text-xs">
              {t("assignments.needTargets")}
            </span>
          )}
          {edit}
          <Button size="sm" disabled={!hasTargets || publishing} onClick={onPublish}>
            <Send aria-hidden="true" />
            {t("assignments.detail.publish")}
          </Button>
        </>
      );
    case "scheduled":
      return (
        <>
          <span className="text-muted-foreground text-xs">
            {t("assignments.detail.closeEarlyWhenOpen")}
          </span>
          {edit}
          <Button variant="outline" size="sm" disabled>
            <Lock aria-hidden="true" />
            {t("assignments.detail.closeEarly")}
          </Button>
        </>
      );
    case "open":
      return (
        <>
          <span className="text-muted-foreground text-xs">{t("monitor.polling")}</span>
          <Button variant="outline" size="sm" onClick={onRefresh}>
            <RefreshCw aria-hidden="true" />
            {t("monitor.refresh")}
          </Button>
          <Button variant="outline" size="sm" onClick={onClose}>
            <Lock aria-hidden="true" />
            {t("assignments.detail.closeEarly")}
          </Button>
        </>
      );
    case "closed":
      // G-09: closing is reversible, and the papers are one click away (G-11).
      return (
        <>
          <ReopenMenu
            count={targetCount}
            todayPossible={now.getHours() < 21}
            onChoose={onReopen}
          />
          <Button size="sm" asChild>
            <Link to={attemptsHref}>
              <Eye aria-hidden="true" />
              {t("assignments.detail.viewAttempts")}
            </Link>
          </Button>
        </>
      );
  }
}

function Note({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <div className="bg-muted/40 flex items-start gap-2 rounded-md px-3 py-2.5">
      {icon}
      <p className="text-xs leading-relaxed">{children}</p>
    </div>
  );
}

function ResultsStrip({
  a,
  version,
  panelOpen,
  onTogglePanel,
}: {
  a: Assignment;
  version: TestVersion | undefined;
  panelOpen: boolean;
  onTogglePanel: () => void;
}) {
  const { t } = useTranslation();
  const submitted = a.submittedCount ?? 0;
  const total = a.targetCount ?? 0;
  // The same read the table below makes, so the names cost nothing extra.
  const monitor = useQuery({
    queryKey: monitorKey(a.id),
    queryFn: ({ signal }) => getMonitor(a.id, signal),
  });
  const missing = (monitor.data?.rows ?? [])
    .filter((row) => row.state === "not_started")
    .map((row) => row.fullName);
  return (
    <Card>
      <CardContent className="flex items-center gap-6">
        <Stat
          label={t("assignments.detail.submitted")}
          value={
            <>
              {submitted}
              <span className="text-muted-foreground text-base">/{total}</span>
            </>
          }
          hint={
            missing.length > 0
              ? t("assignments.detail.notSubmittedNames", {
                  names: someNames(missing, t),
                })
              : t("assignments.detail.notSubmitted", {
                  count: Math.max(0, total - submitted),
                })
          }
        />
        <div className="bg-border h-10 w-px" />
        <Stat
          label={t("assignments.detail.pending")}
          value={a.pendingGradingCount ?? 0}
          hint={
            version === undefined
              ? null
              : t("assignments.detail.pendingHint", { count: version.manualCount })
          }
        />
        <div className="bg-border h-10 w-px" />
        <Stat
          label={t("assignments.detail.flagged")}
          value={a.flaggedCount ?? 0}
          hint={
            a.integrity.maxFocusLoss > 0
              ? t("assignments.detail.flaggedHint", { count: a.integrity.maxFocusLoss })
              : t("assignments.detail.flaggedHintNone")
          }
        />
        <Button variant="link" size="sm" className="ml-auto" onClick={onTogglePanel}>
          {t(
            panelOpen
              ? "assignments.detail.closeTable"
              : "assignments.detail.openTable",
          )}
          <ArrowRight aria-hidden="true" />
        </Button>
      </CardContent>
    </Card>
  );
}

/** Three names in full; past that, the count, so the strip stays one line. */
function someNames(names: string[], t: TFunction): string {
  if (names.length <= 3) return names.join(", ");
  return `${names.slice(0, 3).join(", ")} ${t("assignments.detail.andMore", {
    count: names.length - 3,
  })}`;
}

function Stat({
  label,
  value,
  hint,
}: {
  label: string;
  value: ReactNode;
  hint: string | null;
}) {
  return (
    <div>
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
      {hint === null ? null : (
        <p className="text-muted-foreground mt-1 text-xs">{hint}</p>
      )}
    </div>
  );
}

function TestCard({
  a,
  version,
  closed,
}: {
  a: Assignment;
  version: TestVersion | undefined;
  closed: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Card className="md:col-span-2">
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
            <Link to={`/admin/tests/${a.testId}`}>
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

function TargetsCard({ a }: { a: Assignment }) {
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
                  {t("assignments.detail.classChip", {
                    name: c.name,
                    count: c.studentCount,
                  })}
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

function TimeCard({ a }: { a: Assignment }) {
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

function RulesCard({ a }: { a: Assignment }) {
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
                  count: integrity.maxFocusLoss,
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

function ReviewCard({ a }: { a: Assignment }) {
  const { t } = useTranslation();
  const { review } = a;
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
        {review.showCorrectAnswers ? null : (
          <p className="text-muted-foreground text-xs leading-relaxed">
            {t("assignments.detail.reuseHint")}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

function Flag({ on, children }: { on: boolean; children: string }) {
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

function Line({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium tabular-nums">{value}</span>
    </div>
  );
}
