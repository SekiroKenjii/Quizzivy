import { useTranslation } from "react-i18next";
import { ContentImage } from "@/components/shared/content/ContentImage";
import type { components } from "@/lib/api/schema";

type MediaAsset = components["schemas"]["MediaAsset"];

/**
 * QuestionImage is a question's own image, as the engine, the result and the
 * teacher's preview draw it: the picture in R3's frame (DG-80), described for
 * a screen reader by the question's alt text, or by the generic "Question
 * image" when the question has none. It draws nothing for a question whose
 * media is not an image. `onRetry` is offered when the signed URL fails.
 */
export function QuestionImage({
  question,
  onRetry,
}: Readonly<{
  question: Readonly<{ media?: MediaAsset | null; mediaAlt?: string | null }>;
  onRetry?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const media = question.media;
  if (media?.kind !== "image" || !media.url) return null;
  const alt = question.mediaAlt?.trim() ? question.mediaAlt : t("takeTest.imageLabel");
  return <ContentImage src={media.url} alt={alt} onRetry={onRetry} />;
}
