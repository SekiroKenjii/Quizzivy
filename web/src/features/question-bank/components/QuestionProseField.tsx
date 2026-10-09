import { lazy, Suspense, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";
import { markdownToQuestionContent } from "@/components/shared/content/editor/markdown";
import { questionContentToMarkdown } from "@/components/shared/content/editor/markdownSerializer";
import { contentPlainText } from "@/components/shared/content/plainText";
import {
  isQuestionContent,
  type QuestionContent,
  type QuestionPromptContent,
} from "@/components/shared/content/questionContent";
import { MarkdownProseEditor } from "./MarkdownProseEditor";
import { ConversionPanel, ProseModeHeader } from "./ProseMode";
import { focusOpener, useProseMode, type ProseMode } from "../proseMode";

const RichProseEditor = lazy(() =>
  import("./RichProseEditor").then((module) => ({ default: module.RichProseEditor })),
);

const PROMPT = { minHeight: 96, fontSize: 15 };
const EXPLANATION = { minHeight: 84, fontSize: 14 };

/**
 * QuestionProseField is a prompt's or an explanation's field in the form it
 * is stored in: rich content in the rich editor, a Markdown string in the
 * Markdown editor; a field with no text opens as rich text. Only "Switch to
 * Markdown" and "Apply conversion" change the stored form.
 */
export function QuestionProseField({
  text,
  content,
  id,
  label,
  hint,
  prompt = false,
  clearOnFocus = false,
  onChange,
}: Readonly<{
  text: string;
  content?: QuestionPromptContent | null | undefined;
  id: string;
  label: string;
  hint?: string | undefined;
  prompt?: boolean;
  clearOnFocus?: boolean;
  onChange: (text: string, content: QuestionContent | null) => void;
}>) {
  const { t } = useTranslation();
  const header = useRef<HTMLDivElement>(null);
  const [moved, setMoved] = useState(false);
  const field = useProseMode((): ProseMode =>
    content != null || !text.trim() ? "rich" : "markdown",
  );
  const size = prompt ? PROMPT : EXPLANATION;
  const cancel = () => {
    field.cancel();
    focusOpener(header.current);
  };
  const switchTo = (mode: ProseMode) => {
    setMoved(true);
    field.finish(mode);
  };
  return (
    <div>
      <div ref={header}>
        <ProseModeHeader
          id={id}
          label={label}
          hint={hint}
          mode={field.mode}
          onMode={field.ask}
        />
      </div>
      {field.mode === "rich" ? (
        <Suspense
          fallback={
            <Skeleton
              className="h-40 w-full rounded-[10px]"
              aria-label={t("contentEditor.loading")}
            />
          }
        >
          <RichProseEditor
            key="rich"
            content={content ?? null}
            id={id}
            label={label}
            {...size}
            leaving={field.step === "leaving"}
            focusOnMount={moved}
            onCancelLeave={cancel}
            onConfirmLeave={() => {
              if (content != null && isQuestionContent(content))
                onChange(questionContentToMarkdown(content), null);
              switchTo("markdown");
            }}
            onChange={onChange}
          />
        </Suspense>
      ) : (
        <MarkdownProseEditor
          key="markdown"
          id={id}
          label={label}
          value={text}
          onChange={(value) => onChange(value, null)}
          {...size}
          clearOnFocus={prompt && clearOnFocus}
          focusOnMount={moved}
          replacement={
            field.step === "converting" ? (
              <ConversionPanel
                convert={() => {
                  const document = markdownToQuestionContent(text);
                  return document
                    ? { content: document }
                    : t("questionEditor.proseConversionBlocked");
                }}
                onKeep={cancel}
                onApply={({ content: document }) => {
                  onChange(contentPlainText(document), document);
                  switchTo("rich");
                }}
              />
            ) : undefined
          }
        />
      )}
    </div>
  );
}
