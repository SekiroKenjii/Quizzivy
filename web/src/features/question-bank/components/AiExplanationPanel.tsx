import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Check, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AiCreditNote } from "@/features/ai/components/AiCreditNote";
import type { AiLanguage } from "@/features/ai/api";

const ACTION =
  "text-meta h-7.5 gap-1.5 rounded-seg px-2.5 font-medium [&_svg:not([class*='size-'])]:size-3.25";

/**
 * AiExplanationPanel is "Suggested explanation" under the Explanation field
 * (V-10): the text the assistant wrote with "Use this", "Shorter", the other
 * language than the teacher's preference ("In Vietnamese" or "In English")
 * and "Discard", or, when the call failed, the sentence that says why with
 * "Discard" only. `note` is what the call cost and what is left. The panel
 * saves nothing: "Use this" hands the text to the field.
 */
export function AiExplanationPanel({
  text,
  error,
  note,
  other,
  onUse,
  onShorter,
  onLanguage,
  onDiscard,
}: Readonly<{
  text: string | null;
  error: string | null;
  note: string | null;
  other: AiLanguage;
  onUse: () => void;
  onShorter: () => void;
  onLanguage: () => void;
  onDiscard: () => void;
}>) {
  const { t } = useTranslation();
  const title = useId();
  return (
    <section
      aria-labelledby={title}
      className="bg-brand-soft flex flex-col gap-2.5 rounded-[10px] border px-3.5 py-3"
    >
      <h3
        id={title}
        className="text-brand-ink text-meta flex items-center gap-1.5 leading-normal font-semibold"
      >
        <Sparkles aria-hidden="true" className="size-3.25 flex-none" />
        {t("ai.explain.title")}
      </h3>
      {text === null ? (
        <p className="text-sm leading-normal">{error}</p>
      ) : (
        <p className="text-base leading-[1.6] text-pretty whitespace-pre-line">
          {text}
        </p>
      )}
      <div className="flex flex-wrap gap-1.5">
        {text === null ? null : (
          <>
            <Button size="md" className={`${ACTION} px-3`} onClick={onUse}>
              <Check aria-hidden="true" />
              {t("ai.explain.use")}
            </Button>
            <Button variant="outline" size="md" className={ACTION} onClick={onShorter}>
              {t("ai.explain.shorter")}
            </Button>
            <Button variant="outline" size="md" className={ACTION} onClick={onLanguage}>
              {t(other === "vi" ? "ai.explain.inVietnamese" : "ai.explain.inEnglish")}
            </Button>
          </>
        )}
        <Button
          variant="ghost"
          size="md"
          className={`${ACTION} text-muted-fg hover:bg-card hover:text-fg dark:hover:bg-card font-normal`}
          onClick={onDiscard}
        >
          {t("ai.explain.discard")}
        </Button>
      </div>
      {note === null ? null : (
        <div className="flex">
          <AiCreditNote text={note} warning={false} />
        </div>
      )}
    </section>
  );
}
