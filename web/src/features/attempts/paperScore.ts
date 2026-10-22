import type { TFunction } from "i18next";
import { scoreText } from "@/features/assignments/studentTime";
import { formatBand } from "@/lib/bands";
import type { Locale } from "@/lib/i18n";
import type { AttemptScore } from "./api";

/**
 * paperScoreText is how a roster, the attempt sheet and the review header
 * write a paper's score: "6.0 band" when the server gives the paper a band
 * (every scored question is a graded essay), otherwise "27/30".
 */
export function paperScoreText(
  score: AttemptScore,
  locale: Locale,
  t: TFunction,
): string {
  if (score.band != null)
    return t("review.paperBand", { band: formatBand(score.band, locale) });
  return scoreText(score.earned, score.total, locale, t);
}
