import { useEffect, useReducer, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { Clock, History, List, Repeat, Timer } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { fetchMyClasses } from "@/features/classes/api";
import { startOrResumeAttempt } from "@/features/take-test/api";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";
import {
  countdown,
  formatTime,
  sameAppDay,
  shortDate,
  weekdayDate,
} from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import { listMyAssignments, type StudentAssignmentCard } from "../api";
import { closesLine, givenName, scoreText, timeLeft } from "../studentTime";

/** StudentHomePage groups every assignment by its next action. */
export default function StudentHomePage() {
  const { t, i18n } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const locale = i18n.language as Locale;
  const assignments = useQuery({
    queryKey: ["my-assignments"],
    queryFn: ({ signal }) => listMyAssignments(signal),
  });
  const classes = useQuery({
    queryKey: ["my-classes"],
    queryFn: ({ signal }) => fetchMyClasses(signal),
  });
  const name = givenName(user?.fullName ?? "");
  const heading = (
    <h1 className="text-2xl font-semibold tracking-tight">
      {t("student.greetingPlain", { name })}
    </h1>
  );
  if (assignments.data === undefined)
    return (
      <div className="space-y-5">
        {heading}
        {assignments.isPending ? (
          <ListSkeleton rows={3} />
        ) : (
          <LoadError
            error={assignments.error}
            onRetry={() => void assignments.refetch()}
          >
            {t("student.loadFailed")}
          </LoadError>
        )}
      </div>
    );

  const data = assignments.data;
  const all = [...data.dueNow, ...data.upcoming, ...data.completed];
  const dueNow = data.dueNow;
  const live = dueNow
    .filter((c) => c.hasLiveAttempt)
    .sort(
      (a, b) =>
        (a.liveDeadlineAt ?? a.closesAt).localeCompare(
          b.liveDeadlineAt ?? b.closesAt,
        ) || a.id.localeCompare(b.id),
    );
  const due = dueNow
    .filter((c) => !c.hasLiveAttempt)
    .sort((a, b) => a.closesAt.localeCompare(b.closesAt) || a.id.localeCompare(b.id));
  const upcoming = data.upcoming;
  const completed = data.completed;
  const now = new Date();
  const dueToday = dueNow.filter((c) => sameAppDay(c.closesAt, now)).length;

  return (
    <div className="space-y-6">
      <div>
        {heading}
        {dueNow.length > 0 && (
          <p className="text-muted-foreground mt-1 text-sm">
            {dueToday > 0
              ? t("student.dueToday", { count: dueToday })
              : t("student.dueOpen", { count: dueNow.length })}
          </p>
        )}
      </div>
      {all.length === 0 && (
        <EmptyState hint={t("student.noAssignmentsHint")}>
          {t("student.noAssignments")}
        </EmptyState>
      )}
      {all.length === 0 && classes.data?.items.length === 0 && (
        <EmptyState
          action={
            <Button asChild>
              <Link to="/join">{t("student.joinClass")}</Link>
            </Button>
          }
        >
          {t("student.noClasses")}
        </EmptyState>
      )}
      {live.length > 0 && (
        <Section title={t("student.inProgress", { count: live.length })}>
          {live.map((card) => (
            <ResumeCard key={card.id} card={card} />
          ))}
        </Section>
      )}
      {due.length > 0 && (
        <Section title={t("student.readyToStart", { count: due.length })}>
          {due.map((card) => (
            <DueCard key={card.id} card={card} now={now} />
          ))}
        </Section>
      )}
      {upcoming.length > 0 && (
        <Section title={t("student.upcoming", { count: upcoming.length })}>
          {upcoming.map((card) => (
            <Card key={card.id} className="surface-lift min-w-0 gap-2 p-5 shadow-sm">
              <p className="text-base font-semibold break-words">{card.testTitle}</p>
              {card.className && (
                <p className="text-muted-foreground text-xs">{card.className}</p>
              )}
              <p className="text-muted-foreground text-sm">
                {t("student.opensAt", {
                  time: formatTime(card.opensAt),
                  date: weekdayDate(card.opensAt, locale),
                })}
              </p>
              <div>
                <StatusBadge kind="assignment" status="scheduled" />
              </div>
              <Button asChild variant="outline" className="mt-2">
                <Link to={`/app/assignments/${card.id}`}>
                  {t("student.viewAssignment")}
                </Link>
              </Button>
            </Card>
          ))}
        </Section>
      )}
      {completed.length > 0 && (
        <Section title={t("student.completed", { count: completed.length })}>
          {completed.map((card) => (
            <Card
              key={card.id}
              className="surface-lift min-w-0 flex-row items-center gap-3 p-4 shadow-xs"
            >
              <div className="min-w-0 flex-1">
                {card.lastAttemptId ? (
                  <Link
                    to={`/app/attempts/${card.lastAttemptId}/result`}
                    className="flex min-h-11 items-center rounded-sm text-sm font-medium hover:underline"
                  >
                    {card.testTitle}
                  </Link>
                ) : (
                  <p className="text-sm font-medium">{card.testTitle}</p>
                )}
                <p className="text-muted-foreground text-xs">
                  {card.className != null && `${card.className} · `}
                  {card.lastSubmittedAt == null
                    ? t("student.attempt", {
                        n: card.attemptsUsed,
                        total: card.maxAttempts,
                      })
                    : t("student.submittedOn", {
                        date: shortDate(card.lastSubmittedAt),
                      })}
                </p>
              </div>
              <Outcome card={card} locale={locale} />
            </Card>
          ))}
        </Section>
      )}
    </div>
  );
}

function Section({
  title,
  children,
}: Readonly<{ title: string; children: ReactNode }>) {
  return (
    <section>
      <h2 className="mb-3 text-base font-semibold">{title}</h2>
      <div className="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">{children}</div>
    </section>
  );
}

function DueCard({ card, now }: Readonly<{ card: StudentAssignmentCard; now: Date }>) {
  const { t } = useTranslation();
  return (
    <Card className="surface-lift min-w-0 gap-0 p-5 shadow-sm">
      <div className="min-w-0 flex-1">
        <div className="flex flex-col items-start gap-2 lg:flex-row lg:items-center">
          <Badge
            variant={
              new Date(card.closesAt).getTime() - now.getTime() <= 86_400_000
                ? "warning"
                : "outline"
            }
          >
            <Clock aria-hidden="true" />
            {timeLeft(card.closesAt, now, t)}
          </Badge>
          {card.className != null && (
            <span className="text-muted-foreground text-xs">{card.className}</span>
          )}
        </div>
        <p className="mt-2.5 text-base leading-snug font-semibold lg:text-lg">
          {card.testTitle}
        </p>
        <div className="text-muted-foreground mt-2 flex flex-wrap items-center gap-4 text-xs">
          <span className="flex items-center gap-1.5">
            <Timer className="size-3.5" aria-hidden="true" />
            {t("student.minutes", { count: card.durationMinutes })}
          </span>
          <span className="flex items-center gap-1.5">
            <List className="size-3.5" aria-hidden="true" />
            {t("student.questions", { count: card.questionCount })}
          </span>
          <span className="flex items-center gap-1.5">
            <Repeat className="size-3.5" aria-hidden="true" />
            {t("student.attempt", {
              n: Math.min(card.attemptsUsed + 1, card.maxAttempts),
              total: card.maxAttempts,
            })}
          </span>
        </div>
        <p className="text-muted-foreground mt-2 text-xs">
          {closesLine(card.closesAt, now, t)}
        </p>
      </div>
      {/* To the intro, not the paper: the rules are read before the clock starts (S-04). */}
      <Button asChild variant="outline" className="mt-5 w-full">
        <Link to={`/app/assignments/${card.id}`}>{t("student.viewAssignment")}</Link>
      </Button>
    </Card>
  );
}

/**
 * Resuming skips the intro: the rules were read when the attempt began, and
 * the only thing this student needs is the way back in.
 */
function ResumeCard({ card }: Readonly<{ card: StudentAssignmentCard }>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const resume = async () => {
    setBusy(true);
    setError(null);
    try {
      const session = await startOrResumeAttempt(card.id);
      await navigate(`/app/attempts/${session.attempt.id}`);
    } catch (cause) {
      setError(
        cause instanceof ApiError ? cause.message : t("student.intro.startFailed"),
      );
      setBusy(false);
    }
  };

  return (
    <Card className="surface-lift border-foreground/20 bg-muted/30 min-w-0 gap-0 p-5 shadow-sm">
      <div className="min-w-0 flex-1">
        <div className="flex items-start gap-3">
          <History
            className="text-warning-ink mt-0.5 size-5 shrink-0"
            aria-hidden="true"
          />
          <div className="min-w-0">
            <p className="text-base font-semibold">{t("student.resumeTitle")}</p>
            {card.className && (
              <p className="text-muted-foreground mt-1 text-xs">{card.className}</p>
            )}
            <ResumeBody card={card} />
          </div>
        </div>
        {error !== null && (
          <p role="alert" className="mt-3 text-xs">
            {error}
          </p>
        )}
      </div>
      <Button
        size="lg"
        className="mt-4 w-full"
        disabled={busy}
        onClick={() => void resume()}
      >
        {t("student.resume")}
      </Button>
    </Card>
  );
}

/** Derived during render; the interval only asks for a repaint, as Clock does. */
function ResumeBody({ card }: Readonly<{ card: StudentAssignmentCard }>) {
  const { t } = useTranslation();
  const deadlineAt = card.liveDeadlineAt;
  const [, repaint] = useReducer((n: number) => n + 1, 0);
  useEffect(() => {
    if (deadlineAt == null) return;
    const tick = setInterval(repaint, 1000);
    return () => clearInterval(tick);
  }, [deadlineAt]);

  return (
    <p className="text-muted-foreground mt-1 text-sm leading-relaxed">
      {deadlineAt == null
        ? t("student.resumeBody", { title: card.testTitle })
        : t("student.resumeBodyLeft", {
            title: card.testTitle,
            left: countdown(new Date(deadlineAt).getTime() - new Date().getTime()),
          })}
    </p>
  );
}

/** A number, or "Chờ chấm", or nothing -- score is absent when the policy hides it. */
function Outcome({
  card,
  locale,
}: Readonly<{ card: StudentAssignmentCard; locale: Locale }>) {
  const { t } = useTranslation();
  const score = card.score;
  if (!score) return null;
  if (score.pendingManual > 0) {
    return <Badge variant="outline">{t("student.pendingGrading")}</Badge>;
  }
  return (
    <span className="text-sm font-semibold tabular-nums">
      {scoreText(score.earned, score.total, locale, t)}
    </span>
  );
}
