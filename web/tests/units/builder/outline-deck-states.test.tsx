import { useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { Toaster } from "@/components/ui/sonner";
import {
  OutlineTree,
  type OutlineQuestion,
} from "@/features/tests/components/OutlineTree";
import { QuestionPickerDialog } from "@/features/tests/components/QuestionPickerDialog";
import type { OutlineSection } from "@/features/tests/outline";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

const start: OutlineSection[] = [
  {
    id: "s1",
    clientId: "client-1",
    title: "Reading",
    instructions: "Read the passage first",
    questionIds: ["q1", "q2"],
  },
];
const questions = new Map<string, OutlineQuestion>([
  ["q1", { id: "q1", prompt: "Câu một", points: 1, hasAudio: false, problem: null }],
  ["q2", { id: "q2", prompt: "Câu hai", points: 1, hasAudio: false, problem: null }],
]);

function renderOutline() {
  const changed = vi.fn();
  function Harness() {
    const [sections, setSections] = useState(start);
    return (
      <>
        <OutlineTree
          sections={sections}
          questions={questions}
          selectedId="q1"
          creating={false}
          onSelect={vi.fn()}
          onCreateQuestion={vi.fn()}
          onPickFromBank={vi.fn()}
          onAddSection={() =>
            setSections((current) => [
              ...current,
              {
                id: null,
                clientId: "client-new",
                title: "Phần 2",
                instructions: null,
                questionIds: [],
              },
            ])
          }
          onChange={(next) => {
            changed(next);
            setSections(next);
          }}
        />
        <Toaster />
      </>
    );
  }
  return { ...render(<Harness />), user: userEvent.setup(), changed };
}

describe("the outline's deck states", () => {
  it("says the question stays in the bank when it leaves the test", async () => {
    const { user } = renderOutline();

    await user.click(screen.getByRole("button", { name: "Gỡ câu 2 khỏi đề" }));

    expect(
      await screen.findByText("Đã gỡ câu hỏi khỏi đề. Câu vẫn còn trong ngân hàng."),
    ).toBeInTheDocument();
  });

  it("opens a new section with its name ready to type", async () => {
    const { user } = renderOutline();

    await user.click(screen.getByRole("button", { name: "Thêm phần" }));

    const name = screen.getByRole("textbox", { name: "Tên phần" });
    expect(name).toHaveValue("Phần 2");
    expect(name).toHaveFocus();
  });

  it("keeps a renamed section's layout until the pointer that blurred it is released, so that click lands", async () => {
    const { user, changed } = renderOutline();
    await user.click(screen.getByRole("button", { name: "Thêm phần" }));
    const name = screen.getByRole("textbox", { name: "Tên phần" });
    await user.clear(name);
    await user.type(name, "Nghe");
    const bank = screen.getByRole("button", { name: "Từ ngân hàng câu hỏi" });

    await user.pointer({ keys: "[MouseLeft>]", target: bank });
    expect(name).not.toHaveFocus();
    expect(screen.getByRole("textbox", { name: "Tên phần" })).toBeInTheDocument();
    await user.pointer({ keys: "[/MouseLeft]", target: bank });

    expect(screen.queryByRole("textbox", { name: "Tên phần" })).toBeNull();
    expect(changed.mock.lastCall![0].at(-1).title).toBe("Nghe");
  });

  it("holds a rename blurred by a touch until the click that follows it", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const picked = vi.fn();
      function Harness() {
        const [sections, setSections] = useState(start);
        return (
          <OutlineTree
            sections={sections}
            questions={questions}
            selectedId="q1"
            creating={false}
            onSelect={vi.fn()}
            onCreateQuestion={vi.fn()}
            onPickFromBank={picked}
            onAddSection={() =>
              setSections((current) => [
                ...current,
                {
                  id: null,
                  clientId: "client-new",
                  title: "Phần 2",
                  instructions: null,
                  questionIds: [],
                },
              ])
            }
            onChange={setSections}
          />
        );
      }
      render(<Harness />);
      fireEvent.click(screen.getByRole("button", { name: "Thêm phần" }));
      const name = screen.getByRole("textbox", { name: "Tên phần" });
      expect(name).toHaveFocus();
      const bank = screen.getByRole("button", { name: "Từ ngân hàng câu hỏi" });

      fireEvent.pointerDown(bank, { pointerType: "touch" });
      fireEvent.pointerUp(bank, { pointerType: "touch" });
      act(() => bank.focus());
      expect(screen.getByRole("textbox", { name: "Tên phần" })).toBeInTheDocument();
      fireEvent.click(bank);

      expect(picked).toHaveBeenCalledOnce();
      expect(screen.queryByRole("textbox", { name: "Tên phần" })).toBeNull();
      await act(() => vi.runOnlyPendingTimersAsync());
      expect(picked).toHaveBeenCalledOnce();
    } finally {
      vi.useRealTimers();
    }
  });

  it("commits a rename at once when the field loses focus to the keyboard", async () => {
    const { user, changed } = renderOutline();
    await user.click(screen.getByRole("button", { name: "Thêm phần" }));
    await user.type(screen.getByRole("textbox", { name: "Tên phần" }), " B");
    await user.tab();
    expect(screen.queryByRole("textbox", { name: "Tên phần" })).toBeNull();
    expect(changed.mock.lastCall![0].at(-1).title).toBe("Phần 2 B");
  });

  it("returns focus to the instructions line when Escape closes its dialog", async () => {
    const { user } = renderOutline();
    const line = screen.getByRole("button", { name: "Read the passage first" });

    await user.click(line);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{Escape}");

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(line).toHaveFocus();
  });

  it("returns focus to the section menu when Escape closes the instructions it opened", async () => {
    const { user } = renderOutline();
    const menu = screen.getByRole("button", { name: "Thao tác với phần" });

    await user.click(menu);
    await user.click(await screen.findByRole("menuitem", { name: "Hướng dẫn" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{Escape}");

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(menu).toHaveFocus());
  });
});

describe("the bank picker", () => {
  it("returns focus to its opener on Escape, even when the click did not focus it", async () => {
    server.use(
      http.get(`${BASE}/teacher/questions`, () =>
        contractJson("/teacher/questions", "get", 200, {
          facets: {
            levels: { pre_a1: 0, a1: 0, a2: 0, b1: 0, b2: 0, c1: 0, c2: 0 },
            skills: {
              grammar: 0,
              vocabulary: 0,
              reading: 0,
              listening: 0,
              writing: 0,
              speaking: 0,
            },
            all: 0,
            single_choice: 0,
            multiple_choice: 0,
            true_false: 0,
            fill_blank: 0,
            short_answer: 0,
          },
          tags: [],
          bankTotal: 0,
          items: [],
          page: 1,
          pageSize: 50,
          total: 0,
        }),
      ),
    );
    function Harness() {
      const [open, setOpen] = useState(false);
      const opener = useRef<HTMLElement | null>(null);
      return (
        <QueryClientProvider client={new QueryClient()}>
          <button
            type="button"
            onClick={(event) => {
              opener.current = event.currentTarget;
              setOpen(true);
            }}
          >
            Từ ngân hàng câu hỏi
          </button>
          <QuestionPickerDialog
            open={open}
            excluded={new Set()}
            returnFocus={opener}
            onOpenChange={setOpen}
            onPick={vi.fn()}
          />
        </QueryClientProvider>
      );
    }
    render(<Harness />);
    const user = userEvent.setup();
    const button = screen.getByRole("button", { name: "Từ ngân hàng câu hỏi" });

    fireEvent.click(button);
    within(await screen.findByRole("dialog")).getByRole("textbox");
    await user.keyboard("{Escape}");

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(button).toHaveFocus();
  });
});
