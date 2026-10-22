import { afterEach, describe, expect, it, vi } from "vitest";
import { Editor } from "@tiptap/core";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { contentExtensions } from "@/components/shared/content/editor/extensions";
import {
  fromEditorDoc,
  toEditorJSON,
} from "@/components/shared/content/editor/adapter";
import {
  insertProseMedia,
  placeAudioMarker,
  removeAudioMarker,
  ProseMediaContext,
  type ProseMediaHost,
} from "@/components/shared/content/editor/proseMedia";
import { ReactNodeViewRenderer } from "@tiptap/react";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { ProseMediaMenu } from "@/components/shared/content/editor/ProseMediaMenu";
import {
  ProseAudioView,
  ProseImageView,
} from "@/components/shared/content/editor/ProseMediaViews";
import type { ProseDocument } from "@/components/shared/content/model";
import { altDraft } from "@/components/shared/content/questionContent";
import "@/lib/i18n";

const MAP = "018f0000-0000-7000-8000-0000000000c1";

const text = (value: string) => ({
  type: "paragraph" as const,
  content: [{ type: "text" as const, text: value, marks: [] }],
});

let editor: Editor | undefined;
afterEach(() => {
  editor?.destroy();
  editor = undefined;
});

function promptEditor(
  blocks: ProseDocument["blocks"],
  profile: "questionPrompt" | "explanation" = "questionPrompt",
) {
  editor = new Editor({
    element: document.createElement("div"),
    extensions: contentExtensions(undefined, profile),
    content: toEditorJSON({ format: "semantic_v1", blocks }, profile),
  });
  return editor;
}

function blocksOf(
  live: Editor,
  profile: "questionPrompt" | "explanation" = "questionPrompt",
) {
  const parsed = fromEditorDoc(live.state.doc, profile);
  if (!parsed.ok) throw new Error("the editor holds an invalid document");
  return (parsed.value as ProseDocument).blocks;
}

describe("the prompt's audio marker", () => {
  it("is placed at the caret, and moved there when the prompt already has one", () => {
    const live = promptEditor([text("Listen."), text("Answer.")]);
    live.commands.setTextSelection(8);

    expect(placeAudioMarker(live)).toBe(true);
    expect(blocksOf(live).map((block) => block.type)).toEqual([
      "paragraph",
      "audio",
      "paragraph",
    ]);

    live.commands.setTextSelection(live.state.doc.content.size - 1);
    expect(placeAudioMarker(live)).toBe(true);
    expect(blocksOf(live).map((block) => block.type)).toEqual([
      "paragraph",
      "paragraph",
      "audio",
    ]);
    expect(blocksOf(live).filter((block) => block.type === "audio")).toHaveLength(1);
  });

  it("comes out when the question loses its audio", () => {
    const live = promptEditor([text("Listen."), { type: "audio" }]);

    expect(removeAudioMarker(live)).toBe(true);
    expect(blocksOf(live).map((block) => block.type)).toEqual(["paragraph"]);
    expect(removeAudioMarker(live)).toBe(false);
  });
});

describe("media the editor refuses", () => {
  it("takes no ninth media block", () => {
    const blocks = Array.from({ length: 8 }, () => ({
      type: "image" as const,
      assetId: MAP,
      alt: "",
    }));
    const live = promptEditor([text("Look."), ...blocks]);

    expect(
      insertProseMedia(live, { type: "proseImage", attrs: { assetId: MAP, alt: "" } }),
    ).toBe(false);
  });

  it("takes no marker in an explanation", () => {
    const live = promptEditor([text("Why.")], "explanation");

    expect(
      insertProseMedia(live, { type: "proseAudio", attrs: { assetId: null } }),
    ).toBe(false);
  });

  it("keeps an image inside the document's own blocks, never inside a list", () => {
    const live = promptEditor([
      { type: "list", ordered: false, start: 1, items: [[text("One")]] },
    ]);
    live.commands.setTextSelection(4);

    expect(
      insertProseMedia(live, { type: "proseImage", attrs: { assetId: MAP, alt: "" } }),
    ).toBe(true);
    expect(blocksOf(live).map((block) => block.type)).toEqual(["list", "image"]);
  });
});

it("keeps white space alone as an empty alt, never a whitespace-only one", () => {
  expect(altDraft("   ")).toBe("");
  expect(altDraft(" A map ")).toBe(" A map ");
});

function host(overrides: Partial<ProseMediaHost> = {}): ProseMediaHost {
  return {
    asset: () => undefined,
    questionAudio: null,
    audioBlocked: null,
    choose: vi.fn(),
    files: vi.fn(),
    register: vi.fn(),
    preview: () => null,
    status: () => null,
    nodeViews: {
      proseImage: ReactNodeViewRenderer(ProseImageView),
      proseAudio: ReactNodeViewRenderer(ProseAudioView),
    },
    Menu: ProseMediaMenu,
    ...overrides,
  };
}

describe("the toolbar's Image and Audio", () => {
  it("offer upload and Media, and say what a prompt's audio is", async () => {
    const choose = vi.fn();
    render(
      <ProseMediaContext value={host({ choose })}>
        <ContentEditor<ProseDocument>
          label="Đề bài"
          profile="questionPrompt"
          initialContent={{ format: "semantic_v1", blocks: [text("Nghe.")] }}
          onChange={() => undefined}
        />
      </ProseMediaContext>,
    );
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Âm thanh" }));
    expect(await screen.findByText(/Đây là âm thanh của câu hỏi/)).toBeInTheDocument();
    await user.click(screen.getByRole("menuitem", { name: /Chọn từ Media/ }));

    await waitFor(() =>
      expect(choose).toHaveBeenCalledWith(
        expect.objectContaining({ kind: "audio", source: "library", field: "prompt" }),
      ),
    );
  });

  it("refuse a prompt's audio while the question shows an image", async () => {
    render(
      <ProseMediaContext value={host({ audioBlocked: "Gỡ hình ảnh trước." })}>
        <ContentEditor<ProseDocument>
          label="Đề bài"
          profile="questionPrompt"
          initialContent={{ format: "semantic_v1", blocks: [text("Nghe.")] }}
          onChange={() => undefined}
        />
      </ProseMediaContext>,
    );
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Âm thanh" }));
    expect(await screen.findByText("Gỡ hình ảnh trước.")).toBeInTheDocument();
    const upload = screen.getByRole("menuitem", { name: /Tải lên từ thiết bị/ });
    expect(upload).toHaveAttribute("aria-disabled", "true");
    expect(upload.className.split(/\s+/)).toEqual(
      expect.arrayContaining([
        "data-[disabled]:opacity-100",
        "data-[disabled]:text-foreground/50",
      ]),
    );
    expect(upload.className).not.toContain("opacity-45");
  });

  it("are not offered where no page hosts media", async () => {
    render(
      <ContentEditor<ProseDocument>
        label="Đề bài"
        profile="questionPrompt"
        initialContent={{ format: "semantic_v1", blocks: [text("Nghe.")] }}
        onChange={() => undefined}
      />,
    );

    await screen.findByRole("textbox", { name: "Đề bài" });
    expect(screen.queryByRole("button", { name: "Hình ảnh" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Âm thanh" })).toBeNull();
  });
});
