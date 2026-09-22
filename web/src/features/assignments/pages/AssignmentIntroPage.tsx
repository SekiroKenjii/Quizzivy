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
import { Separator } from "@/components/ui/separator";
import { BackLink } from "@/components/shared/BackLink";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { PageAside } from "@/components/shared/PageAside";
import { PanelLabel, PanelRow } from "@/components/shared/PanelLabel";
import { Card } from "@/components/ui/card";
import { enterFullscreen } from "@/features/integrity/fullscreen";
import { startOrResumeAttempt } from "@/features/take-test/api";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { ApiError } from "@/lib/api/errors";
import { formatTime, shortDate } from "@/lib/i18n/datetime";
import { getMyAssignment, type StudentAssignmentDetail } from "../api";
import { duringRules, type Rule } from "../studentRules";

/**
 * S-04: the contract before the clock starts. From 1024px it is S-14: the
 * two cards side by side in the middle, the facts and the start button in
 * F-11's panel, and a text link back where the phone had its arrow.
 */
export default function AssignmentIntroPage() {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();
  const wide = useMediaQuery("(min-width: 1024px)");

  const detail = useQuery({
    queryKey: ["my-assignment", id],
    queryFn: ({ signal }) => getMyAssignment(id ?? "", signal),
    enabled: id !== undefined,
  });

  if (detail.isPending) {
    return <ListSkeleton rows={6} />;
  }
  if (detail.isError) {
    return (
      <LoadError error={detail.error} onRetry={() => void detail.refetch()}>
        {t("student.loadFailed")}
      </LoadError>
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
  const facts = [
    [t("student.intro.duration"), t("student.minutes", { count: a.durationMinutes })],
    [
      t("student.intro.questions"),
      t("student.intro.questionsValue", {
        questions: t("student.questions", { count: a.questionCount }),
        points: decimal(a.totalPoints),
      }),
    ],
    [
      t("student.intro.attempts"),
      t("student.intro.attemptsValue", { n: attemptNo, total: a.maxAttempts }),
    ],
    [
      t("student.intro.closes"),
      t("student.intro.closesValue", {
        time: formatTime(a.closesAt),
        date: shortDate(a.closesAt),
      }),
    ],
  ] as const;

  const during = (
    <Card className="gap-0 p-5">
      <h2 className="text-sm font-semibold">{t("student.intro.during")}</h2>
      <ul className="mt-3 space-y-2.5">
        {rules
          .filter((rule) => rule.kind !== "attempts")
          .map((rule) => (
            <li key={rule.kind} className="flex gap-2.5">
              <RuleIcon kind={rule.kind} />
              <p className="text-sm leading-relaxed">{rule.text}</p>
            </li>
          ))}
      </ul>
    </Card>
  );
  const after = (
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
        {a.showsTranscript && <Permission on yes="seeTranscript" no="seeTranscript" />}
      </ul>
    </Card>
  );

  if (wide) {
    return (
      <div className="space-y-5">
        <div>
          <BackLink to="/app">{t("student.myAssignments")}</BackLink>
          {provenance !== null && (
            <p className="text-muted-foreground mt-3 text-xs">{provenance}</p>
          )}
          <h1 className="mt-1 text-xl leading-snug font-semibold tracking-tight">
            {a.testTitle}
          </h1>
        </div>
        <div className="grid items-start gap-4 xl:grid-cols-[3fr_2fr]">
          {during}
          {after}
        </div>
        <PageAside label={t("student.intro.summary")}>
          <div>
            <PanelLabel>{t("student.intro.summary")}</PanelLabel>
            <div className="space-y-2">
              {facts.map(([label, value]) => (
                <PanelRow key={label} label={label}>
                  {value}
                </PanelRow>
              ))}
            </div>
            {a.maxAttempts > 1 && (
              <p className="text-muted-foreground mt-2 text-xs">
                {t("student.intro.bestScore")}
              </p>
            )}
          </div>
          <Separator />
          <StartControl assignment={a} live={live} />
        </PageAside>
      </div>
    );
  }

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
        {facts.map(([label, value]) => (
          <Fact key={label} label={label}>
            {value}
          </Fact>
        ))}
        {a.maxAttempts > 1 && (
          <p className="text-muted-foreground col-span-2 text-xs">
            {t("student.intro.bestScore")}
          </p>
        )}
      </Card>

      {during}
      {after}

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

function Fact({ label, children }: Readonly<{ label: string; children: string }>) {
  return (
    <div>
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="text-sm font-medium">{children}</p>
    </div>
  );
}

function RuleIcon({ kind }: Readonly<{ kind: Rule["kind"] }>) {
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

function Permission({
  on,
  yes,
  no,
}: Readonly<{ on: boolean; yes: string; no: string }>) {
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

/** The button, and the one line under it that students actually need. */
function StartControl({
  assignment: a,
  live,
}: Readonly<{
  assignment: StudentAssignmentDetail;
  live: boolean;
}>) {
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
        {blockedText(exhausted, a, t)}
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

function blockedText(
  exhausted: boolean,
  a: { readonly status: string; readonly opensAt: string },
  t: TFunction,
): string {
  if (exhausted) return t("student.intro.exhausted");
  if (a.status === "scheduled") {
    return t("student.intro.opensLater", {
      time: formatTime(a.opensAt),
      date: shortDate(a.opensAt),
    });
  }
  return t("student.intro.closed");
}
