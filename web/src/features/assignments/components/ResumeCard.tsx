import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Timer } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { startOrResumeAttempt } from "@/features/take-test/api";
import { useTick } from "@/hooks/useTick";
import { ApiError } from "@/lib/api/errors";
import { notify } from "@/lib/toast";
import { clockTime, formatTime, sameAppDay } from "@/lib/i18n/datetime";
import type { StudentAssignmentCard } from "../api";
import { minutesLeft } from "../studentHome";
import { HOME_PILL } from "./homeStyles";

/**
 * ResumeCard is Home's way back into the attempt in progress. It says how
 * long is left, how much is answered and when the attempt closes, and its
 * button goes straight to the paper: the rules were read when the attempt
 * began. The minutes repaint as they pass; nothing is fetched to do so. When
 * the server refuses to resume, the lists are read again, so a card for an
 * attempt that has ended does not stay; the reason is a toast, because it has
 * to outlive the card.
 */
export function ResumeCard({ card }: Readonly<{ card: StudentAssignmentCard }>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [busy, setBusy] = useState(false);
  const deadline = card.liveDeadlineAt ?? null;
  useTick(deadline !== null);
  const now = new Date();
  const closes = deadline ?? card.closesAt;
  const answered = card.liveAnsweredCount ?? 0;
  const share =
    card.questionCount > 0 ? Math.round((answered / card.questionCount) * 100) : 0;
  const meta = [
    card.className,
    t("student.home.answered", { answered, total: card.questionCount }),
    sameAppDay(closes, now)
      ? t("student.home.closesToday", { time: formatTime(closes) })
      : t("student.home.closesOn", { when: clockTime(closes, now) }),
  ]
    .filter((part) => part != null && part !== "")
    .join(" · ");

  const resume = async () => {
    setBusy(true);
    try {
      const session = await startOrResumeAttempt(card.id);
      await navigate(`/app/attempts/${session.attempt.id}`);
    } catch (cause) {
      notify.error(
        cause instanceof ApiError ? cause.message : t("student.intro.startFailed"),
      );
      setBusy(false);
      void queryClient.invalidateQueries({ queryKey: ["my-assignments"] });
    }
  };

  return (
    <section className="bg-card shadow-card flex flex-wrap items-center gap-4.5 rounded-2xl border p-5">
      <div className="flex min-w-0 flex-[1_1_280px] flex-col gap-1.5">
        <Badge variant="info" className={`${HOME_PILL} self-start`}>
          <Timer aria-hidden="true" className="size-[13px]" />
          {deadline === null
            ? t("student.home.inProgress")
            : t("student.home.inProgressLeft", { count: minutesLeft(deadline, now) })}
        </Badge>
        <h2 className="text-[1.1875rem] leading-[1.3] font-semibold tracking-[-0.01em] break-words">
          {card.testTitle}
        </h2>
        <p className="text-muted-fg text-ui leading-normal">{meta}</p>
        <div
          aria-hidden="true"
          className="bg-muted mt-1.5 h-1.5 max-w-105 overflow-hidden rounded-full"
        >
          <span className="bg-brand block h-full" style={{ width: `${share}%` }} />
        </div>
      </div>
      <Button
        size="xl"
        className="flex-[1_1_100%] px-5.5 min-[768px]:flex-none"
        disabled={busy}
        onClick={() => void resume()}
      >
        {t("student.resume")}
        <ArrowRight aria-hidden="true" className="size-[17px]" />
      </Button>
    </section>
  );
}
