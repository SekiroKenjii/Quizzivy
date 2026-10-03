import { useState } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  ArrowRight,
  CalendarClock,
  CircleCheck,
  ClipboardX,
  Eye,
  Headphones,
  LoaderCircle,
  Maximize,
  Timer,
  type LucideIcon,
} from "lucide-react";
import {
  DeckDialog,
  DeckDialogActions,
  DeckDialogCancel,
} from "@/components/shared/DeckDialog";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { enterFullscreen, fullscreenSupported } from "@/features/integrity/fullscreen";
import { startOrResumeAttempt } from "@/features/take-test/api";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useMinute } from "@/hooks/useTick";
import { ApiError } from "@/lib/api/errors";
import type { Locale } from "@/lib/i18n";
import {
  appDaysUntil,
  clockTime,
  dayDate,
  formatTime,
  weekdayName,
} from "@/lib/i18n/datetime";
import { getMyAssignment, type StudentAssignmentDetail } from "../api";
import { studentRules, type RuleKind } from "../studentRules";

const COLUMN = "mx-auto flex w-full max-w-180 flex-col gap-4.5";
const ACTION =
  "to-bg sticky bottom-0 flex flex-col gap-2 bg-linear-to-b from-transparent to-30% pt-3 pb-1";
const CTA = "text-title h-12.5 w-full gap-2 rounded-[11px]";
const HINT = "text-muted-fg text-meta text-center leading-normal";
const ROWS = ["first", "second", "third", "fourth"] as const;

const ICON: Record<RuleKind, LucideIcon> = {
  availability: CalendarClock,
  timer: Timer,
  fullscreen: Maximize,
  copy: ClipboardX,
  leaving: Eye,
  audio: Headphones,
  score: CircleCheck,
};

type Detail = StudentAssignmentDetail;

function refusal(cause: unknown, t: TFunction): string {
  if (!(cause instanceof ApiError)) return t("student.intro.startFailed");
  if (cause.code === "ATTEMPT_LIMIT_REACHED") return t("student.intro.exhausted");
  if (cause.code === "ASSIGNMENT_NOT_OPEN") return t("student.intro.notOpen");
  return cause.message;
}

function opensLabel(a: Detail, now: Date, locale: Locale, t: TFunction): string {
  const days = appDaysUntil(a.opensAt, now);
  if (days <= 0) return t("student.intro.opensToday", { time: formatTime(a.opensAt) });
  return t("student.intro.opensOn", {
    when: days < 7 ? weekdayName(a.opensAt, locale) : dayDate(a.opensAt, locale),
  });
}

function startBody(a: Detail, now: Date, t: TFunction): string {
  const left = Math.floor((Date.parse(a.closesAt) - now.getTime()) / 60_000);
  return left < a.durationMinutes
    ? t("student.intro.startBodyShort", {
        time: clockTime(a.closesAt, now),
        count: Math.max(1, left),
      })
    : t("student.intro.startBody", { minutes: a.durationMinutes });
}

/**
 * AssignmentIntroPage is what a student reads before a test, as the design
 * deck draws it: the class and the title, three facts, "Before you start"
 * generated from the stored policy and dates, and one action that stays at
 * the bottom of the page. "Start test" asks "Start now?" first; its Start
 * button is the click that enters fullscreen when the policy asks for it, and
 * "Continue test" does the same in its own click, with no question. A test
 * that has not opened shows when it opens on a button that does nothing, and
 * a closed or used-up test says so in words. The page reads the assignment
 * afresh each time it opens, and every minute while it waits to open.
 */
