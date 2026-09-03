import { useEffect, useReducer, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { Clock, History, List, Repeat, Timer } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { fetchMyClasses } from "@/features/classes/api";
import { startOrResumeAttempt } from "@/features/take-test/api";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";
import { formatTime } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import { listMyAssignments, type StudentAssignmentCard } from "../api";
import {
  closesLine,
  countdown,
  givenName,
  sameAppDay,
  scoreText,
  shortDate,
  timeLeft,
  weekdayDate,
} from "../studentTime";

/**
 * S-03: what to do next, in the order it matters.
 *
 * A live attempt outranks everything, including a nearer deadline -- it is
 * burning a server-side clock the student cannot see from anywhere else. Then
 * what is due, what is coming, what is done. Sections with nothing in them
 * are not drawn; the whole-page empties are two different truths (no work
 * yet, or no class yet), and a student who has a class is never offered a
 * join code as if something had gone wrong.
 */
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

  // The greeting is the student's, not the data's: it stays put while the
  // list loads, and when it fails.
  if (!assignments.isSuccess) {
    return (
      <div className="space-y-5">
        <h1 className="text-lg font-semibold tracking-tight">
          {t("student.greetingPlain", { name })}
        </h1>
        {assignments.isPending ? (
          <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
            {t("common.loading")}
          </p>
        ) : (
          <div className="space-y-3">
            <p role="alert" className="text-sm">
              {t("student.loadFailed")}
            </p>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void assignments.refetch()}
            >
              {t("common.retry")}
            </Button>
          </div>
        )}
      </div>
    );
  }

  const { dueNow, upcoming, completed } = assignments.data;
  const live = dueNow.find((c) => c.hasLiveAttempt === true);
  const due = dueNow.filter((c) => c.hasLiveAttempt !== true);
  const nothing = dueNow.length + upcoming.length + completed.length === 0;
  const now = new Date();
  const dueToday = due.filter((c) => sameAppDay(c.closesAt, now)).length;

  if (nothing) {
    return (
      <div>
        <h1 className="text-lg font-semibold tracking-tight">
          {t("student.greetingPlain", { name })}
        </h1>
        <div className="mt-6 rounded-lg border border-dashed p-8 text-center">
          <p className="text-sm">{t("student.noAssignments")}</p>
          <p className="text-muted-foreground mt-1.5 text-xs leading-relaxed">
            {t("student.noAssignmentsHint")}
          </p>
        </div>
        {classes.data !== undefined && classes.data.items.length === 0 && (
          <div className="mt-3 rounded-lg border border-dashed p-8 text-center">
            <p className="text-sm">{t("student.noClasses")}</p>
            <Button asChild size="sm" className="mt-3">
              <Link to="/join">{t("student.joinClass")}</Link>
            </Button>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-lg font-semibold tracking-tight">
          {t(live ? "student.greetingPlain" : "student.greeting", { name })}
        </h1>
        {due.length > 0 && (
          <p className="text-muted-foreground text-sm">
            {dueToday > 0
              ? t("student.dueToday", { count: dueToday })
              : t("student.dueOpen", { count: due.length })}
          </p>
        )}
      </div>

      {live && <ResumeCard card={live} />}

      {due.map((card) => (
        <DueCard key={card.id} card={card} now={now} />
      ))}

      {upcoming.length > 0 && (
        <Section title={t("student.upcoming", { count: upcoming.length })}>
          {upcoming.map((card) => (
            <Card key={card.id} className="flex-row items-center gap-3 p-3.5">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{card.testTitle}</p>
                <p className="text-muted-foreground text-xs">
                  {t("student.opensAt", {
                    time: formatTime(card.opensAt),
                    date: weekdayDate(card.opensAt, locale),
                  })}
                </p>
              </div>
              <Badge variant="outline">{t("student.scheduled")}</Badge>
            </Card>
          ))}
        </Section>
      )}

      {completed.length > 0 && (
        <Section title={t("student.completed", { count: completed.length })}>
          {completed.map((card) => (
            <Card key={card.id} className="flex-row items-center gap-3 p-3.5">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{card.testTitle}</p>
                <p className="text-muted-foreground text-xs">
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

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h2 className="text-muted-foreground mb-2 text-xs font-medium tracking-wide uppercase">
        {title}
      </h2>
      <div className="space-y-2">{children}</div>
    </section>
  );
}

/** The deck's due card: the one with a button, because it is the one to act on. */
function DueCard({ card, now }: { card: StudentAssignmentCard; now: Date }) {
  const { t } = useTranslation();
  return (
    <Card className="border-foreground/20 gap-0 p-5 shadow-sm">
      <div className="flex items-center gap-2">
        <Badge variant="warning">
          <Clock aria-hidden="true" />
          {timeLeft(card.closesAt, now, t)}
        </Badge>
        {card.className != null && (
          <span className="text-muted-foreground text-xs">{card.className}</span>
        )}
      </div>
      <p className="mt-2.5 text-base leading-snug font-semibold">{card.testTitle}</p>
      <div className="text-muted-foreground mt-2 flex items-center gap-4 text-xs">
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
      {/* To the intro, not the paper: the rules are read before the clock starts (S-04). */}
      <Button asChild size="lg" className="mt-4 w-full">
        <Link to={`/app/assignments/${card.id}`}>{t("student.start")}</Link>
      </Button>
    </Card>
  );
}

/**
 * Resuming skips the intro: the rules were read when the attempt began, and
 * the only thing this student needs is the way back in.
 */
function ResumeCard({ card }: { card: StudentAssignmentCard }) {
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
      // The server's own sentence when it has one -- "hết lượt" is an answer,
      // "thử lại" is not.
      setError(
        cause instanceof ApiError ? cause.message : t("student.intro.startFailed"),
      );
      setBusy(false);
    }
  };

  return (
    <Card className="border-warning/30 bg-warning/8 gap-0 p-5">
      <div className="flex items-start gap-3">
        <History
          className="text-warning-ink mt-0.5 size-5 shrink-0"
          aria-hidden="true"
        />
        <div className="min-w-0">
          <p className="text-sm font-semibold">{t("student.resumeTitle")}</p>
          <ResumeBody card={card} />
        </div>
      </div>
      {error !== null && (
        <p role="alert" className="mt-3 text-xs">
          {error}
        </p>
      )}
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
function ResumeBody({ card }: { card: StudentAssignmentCard }) {
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
            left: countdown(deadlineAt, new Date()),
          })}
    </p>
  );
}

/** A number, or "Chờ chấm", or nothing -- score is absent when the policy hides it. */
function Outcome({ card, locale }: { card: StudentAssignmentCard; locale: Locale }) {
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
