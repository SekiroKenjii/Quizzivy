import { useEffect, useState } from "react";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import type { ProseDocument } from "@/components/shared/content/model";
import { contentPlainText } from "@/components/shared/content/plainText";
import type {
  ProseField,
  StoredQuestionProse,
} from "@/components/shared/content/questionContent";
import { SwitchToMarkdown } from "./ProseMode";

const EMPTY: ProseDocument = {
  format: "semantic_v1",
  blocks: [{ type: "paragraph", content: [] }],
};

/**
 * RichProseEditor is a prompt's or an explanation's rich body, always live.
 * It starts from the stored content, or from an empty paragraph when the
 * field has none, and reports each valid document with its plain text.
 * `field` names its rules: a prompt without gaps or an explanation, each with
 * its media (DG-109). While `leaving` it asks "Switch to Markdown" under its
 * toolbar. A document whose text is exactly `starter` opens empty, showing
 * `placeholder`, and the starter stays stored until the first edit.
 */
export function RichProseEditor({
  content,
  field,
  id,
  label,
  describedBy,
  minHeight,
  fontSize,
  leaving,
  focusOnMount = false,
  placeholder,
  starter,
  onCancelLeave,
  onConfirmLeave,
  onChange,
}: Readonly<{
  content: StoredQuestionProse | null;
  field: ProseField;
  id: string;
  label: string;
  describedBy?: string | undefined;
  minHeight: number;
  fontSize: number;
  leaving: boolean;
  focusOnMount?: boolean;
  placeholder?: string | undefined;
  starter?: string | undefined;
  onCancelLeave: () => void;
  onConfirmLeave: () => void;
  onChange: (text: string, content: StoredQuestionProse) => void;
}>) {
  const [initial] = useState<ProseDocument>(() =>
    content === null || (starter !== undefined && contentPlainText(content) === starter)
      ? EMPTY
      : content,
  );
  useEffect(() => {
    if (focusOnMount) document.getElementById(id)?.focus();
  }, [focusOnMount, id]);
  return (
    <ContentEditor<ProseDocument>
      initialContent={initial}
      id={id}
      label={label}
      describedBy={describedBy}
      profile={field === "prompt" ? "questionPrompt" : "explanation"}
      minHeight={minHeight}
      fontSize={fontSize}
      placeholder={placeholder}
      tools={() =>
        leaving && (
          <SwitchToMarkdown onCancel={onCancelLeave} onConfirm={onConfirmLeave} />
        )
      }
      onChange={(document) =>
        onChange(contentPlainText(document), document as StoredQuestionProse)
      }
    />
  );
}
