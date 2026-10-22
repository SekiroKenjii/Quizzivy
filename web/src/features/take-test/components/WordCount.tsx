import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { countWords } from "@/lib/words";
import { CAPTION } from "./bodyParts";

/**
 * WordCount is the line under a written answer: "12 words", or against a
 * word limit "2 of 3 words", which turns danger with "· too many" past the
 * limit. It never stops the typing; the field names it as its description.
 */
export function WordCount({
  id,
  value,
  limit,
}: Readonly<{ id: string; value: string; limit: number | null | undefined }>) {
  const { t } = useTranslation();
  const words = countWords(value);
  const over = limit != null && words > limit;
  let text = t("takeTest.wordCount", { count: words });
  if (limit != null)
    text = t(over ? "takeTest.wordLimitOver" : "takeTest.wordLimit", {
      words,
      count: limit,
    });
  return (
    <p id={id} className={cn(CAPTION, "tabular-nums", over && "text-danger-ink")}>
      {text}
    </p>
  );
}