export default function AssignmentIntroPage() {
  const { t, i18n } = useTranslation();
  const locale = i18n.language as Locale;
  const { id } = useParams<{ id: string }>();
  const wide = useMediaQuery("(min-width: 768px)");
  const detail = useQuery({
    queryKey: ["my-assignment", id],
    queryFn: ({ signal }) => getMyAssignment(id ?? "", signal),
    enabled: id !== undefined,
    staleTime: 0,
    refetchInterval: (query) =>
      query.state.data?.status === "scheduled" ? 60_000 : false,
  });
  useMinute(true);
  const now = new Date();
  const back = wide ? (
    <Link
      to="/app"
      className="text-muted-fg hover:text-fg text-ui inline-flex items-center gap-1.5 self-start leading-4.5"
    >
      <ArrowLeft aria-hidden="true" className="size-[15px]" />
      {t("student.shell.home")}
    </Link>
  ) : null;

  if (detail.data === undefined) {
    const missing =
      detail.error instanceof ApiError &&
      (detail.error.status === 404 || detail.error.status === 403);
    return (
      <div className={COLUMN}>
        {back}
        {detail.isPending && <IntroSkeleton />}
        {!detail.isPending && missing && (
          <EmptyState
            action={
              <Button asChild>
                <Link to="/app">{t("student.shell.home")}</Link>
              </Button>
            }
          >
            {t("student.intro.notFound")}
          </EmptyState>
        )}
        {!detail.isPending && !missing && (
          <LoadError error={detail.error} onRetry={() => void detail.refetch()}>
            {t("student.loadFailed")}
          </LoadError>
        )}
      </div>
    );
  }

  const a = detail.data;
  const live = a.hasLiveAttempt === true;
  const used = Math.min(a.attemptsUsed + (live ? 1 : 0), a.maxAttempts);
  const rules = studentRules(
    {
      review: a.review,
      integrity: a.integrity,
      running: live,
      fullscreenSupported: fullscreenSupported(),
      ...(a.status === "closed"
        ? {}
        : {
            window: {
              opensAt: a.opensAt,
              closesAt: a.closesAt,
              upcoming: a.status === "scheduled",
            },
          }),
      ...(a.hasAudio
        ? {
            audio: {
              maxPlays: a.audioMaxPlays ?? null,
              shared: a.hasSharedAudio === true,
            },
          }
        : {}),
    },
    t,
    locale,
    now,
  );
  const facts = [
    {
      label: t("student.intro.duration"),
      value: t("student.home.minutes", { count: a.durationMinutes }),
    },
    { label: t("student.intro.questions"), value: String(a.questionCount) },
    {
      label: t("student.intro.attempts"),
      value: t("student.intro.attemptsValue", { used, max: a.maxAttempts }),
    },
  ];

  return (
    <div className={COLUMN}>
      {back}
      <div className="flex flex-col gap-1.5">
        {a.className != null && (
          <span className="text-muted-fg text-sm">{a.className}</span>
        )}
        <h1 className="text-stat min-[768px]:text-h1-student font-semibold tracking-[-0.02em] text-balance break-words">
          {a.testTitle}
        </h1>
      </div>
      <dl className="bg-border grid grid-cols-3 gap-px overflow-hidden rounded-xl border">
        {facts.map((fact) => (
          <div
            key={fact.label}
            className="bg-card min-w-0 px-3 py-3.5 min-[360px]:px-4"
          >
            <dt className="text-muted-fg text-meta leading-normal break-words">
              {fact.label}
            </dt>
            <dd className="mt-0.5 text-lg leading-normal font-semibold break-words">
              {fact.value}
            </dd>
          </div>
        ))}
      </dl>
      <section className="bg-card shadow-card overflow-hidden rounded-xl border">
        <h2 className="text-body border-b px-4 py-3.5 leading-normal font-semibold">
          {t("student.intro.before")}
        </h2>
        <ul>
          {rules.map((rule) => {
            const Icon = ICON[rule.kind];
            return (
              <li
                key={rule.id}
                className="flex items-start gap-3 border-t px-4 py-3 first:border-t-0"
              >
                <span
                  aria-hidden="true"
                  className="bg-muted grid size-7 flex-none place-items-center rounded-md"
                >
                  <Icon className="size-[15px]" />
                </span>
                <p className="pt-[3px] text-base leading-[1.55] text-pretty">
                  {rule.text}
                </p>
              </li>
            );
          })}
        </ul>
      </section>
      <Action key={a.id} assignment={a} now={now} />
    </div>
  );
}

function IntroSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="flex flex-col gap-4.5"
    >
      <div className="flex flex-col gap-2">
        <Skeleton className="h-3.5 w-32 rounded-sm" />
        <Skeleton className="h-7 w-[70%] rounded-sm" />
      </div>
      <Skeleton className="h-[4.75rem] rounded-xl" />
      <div className="rounded-xl border">
        {ROWS.map((row) => (
          <div
            key={row}
            className="flex items-center gap-3 border-t px-4 py-3 first:border-t-0"
          >
            <Skeleton className="size-7 rounded-md" />
            <Skeleton className="h-3.5 w-[65%] rounded-sm" />
          </div>
        ))}
      </div>
      <Skeleton className="h-12.5 rounded-[11px]" />
    </div>
  );
}

function Action({ assignment: a, now }: Readonly<{ assignment: Detail; now: Date }>) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language as Locale;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [asked, setAsked] = useState<Date | null>(null);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const live = a.hasLiveAttempt === true;

  const start = async () => {
    if (busy) return;
    if (a.integrity.requireFullscreen) void enterFullscreen();
    setBusy(true);
    setError(null);
    try {
      const session = await startOrResumeAttempt(a.id);
      await navigate(`/app/attempts/${session.attempt.id}`);
      queryClient.removeQueries({ queryKey: ["my-assignment", a.id] });
    } catch (cause) {
      setError(refusal(cause, t));
      setOpen(false);
      setBusy(false);
      void queryClient.invalidateQueries({ queryKey: ["my-assignment", a.id] });
    }
  };

  const alert = error !== null && (
    <p role="alert" className="text-danger-ink text-center text-sm">
      {error}
    </p>
  );

  if (live)
    return (
      <div className={ACTION}>
        {alert}
        <Button
          size="xl"
          className={CTA}
          aria-busy={busy || undefined}
          onClick={() => void start()}
        >
          {busy && (
            <LoaderCircle aria-hidden="true" className="size-[18px] animate-spin" />
          )}
          {t("student.resume")}
          <ArrowRight aria-hidden="true" className="size-[18px]" />
        </Button>
        <span className={HINT}>
          {a.liveDeadlineAt == null
            ? t("student.intro.startHint")
            : t("student.intro.resumeHint", { time: clockTime(a.liveDeadlineAt, now) })}
        </span>
      </div>
    );

  if (a.attemptsUsed >= a.maxAttempts || a.status === "closed")
    return (
      <div className={ACTION}>
        <p className="bg-muted text-muted-fg grid min-h-12.5 place-items-center rounded-[11px] px-4 py-2 text-center text-base font-medium">
          {t(
            a.attemptsUsed >= a.maxAttempts
              ? "student.intro.exhausted"
              : "student.intro.closed",
          )}
        </p>
      </div>
    );

  if (a.status !== "open")
    return (
      <div className={ACTION}>
        <Button
          size="xl"
          aria-disabled="true"
          className={`${CTA} bg-muted text-muted-fg hover:bg-muted cursor-default`}
        >
          {opensLabel(a, now, locale, t)}
          <ArrowRight aria-hidden="true" className="size-[18px]" />
        </Button>
        <span className={HINT}>{t("student.intro.opensHint")}</span>
      </div>
    );

  const from = asked !== null && asked > now ? asked : now;

  return (
    <div className={ACTION}>
      {alert}
      <Button
        size="xl"
        className={CTA}
        onClick={() => {
          setAsked(new Date());
          setOpen(true);
        }}
      >
        {t("student.start")}
        <ArrowRight aria-hidden="true" className="size-[18px]" />
      </Button>
      <span className={HINT}>{t("student.intro.startHint")}</span>
      <DeckDialog
        open={open}
        onOpenChange={(next) => {
          if (!next && !busy) setOpen(false);
        }}
        title={t("student.intro.startTitle")}
        description={asked === null ? "" : startBody(a, from, t)}
      >
        <DeckDialogActions>
          <DeckDialogCancel
            aria-disabled={busy || undefined}
            onClick={() => {
              if (!busy) setOpen(false);
            }}
          >
            {t("student.intro.startCancel")}
          </DeckDialogCancel>
          <Button size="lg" aria-busy={busy || undefined} onClick={() => void start()}>
            {busy && (
              <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
            )}
            {t("student.intro.startConfirm")}
          </Button>
        </DeckDialogActions>
      </DeckDialog>
    </div>
  );
}
