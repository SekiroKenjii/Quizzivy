import { useState } from "react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { BlanksEditor } from "@/features/question-bank/components/BlanksEditor";
import { GapAnswers } from "@/features/question-bank/components/GapAnswers";
import type { QuestionPromptContent } from "@/components/shared/content/questionContent";
import QuestionEditorPage from "@/features/question-bank/pages/teacher/QuestionEditorPage";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import "@/lib/i18n";

type Blank = QuestionValues["blanks"][number];

Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
Range.prototype.getBoundingClientRect ??= () => new DOMRect();
document.elementFromPoint ??= () => null;

function blank(ordinal: number): Blank {
  return { id: null, ordinal, acceptedAnswers: ["x"], caseSensitive: false };
}

function renderBlanks(prompt: string, blanks: Blank[]) {
  function Harness() {
    const [value, setValue] = useState(blanks);
    return <BlanksEditor prompt={prompt} blanks={value} onChange={setValue} />;
  }
  render(<Harness />);
  return userEvent.setup();
}

/**
 * The placeholder rule, checked where it is authored.
 *
 * The server enforces it at save and again at publish, so nothing invalid can
 * be stored either way. This exists so the teacher is not told at publish time
 * that the `{{3}}` they typed twenty questions ago has no blank behind it.
 */
