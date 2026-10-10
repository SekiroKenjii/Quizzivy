import { useState } from "react";
import { beforeAll, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { BlanksEditor } from "@/features/question-bank/components/BlanksEditor";
import QuestionEditorPage from "@/features/question-bank/pages/teacher/QuestionEditorPage";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import "@/lib/i18n";

type Blank = QuestionValues["blanks"][number];

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

    await user.click(screen.getByRole("tab", { name: "Điền từ" }));
    await screen.findByRole("textbox", { name: "Nội dung câu hỏi" });
    await user.click(
      within(
        screen.getByRole("group", { name: "Chế độ soạn: Nội dung câu hỏi" }),
      ).getByRole("button", { name: "Markdown" }),
    );
    await user.click(
      await screen.findByRole("button", { name: "Chuyển sang Markdown" }),
    );
    await user.click(screen.getByLabelText("Nội dung câu hỏi"));
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
