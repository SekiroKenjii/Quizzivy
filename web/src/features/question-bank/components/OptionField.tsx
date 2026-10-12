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
import { optionLetter } from "../questionType";

const ContentEditor = lazy(() =>
  import("@/components/shared/content/editor/ContentEditor").then((module) => ({
    default: module.ContentEditor,
  })),
);

/**
 * OptionField preserves plain options and opens a compact, lazily loaded
 * formatting editor on demand. Its controls are named by the option's letter,
 * and `bare` draws the plain input without its own frame, for a row that draws
 * one.
 */
export function OptionField({
  text,
  content,
  index,
  bare = false,
  onChange,
}: Readonly<{
  text: string;
  content?: OptionContent | null;
  index: number;
  bare?: boolean;
  onChange: (text: string, content: OptionContent | null) => void;
}>) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const letter = optionLetter(index);
  const label = t("questionEditor.optionLetter", { letter });
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
              className={
                bare
                  ? "h-8 rounded-[6px] border-transparent bg-transparent px-px shadow-none in-data-[scale=deck]:h-8 in-data-[scale=deck]:bg-transparent in-data-[scale=deck]:px-px lg:text-[14px] in-data-[scale=deck]:lg:text-[14px] dark:bg-transparent dark:in-data-[scale=deck]:bg-transparent"
                  : undefined
              }
              onChange={(event) => onChange(event.target.value, null)}
            />
          ) : (
            <Button
              type="button"
              variant="outline"
              className="h-auto min-h-9 min-w-0 flex-1 justify-start text-left font-normal whitespace-normal"
              aria-label={t("questionEditor.editFormattedOption", { letter })}
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
              aria-label={t("questionEditor.formatOption", { letter })}
              title={t("questionEditor.formatOption", { letter })}
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
