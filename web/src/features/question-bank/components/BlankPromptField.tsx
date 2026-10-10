import { lazy, Suspense, useRef, useState } from "react";
import type { Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";
import { questionGaps } from "@/components/shared/content/gaps";
import { markdownToQuestionContent } from "@/components/shared/content/editor/markdown";
import { questionContentToMarkdown } from "@/components/shared/content/editor/markdownSerializer";
import { contentPlainText } from "@/components/shared/content/plainText";
import { isQuestionContent } from "@/components/shared/content/questionContent";
import { bindLegacyBlanks } from "../blankContent";
import { blankSlots } from "../blankSlots";
import type { QuestionValues } from "../questionSchema";
import { MarkdownProseEditor } from "./MarkdownProseEditor";
import { ConversionPanel, ProseModeHeader } from "./ProseMode";
import { focusOpener, useProseMode, type ProseMode } from "../proseMode";

const RichBlankEditor = lazy(() =>
  import("./RichBlankEditor").then((module) => ({ default: module.RichBlankEditor })),
);

const ID = "question-prompt";
const PREVIEW_PLUGINS = [blankSlots];

/**
 * BlankPromptField is a fill-in-the-blank prompt in the form it is stored in:
 * rich content whose gaps bind the answers, or Markdown with `{{n}}`
 * placeholders. "Markdown" is disabled while the prompt holds a gap. Only
 * "Switch to Markdown" and "Apply conversion" change the stored form.
 * `onEditor` receives the rich editor while it is live and `null` otherwise.
 */
export function BlankPromptField({
  value,
  onChange,
  onEditor,
  placeholder,
}: Readonly<{
  value: QuestionValues;
  onChange: (value: QuestionValues) => void;
  onEditor?: ((editor: Editor | null) => void) | undefined;
  placeholder?: string | undefined;
}>) {
  const { t } = useTranslation();
  const header = useRef<HTMLDivElement>(null);
  const [moved, setMoved] = useState(false);
  const field = useProseMode((): ProseMode =>
    value.promptContent != null || !value.prompt.trim() ? "rich" : "markdown",
  );
  const label = t("questionEditor.prompt");
  const content = value.promptContent;
  const holdsGaps = content != null && questionGaps(content).length > 0;
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
          id={ID}
          label={label}
          mode={field.mode}
          onMode={field.ask}
          markdownBlocked={holdsGaps ? t("proseMode.gapsBlockMarkdown") : undefined}
        />
      </div>
      {field.mode === "rich" ? (
        <>
          <p className="text-muted-fg mb-1.5 text-xs">
            {t("questionEditor.richBlankHint")}
          </p>
          <Suspense
            fallback={
              <Skeleton
                className="h-40 w-full rounded-[10px]"
                aria-label={t("contentEditor.loading")}
              />
            }
          >
            <RichBlankEditor
              key="rich"
              value={value}
              leaving={field.step === "leaving" && !holdsGaps}
              focusOnMount={moved}
              onCancelLeave={cancel}
              onConfirmLeave={() => {
                if (content != null && isQuestionContent(content))
                  onChange({
                    ...value,
                    prompt: questionContentToMarkdown(content),
                    promptContent: null,
                    blanks: value.blanks.map((blank) => ({ ...blank, gapId: null })),
                  });
                switchTo("markdown");
              }}
              onChange={onChange}
              onEditor={onEditor}
              placeholder={placeholder}
            />
          </Suspense>
        </>
      ) : (
        <MarkdownProseEditor
          key="markdown"
          id={ID}
          label={label}
          value={value.prompt}
          onChange={(prompt) => onChange({ ...value, prompt })}
          minHeight={96}
          fontSize={15}
          placeholder={placeholder}
          previewPlugins={PREVIEW_PLUGINS}
          focusOnMount={moved}
          replacement={
            field.step === "converting" ? (
              <ConversionPanel
                convert={() => {
                  const document = markdownToQuestionContent(value.prompt);
                  const bound = document && bindLegacyBlanks(document, value.blanks);
                  return (
                    bound ??
                    t(
                      document
                        ? "questionEditor.blankConversionBlocked"
                        : "questionEditor.proseConversionBlocked",
                    )
                  );
                }}
                onKeep={cancel}
                onApply={(conversion) => {
                  onChange({
                    ...value,
                    prompt: contentPlainText(conversion.content),
                    promptContent: conversion.content,
                    blanks: conversion.blanks,
                  });
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
