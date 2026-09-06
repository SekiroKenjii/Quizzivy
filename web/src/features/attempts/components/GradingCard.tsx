import { useState } from "react";
import { useTranslation } from "react-i18next";
import { CornerDownLeft, Minus, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { AdminQuestion, ReviewAnswer } from "../api";

/**
 * G-03's marking loop for one short answer: points three ways (steppers,
 * typing, the common values), a comment the student will read, save and move
 * on. Saved per question, so a half-graded paper survives a refresh.
 */
export function GradingCard({
  question,
  answer,
  pending,
  error,
  onSave,
  onSkip,
}: Readonly<{
  question: AdminQuestion;
  answer: ReviewAnswer | undefined;
  pending: boolean;
  error: string | null;
  onSave: (points: number, comment: string | null) => void;
  onSkip: () => void;
}>) {
  const { t } = useTranslation();
  const max = question.points;
  const [points, setPoints] = useState<string>(
    answer?.manualScore == null ? "" : String(answer.manualScore),
  );
  const [comment, setComment] = useState(answer?.graderComment ?? "");

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
  const save = () => {
    if (!valid) return;
    onSave(value, comment.trim() === "" ? null : comment.trim());
  };

  return (
    <Card>
      <CardContent className="space-y-3">
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
        {error !== null && (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        )}
        <div className="flex items-center gap-2 pt-1">
          <Button type="button" disabled={!valid || pending} onClick={save}>
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
