import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Bold, Italic, Link2, List } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

interface PromptFieldProps {
  value: string;
  clearOnFocus?: boolean;
  onChange: (value: string) => void;
  id: string;
}

/**
 * The deck's A-04 prompt field: a bordered box with a small toolbar and a
 * "Markdown" hint, over a plain textarea.
 */
export function PromptField({
  value,
  onChange,
  id,
  clearOnFocus = false,
}: Readonly<PromptFieldProps>) {
  const { t } = useTranslation();
  const untouched = useRef(clearOnFocus && value === t("builder.starterPrompt"));
  const [cleared, setCleared] = useState(false);
  const displayed = cleared && value === t("builder.starterPrompt") ? "" : value;

  function wrap(marker: string) {
    const field = document.getElementById(id);
    if (!(field instanceof HTMLTextAreaElement)) return;
    const { selectionStart: start, selectionEnd: end } = field;
    onChange(
      displayed.slice(0, start) +
        marker +
        displayed.slice(start, end) +
        marker +
        displayed.slice(end),
    );
  }

  function insertLink() {
    const field = document.getElementById(id);
    if (!(field instanceof HTMLTextAreaElement)) return;
    const { selectionStart: start, selectionEnd: end } = field;
    const text = displayed.slice(start, end);
    onChange(`${displayed.slice(0, start)}[${text}](url)${displayed.slice(end)}`);
  }

  function prefixLine(marker: string) {
    const field = document.getElementById(id);
    if (!(field instanceof HTMLTextAreaElement)) return;
    const lineStart = displayed.lastIndexOf("\n", field.selectionStart - 1) + 1;
    onChange(displayed.slice(0, lineStart) + marker + displayed.slice(lineStart));
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
        value={displayed}
        onFocus={() => {
          if (untouched.current) {
            untouched.current = false;
            setCleared(true);
          }
        }}
        onChange={(event) => {
          untouched.current = false;
          setCleared(false);
          onChange(event.target.value);
        }}
        className="min-h-18 rounded-none border-0 shadow-none"
      />
    </div>
  );
}