describe("the fill_blank editor's placeholder check", () => {
  it("flags a {{3}} that has no blank behind it, before anything is submitted", () => {
    renderBlanks("She {{1}} and they {{3}} here.", [blank(1), blank(2)]);

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("{{3}}");
    expect(alert).toHaveTextContent(/Chỗ trống 2 không có ký hiệu/);
  });

  it("says nothing when the prompt and the blanks agree", () => {
    renderBlanks("She {{1}} and they {{2}} here.", [blank(1), blank(2)]);

    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows the markers to type, not the translation's quoting", () => {
    renderBlanks("She {{1}} here.", [blank(1)]);

    expect(
      screen.getByText("Đánh dấu chỗ trống trong đề bài bằng {{1}}, {{2}}"),
    ).toBeInTheDocument();
  });

  it("renumbers the remaining blanks so none is left unaddressable", async () => {
    const user = renderBlanks("She {{1}} here.", [blank(1), blank(2)]);

    await user.click(screen.getByRole("button", { name: "Xoá chỗ trống 1" }));

    expect(screen.getByText("Chỗ trống 1")).toBeInTheDocument();
    expect(screen.queryByText("Chỗ trống 2")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("the question editor page, on a fill_blank mismatch", () => {
  beforeAll(() => import("@/features/question-bank/components/RichBlankEditor"));

  it("refuses to save while a {{3}} has no blank", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter(
      [{ path: "/teacher/question-bank/new", element: <QuestionEditorPage /> }],
      { initialEntries: ["/teacher/question-bank/new"] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Điền từ" }));
    await screen.findByRole("textbox", { name: "Câu hỏi" });
    await user.click(
      within(screen.getByRole("group", { name: "Chế độ soạn: Câu hỏi" })).getByRole(
        "button",
        { name: "Markdown" },
      ),
    );
    await user.click(
      await screen.findByRole("button", { name: "Chuyển sang Markdown" }),
    );
    await user.click(screen.getByLabelText("Câu hỏi"));
    await user.paste("She {{1}} and {{3}}.");
    await user.click(screen.getByRole("button", { name: "Thêm chỗ trống" }));
    await user.type(
      screen.getByLabelText("Đáp án được chấp nhận cho chỗ trống 1"),
      "lives",
    );

    expect(screen.getByRole("button", { name: "Lưu" })).toBeDisabled();
    expect(screen.getByText("Ký hiệu chỗ trống chưa khớp.")).toBeInTheDocument();
  });
});

const TWO_GAPS: QuestionPromptContent = {
  format: "semantic_v1",
  blocks: [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "They ", marks: [] },
        { type: "gap", id: "b", label: "2" },
        { type: "text", text: ", she ", marks: [] },
        { type: "gap", id: "a", label: "1" },
      ],
    },
  ],
};

function gapBlank(gapId: string, ordinal: number, acceptedAnswers: string[]): Blank {
  return { id: null, gapId, ordinal, acceptedAnswers, caseSensitive: false };
}

function renderGaps(
  content: QuestionPromptContent | null,
  blanks: Blank[],
  onInsertGap?: () => void,
) {
  const seen: Blank[][] = [];
  function Harness() {
    const [value, setValue] = useState(blanks);
    return (
      <>
        <GapAnswers
          content={content}
          blanks={value}
          onChange={(next) => {
            seen.push(next);
            setValue(next);
          }}
          onInsertGap={onInsertGap}
        />
        <button type="button">Tiếp</button>
      </>
    );
  }
  render(<Harness />);
  return { user: userEvent.setup(), seen };
}

function answersOf(group: HTMLElement) {
  return [...group.querySelectorAll('[data-slot="chip"]')].map(
    (chip) => chip.textContent,
  );
}

describe("the rich prompt's gap answers", () => {
  it("draws one row per gap in the order the gaps stand in the prompt", () => {
    renderGaps(TWO_GAPS, [gapBlank("a", 1, ["lives"]), gapBlank("b", 2, ["go"])]);

    const groups = screen.getAllByRole("group");
    expect(groups.map((group) => group.getAttribute("aria-label"))).toEqual([
      "Ô trống 2",
      "Ô trống 1",
    ]);
    expect(answersOf(groups[0]!)).toEqual(["go"]);
    expect(answersOf(groups[1]!)).toEqual(["lives"]);
  });

  it("adds an answer on Enter, on a comma and on leaving the input", async () => {
    const { user, seen } = renderGaps(TWO_GAPS, [
      gapBlank("a", 1, ["lives"]),
      gapBlank("b", 2, []),
    ]);
    const input = screen.getByRole("textbox", {
      name: "Đáp án được chấp nhận cho ô 2",
    });
    expect(input).toHaveAttribute("placeholder", "Nhập đáp án rồi nhấn Enter");

    await user.type(input, "go{Enter}");
    await user.type(input, "went,");
    await user.type(input, "goes");
    await user.click(screen.getByRole("button", { name: "Tiếp" }));

    expect(answersOf(screen.getByRole("group", { name: "Ô trống 2" }))).toEqual([
      "go",
      "went",
      "goes",
    ]);
    expect(seen.at(-1)!.find((blank) => blank.gapId === "b")!.acceptedAnswers).toEqual([
      "go",
      "went",
      "goes",
    ]);
    expect(input).toHaveAttribute("placeholder", "Thêm đáp án khác");
  });

  it("refuses an answer already there whatever its case, until Match case is on", async () => {
    const { user } = renderGaps(TWO_GAPS, [
      gapBlank("a", 1, ["London"]),
      gapBlank("b", 2, ["go"]),
    ]);
    const group = screen.getByRole("group", { name: "Ô trống 1" });
    const input = screen.getByRole("textbox", {
      name: "Đáp án được chấp nhận cho ô 1",
    });

    await user.type(input, "london{Enter}");
    expect(answersOf(group)).toEqual(["London"]);

    await user.click(
      screen.getByRole("switch", { name: "Phân biệt hoa thường cho ô 1" }),
    );
    await user.type(input, "london{Enter}");
    await user.type(input, "London{Enter}");
    expect(answersOf(group)).toEqual(["London", "london"]);
  });

  it("says capital letters are ignored only while no gap matches case", async () => {
    const { user } = renderGaps(TWO_GAPS, [
      gapBlank("a", 1, ["lives"]),
      gapBlank("b", 2, ["go"]),
    ]);
    const sentence = /Chữ hoa và khoảng trắng thừa không được tính\./;
    expect(screen.getByText(sentence)).toBeInTheDocument();

    await user.click(
      screen.getByRole("switch", { name: "Phân biệt hoa thường cho ô 2" }),
    );
    expect(screen.queryByText(sentence)).toBeNull();
  });

  it("marks a gap with no answer and reads the message with its input", () => {
    renderGaps(TWO_GAPS, [gapBlank("a", 1, ["lives"]), gapBlank("b", 2, [])]);

    const input = screen.getByRole("textbox", {
      name: "Đáp án được chấp nhận cho ô 2",
    });
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription(
      "Hãy thêm ít nhất một đáp án được chấp nhận cho ô 2.",
    );
    expect(
      screen.getByRole("textbox", { name: "Đáp án được chấp nhận cho ô 1" }),
    ).not.toHaveAttribute("aria-invalid");
  });

  it("offers Insert gap on the empty card while the prompt has no gap", async () => {
    const onInsertGap = vi.fn();
    const { user } = renderGaps(null, [], onInsertGap);

    expect(screen.getByText("Đề bài chưa có ô trống nào")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Thêm ô trống" }));
    expect(onInsertGap).toHaveBeenCalledTimes(1);
  });
});

describe("the question editor page, on a new fill_blank question", () => {
  beforeAll(() => import("@/features/question-bank/components/RichBlankEditor"));

  it("inserts a gap at the prompt's caret from the empty card and gives it a row", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter(
      [{ path: "/teacher/question-bank/new", element: <QuestionEditorPage /> }],
      { initialEntries: ["/teacher/question-bank/new"] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Điền từ" }));
    const prompt = await screen.findByRole("textbox", { name: "Câu hỏi" });
    await user.click(prompt);
    await user.keyboard("She ");
    const title = await screen.findByText("Đề bài chưa có ô trống nào");
    const card = title.parentElement!.parentElement!;
    await user.click(within(card).getByRole("button", { name: "Thêm ô trống" }));

    await waitFor(() =>
      expect(prompt.querySelectorAll(".content-gap")).toHaveLength(1),
    );
    expect(prompt).toHaveTextContent("She 1");
    expect(screen.queryByText("Đề bài chưa có ô trống nào")).toBeNull();
    expect(
      screen.getByRole("textbox", { name: "Đáp án được chấp nhận cho ô 1" }),
    ).toHaveAccessibleDescription(
      "Hãy thêm ít nhất một đáp án được chấp nhận cho ô 1.",
    );
  });
});
