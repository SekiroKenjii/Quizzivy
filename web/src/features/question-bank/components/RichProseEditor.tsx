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
import {
  isQuestionContent,
  type QuestionContent,
} from "@/components/shared/content/questionContent";

/** RichProseEditor previews a bounded Markdown conversion before applying it to the existing save coordinator. */
export function RichProseEditor({
  text,
  content,
  id,
  label,
  onChange,
  onClose,
}: Readonly<{
  text: string;
  content: QuestionContent | null;
  id: string;
  label: string;
  onChange: (text: string, content: QuestionContent | null) => void;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const [initial] = useState(() => content ?? markdownToQuestionContent(text));
  const [previewing, setPreviewing] = useState(content == null);
  const [removing, setRemoving] = useState(false);
  if (!initial)
    return (
      <div className="flex flex-col gap-3">
        <Alert>
          <AlertDescription>
            {t("questionEditor.proseConversionBlocked")}
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
        <ContentView document={initial} className="rounded-md border p-4" />
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose}>
            {t("questionEditor.keepMarkdown")}
          </Button>
          <Button
            type="button"
            onClick={() => {
              onChange(contentPlainText(initial), initial);
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
      <ContentEditor
        initialContent={initial}
        id={id}
        label={label}
        profile="question"
        onChange={(document) => {
          if (isQuestionContent(document))
            onChange(contentPlainText(document), document);
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
                  onChange(plainTextMarkdown(text), null);
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
