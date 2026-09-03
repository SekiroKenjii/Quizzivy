import { useState } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Check,
  Copy,
  Flag,
  Headphones,
  Info,
  Maximize,
  Repeat,
  Save,
  Timer,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { enterFullscreen } from "@/features/integrity/fullscreen";
import { startOrResumeAttempt } from "@/features/take-test/api";
import { ApiError } from "@/lib/api/errors";
import { formatTime } from "@/lib/i18n/datetime";
import { getMyAssignment, type StudentAssignmentDetail } from "../api";
import { duringRules, type Rule } from "../studentRules";
import { shortDate } from "../studentTime";

/**
 * S-04: the contract before the clock starts.
 *
 * §10.2 says rules are "announced, visible, never silent", and this is the
 * only screen where they can be read without time pressure -- so every rule
 * the engine will enforce is stated here, from the same sentences the
 * teacher saw in G-01's preview. "Sau khi nộp" is a permission list, ticks
 * and crosses in one block, because a student who expects the answer key and
 * does not get it assumes the app is broken.
 */
export default function AssignmentIntroPage() {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();

  const detail = useQuery({
    queryKey: ["my-assignment", id],
    queryFn: ({ signal }) => getMyAssignment(id ?? "", signal),
    enabled: id !== undefined,
  });

  if (detail.isPending) {
    return (
      <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
        {t("common.loading")}
      </p>
    );
  }
  if (detail.isError) {
    return (
      <div className="space-y-3">
        <p role="alert" className="text-sm">
          {t("student.loadFailed")}
        </p>
        <Button variant="outline" size="sm" onClick={() => void detail.refetch()}>
          {t("common.retry")}
        </Button>
      </div>
    );
  }

  const a = detail.data;
  const rules = duringRules(
    {
      durationMinutes: a.durationMinutes,
      maxAttempts: a.maxAttempts,
      review: a.review,
      integrity: a.integrity,
      ...(a.hasAudio ? { audio: { maxPlays: a.audioMaxPlays ?? null } } : {}),
    },
    t,
  );
  const live = a.hasLiveAttempt === true;
  const attemptNo = Math.min(a.attemptsUsed + 1, a.maxAttempts);

  const provenance = introProvenance(a, t);

  return (
    <div className="space-y-4">
      <div>
        {provenance !== null && (
          <p className="text-muted-foreground text-xs">{provenance}</p>
        )}
        <h1 className="mt-1 text-xl leading-snug font-semibold tracking-tight">
          {a.testTitle}
        </h1>
      </div>

      <Card className="grid grid-cols-2 gap-x-4 gap-y-3 p-5">
        <Fact label={t("student.intro.duration")}>
          {t("student.minutes", { count: a.durationMinutes })}
        </Fact>
        <Fact label={t("student.intro.questions")}>
          {t("student.intro.questionsValue", {
            questions: t("student.questions", { count: a.questionCount }),
            points: decimal(a.totalPoints),
          })}
        </Fact>
        <Fact label={t("student.intro.attempts")}>
          {t("student.intro.attemptsValue", { n: attemptNo, total: a.maxAttempts })}
        </Fact>
        <Fact label={t("student.intro.closes")}>
          {t("student.intro.closesValue", {
            time: formatTime(a.closesAt),
            date: shortDate(a.closesAt),
          })}
        </Fact>
      </Card>

      <Card className="gap-0 p-5">
        <h2 className="text-sm font-semibold">{t("student.intro.during")}</h2>
        <ul className="mt-3 space-y-2.5">
          {rules.map((rule) => (
            <li key={rule.kind} className="flex gap-2.5">
              <RuleIcon kind={rule.kind} />
              <p className="text-sm leading-relaxed">{rule.text}</p>
            </li>
          ))}
        </ul>
      </Card>

      <Card className="gap-0 p-5">
        <h2 className="text-sm font-semibold">{t("student.intro.after")}</h2>
        <ul className="mt-3 space-y-2">
          <Permission on={a.review.showScore} yes="seeScore" no="notSeeScore" />
          <Permission
            on={a.review.showCorrectAnswers}
            yes="seeCorrect"
            no="notSeeCorrect"
          />
          <Permission
            on={a.review.showExplanations}
            yes="seeExplanations"
            no="notSeeExplanations"
          />
          {a.showsTranscript && (
            <Permission on yes="seeTranscript" no="seeTranscript" />
          )}
        </ul>
      </Card>

      <StartControl assignment={a} live={live} />
    </div>
  );
}

