import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { QuestionPromptContent } from "@/components/shared/content/questionContent";
import {
  BuilderQuestionEditor,
  type BuilderQuestionActions,
} from "@/features/tests/components/BuilderQuestionEditor";
import {
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import i18n from "@/lib/i18n";

Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
Range.prototype.getBoundingClientRect ??= () => new DOMRect();
document.elementFromPoint ??= () => null;

const GAP_PROMPT: QuestionPromptContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "She ", marks: [] },
        { type: "gap", id: "g1", label: "1" },
      ],
    },
  ],
};

function renderPane(
  overrides: Partial<QuestionValues> = {},
  clearPromptOnFocus = false,
) {
  const actions: BuilderQuestionActions = {
    onDuplicate: vi.fn(),
    onMove: vi.fn(),
    onDelete: vi.fn(),
  };
  function Harness() {
    const [value, setValue] = useState<QuestionValues>({
      ...emptyQuestion(),
      ...overrides,
    });
    return (
      <BuilderQuestionEditor
        value={value}
        asset={null}
        number={3}
        clearPromptOnFocus={clearPromptOnFocus}
        actions={actions}
        onChange={setValue}
        onAssetChange={vi.fn()}
      />
    );
  }
  render(
    <QueryClientProvider client={new QueryClient()}>
      <Harness />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), actions };
}

describe("the builder's editor pane on its own", () => {
  it("keeps the other types out of reach while gaps bind answers", async () => {
    const { user } = renderPane({
      type: "fill_blank",
      prompt: "She ___",
      promptContent: GAP_PROMPT,
      options: [],
      blanks: [
        {
          id: null,
          ordinal: 1,
          gapId: "g1",
          acceptedAnswers: ["runs"],
          caseSensitive: false,
        },
      ],
    });

    await user.click(screen.getByRole("button", { name: "Loại câu hỏi: Điền từ" }));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByText("Loại câu hỏi")).toBeVisible();
    for (const name of ["Một đáp án", "Nhiều đáp án", "Đúng/Sai", "Tự luận"]) {
      const item = within(menu).getByRole("menuitem", { name: new RegExp(name) });
      expect(item).toHaveAttribute("aria-disabled", "true");
      expect(item).toHaveTextContent("Bỏ ô trống trước");
    }
    expect(within(menu).getByRole("menuitem", { name: /Điền từ/ })).toHaveTextContent(
      "Đang dùng",
    );
  });

  it("offers Duplicate, Move to section and Delete question in its menu", async () => {
    const { user, actions } = renderPane();

    for (const [name, action] of [
      ["Nhân bản", actions.onDuplicate],
      ["Chuyển sang phần khác", actions.onMove],
      ["Xoá câu hỏi", actions.onDelete],
    ] as const) {
      await user.click(screen.getByRole("button", { name: "Thao tác với câu hỏi" }));
      await user.click(await screen.findByRole("menuitem", { name }));
      expect(action).toHaveBeenCalledOnce();
    }
  });

  it("opens the starter prompt empty, with the placeholder, in Rich text", async () => {
    const starter = i18n.t("builder.starterPrompt");
    renderPane(
      {
        prompt: starter,
        promptContent: {
          format: "semantic_v1",
          blocks: [
            {
              type: "paragraph",
              content: [{ type: "text", text: starter, marks: [] }],
            },
          ],
        },
      },
      true,
    );

    expect(await screen.findByText("Viết câu hỏi học viên sẽ đọc")).toBeInTheDocument();
    await waitFor(() =>
      expect(document.getElementById("question-prompt")).toHaveTextContent(""),
    );
    expect(
      screen.getAllByRole("button", { name: /Văn bản định dạng/ })[0],
    ).toHaveAttribute("aria-pressed", "true");
  });

  it("puts the explanation's placeholder under More options", async () => {
    const { user } = renderPane();
    await user.click(screen.getByText("Tuỳ chọn khác"));
    expect(await screen.findByText("Giải thích đáp án đúng.")).toBeInTheDocument();
  });
});
