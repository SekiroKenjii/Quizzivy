import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";
import { Type } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { OptionText } from "@/components/shared/content/OptionText";
import {
  isOptionContent,
  plainOptionContent,
  type OptionContent,
} from "@/components/shared/content/optionContent";
import { contentPlainText } from "@/components/shared/content/plainText";

const ContentEditor = lazy(() =>
  import("@/components/shared/content/editor/ContentEditor").then((module) => ({
    default: module.ContentEditor,
  })),
);

/** OptionField preserves plain options and opens a compact, lazily loaded formatting editor on demand. */
export function OptionField({
  text,
  content,
  index,
  onChange,
}: Readonly<{
  text: string;
  content?: OptionContent | null;
  index: number;
  onChange: (text: string, content: OptionContent | null) => void;
}>) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const label = t("questionEditor.optionPlaceholder", { n: index + 1 });
  const canFormat =
    content != null || import.meta.env.VITE_RICH_OPTION_EDITOR === "true";
  return (
    <div className="min-w-0 flex-1 space-y-2">
      {editing ? (
        <>
          <Suspense
            fallback={
              <Skeleton
                className="h-24 w-full"
                aria-label={t("contentEditor.loading")}
              />
            }
          >
            <ContentEditor
              initialContent={content ?? plainOptionContent(text)}
              label={label}
              profile="option"
              onChange={(document) => {
                if (isOptionContent(document))
                  onChange(contentPlainText(document), document);
              }}
            />
          </Suspense>
          <div className="flex flex-wrap justify-end gap-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                onChange(text, null);
                setEditing(false);
              }}
            >
              {t("questionEditor.removeFormatting")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setEditing(false)}
            >
              {t("questionEditor.doneFormatting")}
            </Button>
          </div>
        </>
      ) : (
        <div className="flex items-center gap-2">
          {content == null ? (
            <Input
              value={text}
              placeholder={label}
              aria-label={label}
              onChange={(event) => onChange(event.target.value, null)}
            />
          ) : (
            <Button
              type="button"
              variant="outline"
              className="h-auto min-h-9 min-w-0 flex-1 justify-start text-left font-normal whitespace-normal"
              aria-label={t("questionEditor.editFormattedOption", { n: index + 1 })}
              onClick={() => setEditing(true)}
            >
              <OptionText text={text} content={content} />
            </Button>
          )}
          {canFormat && content == null ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={t("questionEditor.formatOption", { n: index + 1 })}
              title={t("questionEditor.formatOption", { n: index + 1 })}
              onClick={() => {
                onChange(text, plainOptionContent(text));
                setEditing(true);
              }}
            >
              <Type aria-hidden="true" />
            </Button>
          ) : null}
        </div>
      )}
    </div>
  );
}
