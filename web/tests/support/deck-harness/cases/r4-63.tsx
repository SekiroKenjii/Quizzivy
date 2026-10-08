import { useEffect, useRef, type ReactElement } from "react";
import type { Editor } from "@tiptap/react";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import type { SemanticContent } from "@/components/shared/content/model";
import { plainOptionContent } from "@/components/shared/content/optionContent";

function prose(text: string): SemanticContent {
  return {
    format: "semantic_v1",
    blocks: [
      { type: "paragraph", content: text ? [{ type: "text", text, marks: [] }] : [] },
    ],
  };
}

const PROMPT = prose("What the writer says about parks in paragraph B?");
const EMPTY = prose("");
const MATERIAL: SemanticContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "heading",
      level: 2,
      content: [{ type: "text", text: "Urban green space", marks: [] }],
    },
    {
      type: "paragraph",
      content: [
        { type: "text", text: "Cities that plan parks early see ", marks: [] },
        { type: "gap", id: "gap-1", label: "1" },
        { type: "text", text: " in health and in ", marks: [] },
        {
          type: "link",
          href: "https://example.com/green",
          content: [{ type: "text", text: "property values", marks: [] }],
        },
        { type: "text", text: ".", marks: [] },
      ],
    },
  ],
};
const TABLE: SemanticContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "table",
      rows: [
        [
          { header: true, rowSpan: 1, colSpan: 1, content: [prose("City").blocks[0]!] },
          {
            header: true,
            rowSpan: 1,
            colSpan: 1,
            content: [prose("Parks").blocks[0]!],
          },
        ],
        [
          {
            header: false,
            rowSpan: 1,
            colSpan: 1,
            content: [prose("Copenhagen").blocks[0]!],
          },
          { header: false, rowSpan: 1, colSpan: 1, content: [prose("31%").blocks[0]!] },
        ],
      ],
    },
  ],
};

function useEditorRef() {
  const ref = useRef<Editor | null>(null);
  const capture = (editor: Editor) => {
    ref.current = editor;
    return null;
  };
  return { ref, capture };
}

export const cases: Record<string, () => ReactElement> = {
  prompt: () => (
    <ContentEditor
      initialContent={PROMPT}
      label="Prompt"
      profile="prompt"
      placeholder="Write the question students will read"
      onChange={() => undefined}
    />
  ),

  explanation: () => (
    <ContentEditor
      initialContent={EMPTY}
      label="Explanation"
      profile="question"
      placeholder="Explain the correct answer. Students see this after grading."
      onChange={() => undefined}
    />
  ),

  "review-question": () => (
    <ContentEditor
      initialContent={prose(
        "According to paragraph C, what should cities prioritise when planning new districts?",
      )}
      label="Question"
      profile="question"
      minHeight={64}
      onChange={() => undefined}
    />
  ),

  option: () => (
    <ContentEditor
      initialContent={plainOptionContent(
        "They improve both health and property values.",
      )}
      label="Option A"
      profile="option"
      onChange={() => undefined}
    />
  ),

  material: () => (
    <ContentEditor
      initialContent={MATERIAL}
      label="Material"
      profile="document"
      onChange={() => undefined}
    />
  ),

  "read-only": () => (
    <ContentEditor
      initialContent={MATERIAL}
      label="Material"
      profile="question"
      fontSize={15}
      readOnly
      onChange={() => undefined}
    />
  ),

  table: function CaretInTable() {
    const { ref, capture } = useEditorRef();
    useEffect(() => {
      ref.current?.commands.focus(3);
    }, [ref]);
    return (
      <ContentEditor
        initialContent={TABLE}
        label="Prompt"
        profile="prompt"
        tools={capture}
        onChange={() => undefined}
      />
    );
  },

  link: function LinkOpen() {
    const { ref, capture } = useEditorRef();
    const opened = useRef(false);
    useEffect(() => {
      if (opened.current) return;
      opened.current = true;
      ref.current?.commands.setTextSelection({ from: 1, to: 5 });
      const button = document.querySelector<HTMLButtonElement>(
        'button[data-roving="link"]',
      );
      button?.scrollIntoView({ block: "nearest", inline: "nearest" });
      button?.click();
    }, [ref]);
    return (
      <ContentEditor
        initialContent={PROMPT}
        label="Prompt"
        profile="prompt"
        tools={capture}
        onChange={() => undefined}
      />
    );
  },

  "notice-blocked": function PasteOverLimit() {
    const { ref, capture } = useEditorRef();
    useEffect(() => {
      const data = new DataTransfer();
      data.setData("text/html", `<p>${"a".repeat(262_145)}</p>`);
      ref.current?.view.dom.dispatchEvent(
        new ClipboardEvent("paste", {
          clipboardData: data,
          bubbles: true,
          cancelable: true,
        }),
      );
    }, [ref]);
    return (
      <ContentEditor
        initialContent={PROMPT}
        label="Prompt"
        profile="prompt"
        tools={capture}
        onChange={() => undefined}
      />
    );
  },

  notice: function PastedFile() {
    const { ref, capture } = useEditorRef();
    useEffect(() => {
      const data = new DataTransfer();
      data.items.add(new File(["x"], "photo.png", { type: "image/png" }));
      ref.current?.view.dom.dispatchEvent(
        new ClipboardEvent("paste", {
          clipboardData: data,
          bubbles: true,
          cancelable: true,
        }),
      );
    }, [ref]);
    return (
      <ContentEditor
        initialContent={PROMPT}
        label="Prompt"
        profile="prompt"
        tools={capture}
        onChange={() => undefined}
      />
    );
  },
};
