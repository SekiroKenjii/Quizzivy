import { useTranslation } from "react-i18next";
import { Bold, Italic, Link2, List } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

interface PromptFieldProps {
  value: string;
  onChange: (value: string) => void;
  id: string;
}

/**
 * The deck's A-04 prompt field: a bordered box with a small toolbar and a
 * "Markdown" hint, over a plain textarea.
 */
export function PromptField({ value, onChange, id }: PromptFieldProps) {
  const { t } = useTranslation();

  function wrap(marker: string) {
    const field = document.getElementById(id);
    if (!(field instanceof HTMLTextAreaElement)) return;
    const { selectionStart: start, selectionEnd: end } = field;
    onChange(
      value.slice(0, start) +
        marker +
        value.slice(start, end) +
        marker +
        value.slice(end),
    );
  }

  function insertLink() {
    const field = document.getElementById(id);
    if (!(field instanceof HTMLTextAreaElement)) return;
    const { selectionStart: start, selectionEnd: end } = field;
    const text = value.slice(start, end);
    onChange(`${value.slice(0, start)}[${text}](url)${value.slice(end)}`);
  }

  function prefixLine(marker: string) {
    const field = document.getElementById(id);
    if (!(field instanceof HTMLTextAreaElement)) return;
    const lineStart = value.lastIndexOf("\n", field.selectionStart - 1) + 1;
    onChange(value.slice(0, lineStart) + marker + value.slice(lineStart));
  }

  return (
    <div className="rounded-md border">
      <div className="flex items-center gap-0.5 border-b px-1.5 py-1">
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label={t("questionEditor.bold")}
          onClick={() => wrap("**")}
        >
          <Bold aria-hidden="true" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label={t("questionEditor.italic")}
          onClick={() => wrap("_")}
        >
          <Italic aria-hidden="true" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label={t("questionEditor.list")}
          onClick={() => prefixLine("- ")}
        >
          <List aria-hidden="true" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label={t("questionEditor.link")}
          onClick={insertLink}
        >
          <Link2 aria-hidden="true" />
        </Button>
        <span className="text-muted-foreground ml-auto pr-1 text-xs">
          {t("questionEditor.markdown")}
        </span>
      </div>
      <Textarea
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="min-h-18 rounded-none border-0 shadow-none focus-visible:ring-0"
      />
    </div>
  );
}