/** "IELTS Foundation · Cô Thương", whichever half the server could name. */
function introProvenance(a: StudentAssignmentDetail, t: TFunction): string | null {
  if (a.className != null && a.teacherName != null) {
    return t("student.intro.classAndTeacher", {
      className: a.className,
      teacher: a.teacherName,
    });
  }
  return a.className ?? a.teacherName ?? null;
}

/** "30", not "30.00". */
function decimal(value: number): string {
  return new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 2 }).format(value);
}

function Fact({ label, children }: { label: string; children: string }) {
  return (
    <div>
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="text-sm font-medium">{children}</p>
    </div>
  );
}

function RuleIcon({ kind }: { kind: Rule["kind"] }) {
  const Icon = {
    clock: Timer,
    attempts: Repeat,
    fullscreen: Maximize,
    copy: Copy,
    focusLoss: Flag,
    audio: Headphones,
    autosave: Save,
    honest: Info,
  }[kind];
  return (
    <Icon className="text-muted-foreground mt-0.5 size-4 shrink-0" aria-hidden="true" />
  );
}

function Permission({ on, yes, no }: { on: boolean; yes: string; no: string }) {
  const { t } = useTranslation();
  return (
    <li
      className={
        on
          ? "flex items-center gap-2.5 text-sm"
          : "text-muted-foreground flex items-center gap-2.5 text-sm"
      }
    >
      {on ? (
        <Check className="text-success size-4" aria-hidden="true" />
      ) : (
        <X className="size-4" aria-hidden="true" />
      )}
      {t(`student.intro.${on ? yes : no}`)}
    </li>
  );
}

/**
 * The button, and the one line under it that students actually need.
 *
 * Fullscreen, when required, is entered by THIS click: browsers grant it only
 * from a gesture (§10.2), so the request goes out before anything is awaited.
 * A refusal changes nothing -- the paper opens, and the exit bar offers the
 * way back.
 */
function StartControl({
  assignment: a,
  live,
}: {
  assignment: StudentAssignmentDetail;
  live: boolean;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const exhausted = !live && a.attemptsUsed >= a.maxAttempts;
  const canStart = live || (a.status === "open" && !exhausted);

  const start = async () => {
    setBusy(true);
    setError(null);
    if (a.integrity.requireFullscreen) void enterFullscreen();
    try {
      const session = await startOrResumeAttempt(a.id);
      await navigate(`/app/attempts/${session.attempt.id}`);
    } catch (cause) {
      setError(
        cause instanceof ApiError ? cause.message : t("student.intro.startFailed"),
      );
      setBusy(false);
    }
  };

  if (!canStart) {
    return (
      <p className="text-muted-foreground text-center text-sm leading-relaxed">
        {exhausted
          ? t("student.intro.exhausted")
          : a.status === "scheduled"
            ? t("student.intro.opensLater", {
                time: formatTime(a.opensAt),
                date: shortDate(a.opensAt),
              })
            : t("student.intro.closed")}
      </p>
    );
  }

  return (
    <div className="space-y-3">
      {error !== null && (
        <p role="alert" className="text-sm">
          {error}
        </p>
      )}
      <Button size="lg" className="w-full" disabled={busy} onClick={() => void start()}>
        {t(live ? "student.resume" : "student.start")}
      </Button>
      {!live && (
        <p className="text-muted-foreground text-center text-xs leading-relaxed">
          {t("student.intro.startNote")}
        </p>
      )}
    </div>
  );
}
