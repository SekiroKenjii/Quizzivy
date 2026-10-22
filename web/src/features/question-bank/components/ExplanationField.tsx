import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import {
  writeAiExplanation,
  type AiExplanationRequest,
  type AiLanguage,
  type AiUsage,
} from "@/features/ai/api";
import { AiEntryButton } from "@/features/ai/components/AiEntryButton";
import { credits } from "@/features/ai/credits";
import { useAiLanguage } from "@/features/ai/useAiAvailable";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";
import { withExplanation } from "../aiDrafts";
import type { QuestionAi } from "../questionAi";
import type { QuestionValues } from "../questionSchema";
import { aiFailureText, useAiSuggestion } from "../useAiSuggestion";
import { AiExplanationPanel } from "./AiExplanationPanel";
import type { QuestionExplanationContent } from "@/components/shared/content/questionContent";
import { QuestionProseField } from "./QuestionProseField";

const WRITE =
  "bg-card text-meta h-7 gap-1.5 rounded-seg px-2.5 [&_svg:not([class*='size-'])]:size-3.25";

type Written = { text: string; usage: AiUsage; request: AiExplanationRequest };

/**
 * ExplanationField is a question's explanation in the content editor, with
 * the rule for when students see it as its hint. Given `ai`, it also offers
 * "Write with AI" (V-10) in the label row, drawn only while the assistant is
 * available and disabled with the reason while the teacher is paused or out of
 * credits; the explanation is written in the teacher's preferred language,
 * and "Use this" writes it into this field through `onChange`, so the page's
 * own save keeps it.
 */
export function ExplanationField({
  value,
  onChange,
  ai,
}: Readonly<{
  value: QuestionValues;
  onChange: (value: QuestionValues) => void;
  ai?: QuestionAi | undefined;
}>) {
  if (ai === undefined) return <ExplanationProse value={value} onChange={onChange} />;
  return <ExplanationWithAi ai={ai} value={value} onChange={onChange} />;
}

function ExplanationProse({
  value,
  action,
  onChange,
}: Readonly<{
  value: QuestionValues;
  action?: ReactNode;
  onChange: (value: QuestionValues) => void;
}>) {
  const { t } = useTranslation();
  return (
    <QuestionProseField
      id="question-explanation"
      text={value.explanation ?? ""}
      content={value.explanationContent}
      label={t("questionEditor.explanation")}
      hint={t("questionEditor.explanationHint")}
      placeholder={t("questionEditor.explanationPlaceholder")}
      action={action}
      onChange={(explanation, explanationContent) =>
        onChange({
          ...value,
          explanation,
          explanationContent: explanationContent as QuestionExplanationContent | null,
        })
      }
    />
  );
}

function ExplanationWithAi({
  ai,
  value,
  onChange,
}: Readonly<{
  ai: QuestionAi;
  value: QuestionValues;
  onChange: (value: QuestionValues) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const language = useAiLanguage();
  const suggestion = useAiSuggestion<Written>(ai, value);
  const [revision, setRevision] = useState(0);
  const trigger = useRef<HTMLButtonElement>(null);
  const { stage } = suggestion;
  const other: AiLanguage = language === "vi" ? "en" : "vi";
  const written = stage.name === "done" ? stage.result : null;

  useEffect(() => {
    if (revision > 0) trigger.current?.focus();
  }, [revision]);

  function write(request: AiExplanationRequest) {
    trigger.current?.focus();
    void suggestion.ask(async (id, signal) => ({
      ...(await writeAiExplanation(id, request, signal)),
      request,
    }));
  }

  function close() {
    trigger.current?.focus();
    suggestion.dismiss();
  }

  function use(text: string) {
    onChange(withExplanation(value, text));
    setRevision((current) => current + 1);
    close();
    notify.success(t("ai.explain.used"));
  }

  const label = (() => {
    if (stage.name === "running") return t("ai.explain.writing");
    if (stage.name === "done") return t("ai.explain.again");
    return t("ai.explain.write");
  })();

  return (
    <div className="flex flex-col gap-2.5">
      <ExplanationProse
        key={revision}
        value={value}
        onChange={onChange}
        action={
          suggestion.offered ? (
            <AiEntryButton
              ref={trigger}
              icon={Sparkles}
              label={label}
              blocked={suggestion.block}
              busy={stage.name === "running"}
              size="md"
              className={WRITE}
              onClick={() => write({ language })}
            />
          ) : undefined
        }
      />
      <div aria-live="polite" className="empty:hidden">
        {suggestion.offered && (written !== null || stage.name === "failed") ? (
          <AiExplanationPanel
            text={written?.text ?? null}
            error={stage.name === "failed" ? aiFailureText(stage.failure, t) : null}
            note={
              written === null
                ? null
                : t("ai.credits.used", {
                    credits: credits(written.usage.credits, locale),
                    left: credits(written.usage.left, locale),
                  })
            }
            other={other}
            onUse={() => {
              if (written !== null) use(written.text);
            }}
            onShorter={() =>
              write({
                language: written?.request.language ?? language,
                length: "short",
              })
            }
            onLanguage={() => write({ language: other })}
            onDiscard={close}
          />
        ) : null}
      </div>
    </div>
  );
}
