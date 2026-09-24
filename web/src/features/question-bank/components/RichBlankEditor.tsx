import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { ContentView } from "@/components/shared/content/ContentView";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import {
  markdownToQuestionContent,
  plainTextMarkdown,
} from "@/components/shared/content/editor/markdown";
import { contentPlainText } from "@/components/shared/content/plainText";
import { isQuestionPromptContent } from "@/components/shared/content/questionContent";
import {
  bindLegacyBlanks,
  blankMarkdownProjection,
  reconcileGapBlanks,
  nextBlankOrdinal,
} from "../blankContent";
import type { QuestionValues } from "../questionSchema";

/** RichBlankEditor preserves answer bindings when formatting, moving, undoing or removing gap nodes. */
export function RichBlankEditor({
  value,
  onChange,
  onClose,
}: Readonly<{
  value: QuestionValues;
  onChange: (value: QuestionValues) => void;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const [initial] = useState(() => {
    if (value.promptContent != null)
      return { content: value.promptContent, blanks: value.blanks };
    const document = markdownToQuestionContent(value.prompt);
    return document ? bindLegacyBlanks(document, value.blanks) : null;
  });
  const [previewing, setPreviewing] = useState(value.promptContent == null);
  const [removing, setRemoving] = useState(false);
  if (!initial)
    return (
      <div className="flex flex-col gap-3">
        <Alert>
          <AlertDescription>
            {t("questionEditor.blankConversionBlocked")}
          </AlertDescription>
        </Alert>
        <Button type="button" variant="outline" onClick={onClose}>
          {t("questionEditor.keepMarkdown")}
        </Button>
      </div>
    );
  if (previewing)
    return (
      <div className="flex flex-col gap-3">
        <Alert>
          <AlertDescription>
            {t("questionEditor.proseConversionPreview")}
          </AlertDescription>
        </Alert>
        <ContentView document={initial.content} className="rounded-md border p-4" />
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose}>
            {t("questionEditor.keepMarkdown")}
          </Button>
          <Button
            type="button"
            onClick={() => {
              onChange({
                ...value,
                prompt: contentPlainText(initial.content),
                promptContent: initial.content,
                blanks: initial.blanks,
              });
              setPreviewing(false);
            }}
          >
            {t("questionEditor.applyProseConversion")}
          </Button>
        </div>
      </div>
    );
  return (
    <div className="flex flex-col gap-3">
      <p className="text-muted-foreground text-xs">
        {t("questionEditor.richBlankHint")}
      </p>
      <ContentEditor
        initialContent={initial.content}
        id="question-prompt"
        label={t("questionEditor.prompt")}
        profile="prompt"
        gapLabel={() => String(nextBlankOrdinal(value.blanks))}
        onChange={(document) => {
          if (isQuestionPromptContent(document))
            onChange({
              ...value,
              prompt: contentPlainText(document),
              promptContent: document,
              blanks: reconcileGapBlanks(document, value.blanks),
            });
        }}
      />
      {removing && (
        <Alert>
          <AlertDescription className="flex flex-col gap-3">
            <p>{t("questionEditor.removeProseWarning")}</p>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setRemoving(false)}
              >
                {t("common.cancel")}
              </Button>
              <Button
                type="button"
                onClick={() => {
                  const projected = blankMarkdownProjection(
                    value.promptContent ?? initial.content,
                    value.blanks,
                  );
                  onChange({
                    ...value,
                    prompt: plainTextMarkdown(contentPlainText(projected)),
                    promptContent: null,
                    blanks: value.blanks.map((blank) => ({ ...blank, gapId: null })),
                  });
                  onClose();
                }}
              >
                {t("questionEditor.confirmRemoveProse")}
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      )}
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => setRemoving(true)}
        >
          {t("questionEditor.returnToMarkdown")}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onClose}>
          {t("questionEditor.doneFormatting")}
        </Button>
      </div>
    </div>
  );
}
