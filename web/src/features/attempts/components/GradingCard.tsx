import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { CornerDownLeft, Minus, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { AdminQuestion, GradeItem, ReviewAnswer } from "../api";
import { useUnsavedSection } from "@/features/settings/unsavedSection";
import {
  markBand,
  markFields,
  markOf,
  markSignature,
  type EssayMark,
} from "../essayMark";
import { pointsSignature, useFocusWithin, useSavedSync } from "../savedSync";
import { EssayBands, GradedElsewhere } from "./BandScale";

/** GradeMarks is a grade item's marks: `points`, or an essay's `band` or `criteria`. */
export type GradeMarks = Omit<GradeItem, "questionId" | "comment">;

/**
 * GradingCard is the marking loop for one short answer: points three ways
 * (steppers, typing, the common values), a comment the student will read, save
 * and move on. An essay takes its band, or a band per rubric criterion,
 * instead, and Save & next looks paused until every criterion has one and,
 * if pressed sooner, says what is missing. Saved per
 * question, so a half-graded paper survives a refresh. While `focusPoints` is
 * set it focuses the points box (an essay's first band), mounted or not, and
 * reports it through `onPointsFocused`, so the caller asks once per move.
 * `error` is a refused save's message for these marks; when it appears with
 * focus lost to the page, focus goes back to the points box. Unsaved marks
 * hold the page's leave guard, and a grade saved elsewhere meanwhile replaces
 * the card's while it is untouched, or is named beside it once it is not.
 */
export function GradingCard({
  question,
  answer,
  pending,
  error,
  focusPoints = false,
  onPointsFocused,
  onSave,
  onSkip,
}: Readonly<{
  question: AdminQuestion;
  answer: ReviewAnswer | undefined;
  pending: boolean;
  error: string | null;
  focusPoints?: boolean;
  onPointsFocused?: () => void;
  onSave: (marks: GradeMarks, comment: string | null) => void;
  onSkip: () => void;
}>) {
  const { t } = useTranslation();
  const pointsInput = useRef<HTMLInputElement>(null);
  const bands = useRef<HTMLDivElement>(null);
  const focused = useEffectEvent(() => onPointsFocused?.());
  useEffect(() => {
    if (!focusPoints) return;
    if (pointsInput.current) pointsInput.current.focus();
    else bands.current?.querySelector<HTMLElement>("[data-score]")?.focus();
    focused();
  }, [focusPoints]);
  const essay = question.type === "essay";
  const rubric = question.rubric ?? [];
  const [mark, setMark] = useState<EssayMark>(() => markOf(answer ?? {}));
  const [missing, setMissing] = useState<string | null>(null);
  const max = question.points;
  const [points, setPoints] = useState<string>(
    answer?.manualScore == null ? "" : String(answer.manualScore),
  );
  const [comment, setComment] = useState(answer?.graderComment ?? "");
  const signed = (marks: string, remark: string) =>
    `${essay ? "band" : "points"}:${marks}|${remark.trim()}`;
  const savedMark = markOf(answer ?? {});
  const saved = signed(
    essay ? markSignature(rubric, savedMark) : String(answer?.manualScore ?? ""),
    answer?.graderComment ?? "",
  );
  const reset = () => {
    setMark(savedMark);
    setPoints(answer?.manualScore == null ? "" : String(answer.manualScore));
    setComment(answer?.graderComment ?? "");
    setMissing(null);
  };
  const [holding, hold] = useFocusWithin();
  const { dirty, elsewhere } = useSavedSync(
    saved,
    signed(essay ? markSignature(rubric, mark) : pointsSignature(points), comment),
    reset,
    holding,
  );
  useUnsavedSection(`grade:${question.id}`, dirty);
  useEffect(() => {
    if (error === null || essay) return;
    const lost =
      document.activeElement === null || document.activeElement === document.body;
    if (lost) pointsInput.current?.focus();
  }, [error, essay]);

  const value = Number(points);
  const valid =
    points.trim() !== "" && Number.isFinite(value) && value >= 0 && value <= max;
  const step = (delta: number) => {
    const next = Math.min(
      max,
      Math.max(0, (Number.isFinite(value) ? value : 0) + delta),
    );
    setPoints(String(Math.round(next * 100) / 100));
  };
  const marked = essay ? markBand(rubric, mark) !== null : valid;
  const save = () => {
    if (!marked) {
      if (essay)
        setMissing(
          t(rubric.length ? "grading.pickCriteriaFirst" : "grading.pickBandFirst"),
        );
      return;
    }
    onSave(
      essay ? markFields(rubric, mark) : { points: value },
      comment.trim() === "" ? null : comment.trim(),
    );
  };

  return (
    <Card>
      <CardContent className="space-y-3" {...hold}>
        {essay ? (
          <div ref={bands}>
            <EssayBands
              rubric={rubric}
              mark={mark}
              error={error ?? missing}
              onChange={(next) => {
                setMissing(null);
                setMark(next);
              }}
            />
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-3">
            <Label htmlFor="grade-points" className="mb-0">
              {t("review.points")}
            </Label>
            <div className="flex items-center gap-1.5">
              <Button
                type="button"
                variant="outline"
                size="icon-sm"
                aria-label={t("review.decrease")}
                onClick={() => step(-0.5)}
              >
                <Minus aria-hidden="true" />
              </Button>
              <Input
                ref={pointsInput}
                id="grade-points"
                inputMode="decimal"
                className="w-16 text-center tabular-nums"
                value={points}
                aria-invalid={points !== "" && !valid}
                onChange={(event) => setPoints(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    save();
                  }
                }}
              />
              <Button
                type="button"
                variant="outline"
                size="icon-sm"
                aria-label={t("review.increase")}
                onClick={() => step(0.5)}
              >
                <Plus aria-hidden="true" />
              </Button>
              <span className="text-muted-foreground text-sm">/ {max}</span>
            </div>
            <div className="ml-3 flex items-center gap-1">
              {[0, max / 2, max].map((quick) => (
                <Button
                  key={quick}
                  type="button"
                  variant="ghost"
                  size="xs"
                  className="text-muted-foreground"
                  onClick={() => setPoints(String(Math.round(quick * 100) / 100))}
                >
                  {Math.round(quick * 100) / 100}
                </Button>
              ))}
            </div>
            <span className="text-muted-foreground ml-auto flex items-center gap-1 text-xs">
              <Kbd>{0}</Kbd>
              {t("review.keyTo")}
              <Kbd>{max}</Kbd> {t("review.keyPoints")} {t("review.keySep")}{" "}
              <Kbd>
                <CornerDownLeft className="size-3" aria-label={t("review.keyEnter")} />
              </Kbd>{" "}
              {t("review.keyNext")}
            </span>
          </div>
        )}
        <div className="space-y-1.5">
          <Label htmlFor="grade-comment">
            {t("review.comment")}{" "}
            <span className="text-muted-foreground font-normal">
              {t("review.commentHint")}
            </span>
          </Label>
          <Textarea
            id="grade-comment"
            className="min-h-16"
            value={comment}
            onChange={(event) => setComment(event.target.value)}
          />
        </div>
        {error !== null && !essay && (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        )}
        {elsewhere && <GradedElsewhere onShow={reset} />}
        <div className="flex items-center gap-2 pt-1">
          <Button
            type="button"
            disabled={pending || (!essay && !marked)}
            aria-disabled={essay && !marked}
            onClick={save}
          >
            {t("review.saveNext")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            className="text-muted-foreground"
            onClick={onSkip}
          >
            {t("review.skip")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
