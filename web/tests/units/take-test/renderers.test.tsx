import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QuestionBody } from "@/features/take-test/components/QuestionBody";
import { QuestionCard } from "@/features/take-test/components/QuestionCard";
import { QuestionSheet } from "@/features/take-test/components/QuestionSheet";
import { UnknownType } from "@/features/take-test/components/UnknownType";
import { questionKind, questionLine } from "@/features/take-test/questionType";
import { useTakeTestStore } from "@/features/take-test/store";
import type { Answer, StudentQuestion } from "@/features/take-test/api";
import { writeLargerTestText } from "@/lib/testText";
import i18n from "@/lib/i18n";
import { PaperSourceContext, type PaperSource } from "@/features/take-test/paper";
import { renderWithPaper, viewport } from "./support";

beforeEach(() => {
  localStorage.clear();
  useTakeTestStore.getState().reset();
});
afterEach(() => {
  act(() => writeLargerTestText(false));
  localStorage.clear();
  useTakeTestStore.getState().reset();
});

function question(
  over: Partial<StudentQuestion> & { type: StudentQuestion["type"] },
): StudentQuestion {
  return { id: "q1", sectionId: "s1", prompt: "Prompt", points: 1, ...over };
}

function renderCard(q: StudentQuestion) {
  return renderWithPaper(
    <QuestionCard question={q} number={1} total={1} onAudioExpired={vi.fn()} />,
  );
}

function renderQuestion(q: StudentQuestion, answer?: Answer) {
  const onAnswer = vi.fn();
  render(<QuestionBody question={q} answer={answer} onAnswer={onAnswer} />);
  return onAnswer;
}

const options = [
  { id: "o1", text: "has been living" },
  { id: "o2", text: "have lived" },
  { id: "o3", text: "is living" },
];

describe("single_choice", () => {
  it("offers one radio per option, keyed A B C", () => {
    renderQuestion(question({ type: "single_choice", options }));
    expect(screen.getAllByRole("radio")).toHaveLength(3);
    expect(screen.getByRole("radiogroup")).toHaveAccessibleDescription(
      "Chọn một đáp án.",
    );
    for (const key of ["A", "B", "C"]) {
      expect(screen.getByText(key)).toBeInTheDocument();
    }
  });

  it("reports the option the student picked", async () => {
    const onAnswer = renderQuestion(question({ type: "single_choice", options }));
    await userEvent.click(screen.getByRole("radio", { name: /have lived/ }));
    expect(onAnswer).toHaveBeenCalledWith({ type: "choice", optionIds: ["o2"] });
  });

  it("replaces the previous choice rather than adding to it", async () => {
    const onAnswer = renderQuestion(question({ type: "single_choice", options }), {
      type: "choice",
      optionIds: ["o1"],
    });
    await userEvent.click(screen.getByRole("radio", { name: /is living/ }));
    expect(onAnswer).toHaveBeenCalledWith({ type: "choice", optionIds: ["o3"] });
  });

  it("shows the stored answer as chosen", () => {
    renderQuestion(question({ type: "single_choice", options }), {
      type: "choice",
      optionIds: ["o2"],
    });
    expect(screen.getByRole("radio", { name: /have lived/ })).toBeChecked();
  });

  it("keeps the options in the order the server sent them", () => {
    renderQuestion(
      question({
        type: "single_choice",
        options: [options[2]!, options[0]!, options[1]!],
      }),
    );
    expect(
      screen.getAllByRole("radio").map((radio) => radio.parentElement?.textContent),
    ).toEqual(["Ais living", "Bhas been living", "Chave lived"]);
  });

  it("draws each option as a row at least 52px high with a round letter marker", () => {
    renderQuestion(question({ type: "single_choice", options }));
    const row = screen.getByRole("radio", { name: "have lived" }).parentElement!;
    expect(row).toHaveClass(
      "min-h-13",
      "rounded-[11px]",
      "border-[1.5px]",
      "border-border",
      "bg-card",
      "px-3.5",
      "py-2.5",
      "text-md",
      "leading-[1.4]",
      "hover:border-ring",
    );
    const marker = within(row).getByText("B");
    expect(marker).toHaveClass(
      "size-6.5",
      "rounded-full",
      "border-ring",
      "text-muted-fg",
    );
    expect(marker).toHaveAttribute("aria-hidden", "true");
    expect(marker.querySelector("svg")).toBeNull();
  });

  it("gives the chosen option a primary border and keeps its letter, light on the primary marker", () => {
    renderQuestion(question({ type: "single_choice", options }), {
      type: "choice",
      optionIds: ["o2"],
    });
    const row = screen.getByRole("radio", { name: "have lived" }).parentElement!;
    expect(row).toHaveClass("border-primary");
    expect(row).not.toHaveClass("border-border");
    expect(row).not.toHaveClass("hover:border-ring");
    const marker = row.querySelector<HTMLElement>("span[aria-hidden]")!;
    expect(marker).toHaveClass("border-primary", "bg-primary", "text-primary-fg");
    expect(marker).toHaveTextContent(/^B$/);
    expect(marker.querySelector("svg")).toBeNull();
    const other = screen.getByRole("radio", { name: "is living" }).parentElement!;
    expect(other).toHaveClass("border-border");
    expect(within(other).getByText("C")).toBeInTheDocument();
  });

  it("says how to answer to a screen reader only, as the line above the question says it", () => {
    renderQuestion(question({ type: "single_choice", options }));
    expect(screen.getByText("Chọn một đáp án.")).toHaveClass("sr-only");
  });

  it("draws the prompt at 17px, medium, on a 1.6 line", () => {
    renderQuestion(question({ type: "single_choice", options }));
    expect(screen.getByText("Prompt").parentElement).toHaveClass(
      "text-lg",
      "leading-[1.6]!",
      "font-medium",
    );
  });
});

describe("multiple_choice", () => {
  it("uses checkboxes, because more than one may be right", () => {
    renderQuestion(question({ type: "multiple_choice", options }));
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
    expect(screen.getByRole("group")).toHaveAccessibleDescription(
      "Chọn các đáp án bạn cho là đúng. Bạn có thể chọn nhiều đáp án.",
    );
  });

  it("adds to the selection rather than replacing it", async () => {
    const onAnswer = renderQuestion(question({ type: "multiple_choice", options }), {
      type: "choice",
      optionIds: ["o1"],
    });
    await userEvent.click(screen.getByRole("checkbox", { name: /is living/ }));
    expect(onAnswer).toHaveBeenCalledWith({ type: "choice", optionIds: ["o1", "o3"] });
  });

  it("removes one that was already chosen", async () => {
    const onAnswer = renderQuestion(question({ type: "multiple_choice", options }), {
      type: "choice",
      optionIds: ["o1", "o3"],
    });
    await userEvent.click(screen.getByRole("checkbox", { name: /has been living/ }));
    expect(onAnswer).toHaveBeenCalledWith({ type: "choice", optionIds: ["o3"] });
  });

  it("draws square markers, and shows the instruction as the hint under the prompt", () => {
    renderQuestion(question({ type: "multiple_choice", options }), {
      type: "choice",
      optionIds: ["o1"],
    });
    const chosen = screen.getByRole("checkbox", {
      name: "has been living",
    }).parentElement!;
    expect(chosen.querySelector("span[aria-hidden]")).toHaveClass(
      "rounded-[7px]",
      "bg-primary",
    );
    expect(chosen.querySelector("span[aria-hidden]")).toHaveTextContent(/^A$/);
    expect(within(chosen.parentElement!).getByText("B")).toHaveClass("rounded-[7px]");
    expect(within(chosen.parentElement!).getByText("B")).not.toHaveClass(
      "rounded-full",
    );
    const hint = screen.getByText(
      "Chọn các đáp án bạn cho là đúng. Bạn có thể chọn nhiều đáp án.",
    );
    expect(hint).toHaveClass("text-muted-fg", "text-sm", "-mt-2");
    expect(hint).not.toHaveClass("sr-only");
  });
});

describe("the options' columns", () => {
  const short = ["went", "goes", "gone", "going"].map((text, index) => ({
    id: `s${index + 1}`,
    text,
  }));
  const rows = (choices: NonNullable<StudentQuestion["options"]>) => {
    renderQuestion(question({ type: "single_choice", options: choices }));
    return screen.getByRole("radiogroup");
  };
  afterEach(() => vi.unstubAllGlobals());

  it("lays four short options in two columns below 768", () => {
    viewport(359);
    expect(rows(short)).toHaveClass("grid", "grid-cols-2", "gap-2");
  });

  it("lays them in up to four columns from 768", () => {
    viewport(768);
    expect(rows(short)).toHaveClass("grid-cols-4");
    cleanup();
    expect(rows(short.slice(0, 3))).toHaveClass("grid-cols-3");
    cleanup();
    expect(rows([...short, { id: "s5", text: "to go" }])).toHaveClass("grid-cols-4");
  });

  it("keeps one column when one option is longer than 16 characters", () => {
    viewport(1280);
    expect(
      rows([...short.slice(0, 3), { id: "long", text: "has been going on" }]),
    ).toHaveClass("grid-cols-1");
    cleanup();
    expect(
      rows([...short.slice(0, 3), { id: "edge", text: "sixteen letters!" }]),
    ).toHaveClass("grid-cols-4");
  });

  it("keeps one column when an option has rich content", () => {
    viewport(1280);
    expect(
      rows([
        ...short.slice(0, 3),
        {
          id: "rich",
          text: "gone",
          content: {
            format: "semantic_v1",
            blocks: [
              {
                type: "paragraph",
                content: [{ type: "text", text: "gone", marks: ["bold"] }],
              },
            ],
          },
        },
      ]),
    ).toHaveClass("grid-cols-1");
  });

  it("keeps one column beside shared content", () => {
    viewport(1280);
    const source = {
      groups: [
        {
          id: "g1",
          sectionId: "s1",
          title: "Reading",
          questionIds: ["q1"],
          stimuli: [
            {
              id: "m1",
              title: "Reading",
              content: { format: "semantic_v1", blocks: [] },
              gaps: [],
            },
          ],
          recordings: [],
          assets: [],
        },
      ],
    } as unknown as PaperSource;
    render(
      <PaperSourceContext value={source}>
        <QuestionBody
          question={question({ type: "single_choice", options: short })}
          answer={undefined}
          onAnswer={vi.fn()}
        />
      </PaperSourceContext>,
    );
    expect(screen.getByRole("radiogroup")).toHaveClass("grid-cols-1");
  });
});

describe("true_false", () => {
  it("is a two-option radio group, not a special control", () => {
    renderQuestion(
      question({
        type: "true_false",
        options: [
          { id: "t", text: "True" },
          { id: "f", text: "False" },
        ],
      }),
    );
    expect(screen.getAllByRole("radio")).toHaveLength(2);
  });

  it("writes the choice answer it has always written", async () => {
    const onAnswer = renderQuestion(
      question({
        type: "true_false",
        options: [
          { id: "t", text: "True" },
          { id: "f", text: "False" },
        ],
      }),
    );
    await userEvent.click(screen.getByRole("radio", { name: "Sai" }));
    expect(onAnswer).toHaveBeenCalledWith({ type: "choice", optionIds: ["f"] });
    expect(onAnswer).toHaveBeenCalledOnce();
  });
});

describe("fill_blank", () => {
  it("keeps focus and the full answer while the controlled value changes", async () => {
    const q = question({
      type: "fill_blank",
      prompt: "If it {{1}} tomorrow, we {{2}} home.",
      blanks: [
        { id: "b1", ordinal: 1, caseSensitive: false },
        { id: "b2", ordinal: 2, caseSensitive: false },
      ],
    });
    function Paper() {
      const [answer, setAnswer] = useState<Answer>();
      return <QuestionBody question={q} answer={answer} onAnswer={setAnswer} />;
    }
    render(<Paper />);
    const user = userEvent.setup();
    const first = screen.getByRole("textbox", { name: "Chỗ trống 1" });
    await user.type(first, "rains");
    expect(first).toHaveFocus();
    expect(first).toHaveValue("rains");
    await user.tab();
    await user.keyboard("stay");
    expect(screen.getByRole("textbox", { name: "Chỗ trống 2" })).toHaveValue("stay");
    expect(first).toHaveValue("rains");
  });
  // T-3.11's named case.
  it("puts three labelled inputs in prompt order", () => {
    renderQuestion(
      question({
        type: "fill_blank",
        prompt: "If it {{1}} tomorrow, we {{2}} the trip, and {{3}} home.",
        blanks: [
          { id: "b1", ordinal: 1, caseSensitive: false },
          { id: "b2", ordinal: 2, caseSensitive: false },
          { id: "b3", ordinal: 3, caseSensitive: false },
        ],
      }),
    );

    const inputs = screen.getAllByRole("textbox");
    expect(inputs).toHaveLength(3);
    expect(inputs[0]).toHaveAccessibleName("Chỗ trống 1");
    expect(inputs[1]).toHaveAccessibleName("Chỗ trống 2");
    expect(inputs[2]).toHaveAccessibleName("Chỗ trống 3");
  });

  it("keeps the words around each blank", () => {
    renderQuestion(
      question({
        type: "fill_blank",
        prompt: "If it {{1}} tomorrow, we cancel.",
        blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
      }),
    );
    expect(screen.getByText(/If it/)).toBeInTheDocument();
    expect(screen.getByText(/tomorrow, we cancel\./)).toBeInTheDocument();
  });

  it("preserves formatting that wraps a blank", () => {
    const { container } = render(
      <QuestionBody
        question={question({
          type: "fill_blank",
          prompt: "She **{{1}} here** now.",
          blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
        })}
        answer={undefined}
        onAnswer={vi.fn()}
      />,
    );
    const strong = container.querySelector("strong");
    expect(strong).not.toBeNull();
    expect(strong?.querySelector("input")).not.toBeNull();
    expect(container.textContent).not.toContain("**");
  });

  it("reports what was typed, keyed by blank id", async () => {
    const onAnswer = renderQuestion(
      question({
        type: "fill_blank",
        prompt: "If it {{1}} tomorrow.",
        blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
      }),
    );
    await userEvent.type(screen.getByRole("textbox"), "r");
    expect(onAnswer).toHaveBeenCalledWith({ type: "fill_blank", values: { b1: "r" } });
  });

  it("shows the marker rather than an unanswerable box when a blank is missing", () => {
    renderQuestion(
      question({ type: "fill_blank", prompt: "If it {{9}} tomorrow.", blanks: [] }),
    );
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(screen.getByText(/\{\{9\}\}/)).toBeInTheDocument();
  });
});

describe("short_answer", () => {
  it("gives a labelled textarea and counts the words", async () => {
    renderQuestion(question({ type: "short_answer", points: 5 }), {
      type: "text",
      value: "I usually wake up at six",
    });
    expect(screen.getByRole("textbox")).toHaveAccessibleName("Bài làm của bạn");
    expect(screen.getByText("6 từ")).toBeInTheDocument();
  });

  it("counts nothing as nothing", () => {
    renderQuestion(question({ type: "short_answer" }), { type: "text", value: "   " });
    expect(screen.getByText("0 từ")).toBeInTheDocument();
  });

  it("is the deck's 52px field, one line that grows, with its count line under it", () => {
    renderQuestion(question({ type: "short_answer" }));
    const field = screen.getByRole("textbox", { name: "Bài làm của bạn" });
    expect(field.tagName).toBe("TEXTAREA");
    expect(field).toHaveAttribute("rows", "1");
    expect(field).toHaveAttribute("placeholder", "Nhập câu trả lời");
    expect(field).toHaveClass(
      "min-h-13",
      "field-sizing-content",
      "rounded-[11px]",
      "border-[1.5px]",
      "border-border",
      "bg-card",
      "px-3.5",
      "py-[13px]",
      "leading-[1.45]",
      "focus:border-primary",
      "lg:text-md",
    );
    expect(field).toHaveAttribute("autocomplete", "off");
    expect(field).toHaveAttribute("spellcheck", "false");
    expect(screen.getByText("0 từ")).toHaveClass("text-meta", "text-muted-fg");
  });

  it("counts one word in the singular in English", async () => {
    await i18n.changeLanguage("en");
    try {
      renderQuestion(question({ type: "short_answer" }), {
        type: "text",
        value: "Parks",
      });
      expect(screen.getByText("1 word")).toBeInTheDocument();
    } finally {
      await i18n.changeLanguage("vi");
    }
  });

  it("keeps an answer written on several lines", async () => {
    const onAnswer = renderQuestion(question({ type: "short_answer" }), {
      type: "text",
      value: "First line\nSecond line",
    });
    expect(screen.getByRole("textbox")).toHaveValue("First line\nSecond line");
    await userEvent.type(screen.getByRole("textbox"), "!");
    expect(onAnswer).toHaveBeenLastCalledWith({
      type: "text",
      value: "First line\nSecond line!",
    });
  });

  it("reports what was typed as a text answer", async () => {
    const onAnswer = renderQuestion(question({ type: "short_answer" }));
    await userEvent.type(screen.getByRole("textbox"), "a");
    expect(onAnswer).toHaveBeenCalledWith({ type: "text", value: "a" });
  });
});

describe("a locked paper", () => {
  it("keeps the answers readable and refuses edits", () => {
    render(
      <QuestionBody
        question={question({ type: "single_choice", options })}
        answer={{ type: "choice", optionIds: ["o1"] }}
        onAnswer={vi.fn()}
        disabled
      />,
    );
    expect(screen.getByRole("radio", { name: /has been living/ })).toBeChecked();
    for (const radio of screen.getAllByRole("radio")) {
      expect(radio).toBeDisabled();
      expect(radio.parentElement).not.toHaveClass("hover:border-ring");
    }
  });

  it("refuses edits through the card once the store is locked", () => {
    renderWithPaper(
      <QuestionCard
        question={question({ type: "single_choice", options })}
        number={1}
        total={1}
        onAudioExpired={vi.fn()}
      />,
    );
    for (const radio of screen.getAllByRole("radio")) expect(radio).toBeEnabled();
    act(() => useTakeTestStore.getState().lockNow("superseded"));
    for (const radio of screen.getAllByRole("radio")) expect(radio).toBeDisabled();
  });
});

describe("blank ordering", () => {
  // The prompt decides where a slot goes; the ordinal decides which blank it belongs to.
  it("matches by ordinal however the blanks arrive", async () => {
    const onAnswer = vi.fn();
    render(
      <QuestionBody
        question={question({
          type: "fill_blank",
          prompt: "First {{1}}, then {{2}}, last {{3}}.",
          blanks: [
            { id: "third", ordinal: 3, caseSensitive: false },
            { id: "first", ordinal: 1, caseSensitive: false },
            { id: "second", ordinal: 2, caseSensitive: false },
          ],
        })}
        answer={undefined}
        onAnswer={onAnswer}
      />,
    );

    const inputs = screen.getAllByRole("textbox");
    expect(inputs.map((i) => i.getAttribute("aria-label"))).toEqual([
      "Chỗ trống 1",
      "Chỗ trống 2",
      "Chỗ trống 3",
    ]);

    await userEvent.type(inputs[1]!, "x");
    expect(onAnswer).toHaveBeenCalledWith({
      type: "fill_blank",
      values: { second: "x" },
    });
  });
});

/**
 * The points line is a promise about scoring, so it has to match what grading
 * actually does (O-17). If these two ever drift, the question tells the student
 * one rule and the server applies another.
 */
describe("what the question says it is worth", () => {
  const render1 = renderCard;

  it("names the per-blank share, as S-05 writes it", () => {
    render1(
      question({
        type: "fill_blank",
        points: 2,
        prompt: "If it {{1}} tomorrow, we {{2}} the trip.",
        blanks: [
          { id: "b1", ordinal: 1, caseSensitive: false },
          { id: "b2", ordinal: 2, caseSensitive: false },
        ],
      }),
    );
    expect(screen.getByText("2 điểm · mỗi chỗ trống 1 điểm")).toBeInTheDocument();
  });

  it("rounds an uneven share rather than inventing precision", () => {
    render1(
      question({
        type: "fill_blank",
        points: 2,
        prompt: "{{1}} {{2}} {{3}}",
        blanks: [
          { id: "b1", ordinal: 1, caseSensitive: false },
          { id: "b2", ordinal: 2, caseSensitive: false },
          { id: "b3", ordinal: 3, caseSensitive: false },
        ],
      }),
    );
    expect(screen.getByText("2 điểm · mỗi chỗ trống 0,67 điểm")).toBeInTheDocument();
  });

  it("says who grades a short answer, and claims no share", () => {
    render1(question({ type: "short_answer", points: 5 }));
    expect(screen.getByText("5 điểm · giáo viên chấm tay")).toBeInTheDocument();
  });

  it("says only the total for a choice question", () => {
    render1(question({ type: "single_choice", points: 1, options }));
    expect(screen.getByText("1 điểm")).toBeInTheDocument();
  });

  it("counts points in the singular and the plural in English", async () => {
    await i18n.changeLanguage("en");
    try {
      const { unmount } = render1(
        question({ type: "single_choice", points: 1, options }),
      );
      expect(screen.getByText("1 point")).toBeInTheDocument();
      unmount();
      render1(
        question({
          type: "fill_blank",
          points: 1.5,
          prompt: "{{1}}",
          blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
        }),
      );
      expect(screen.getByText("1,5 points · 1,5 per blank")).toBeInTheDocument();
    } finally {
      await i18n.changeLanguage("vi");
    }
  });
});

describe("the fill-blank matching rule (S-05)", () => {
  const render1 = renderCard;

  it("says capitals do not matter when no blank is case-sensitive", () => {
    render1(
      question({
        type: "fill_blank",
        prompt: "If it {{1}} tomorrow.",
        blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
      }),
    );
    expect(screen.getByRole("note")).toHaveTextContent(
      "Không phân biệt hoa thường. Viết đúng chính tả.",
    );
  });

  it("says capitals matter as soon as one blank is case-sensitive", () => {
    render1(
      question({
        type: "fill_blank",
        prompt: "{{1}} {{2}}",
        blanks: [
          { id: "b1", ordinal: 1, caseSensitive: false },
          { id: "b2", ordinal: 2, caseSensitive: true },
        ],
      }),
    );
    expect(screen.getByRole("note")).toHaveTextContent(
      "Phân biệt hoa thường. Viết đúng chính tả.",
    );
  });

  it("puts a short answer's worth and word count on one row", () => {
    render1(question({ type: "short_answer", points: 5 }));
    const row = screen.getByText("5 điểm · giáo viên chấm tay").parentElement!;
    expect(row).toHaveTextContent("0 từ");
    expect(screen.queryByRole("note")).toBeNull();
  });
});

describe("larger text in tests", () => {
  it("raises the prompt to 19px and the options to 17px", () => {
    localStorage.setItem("quizzivy.testText", "large");
    renderQuestion(question({ type: "single_choice", options }));
    const prompt = screen.getByText("Prompt").parentElement!;
    expect(prompt).toHaveClass("text-[1.1875rem]", "leading-[1.6]!");
    expect(prompt).not.toHaveClass("text-lg");
    const row = screen.getByRole("radio", { name: "have lived" }).parentElement!;
    expect(row).toHaveClass("text-lg", "leading-[1.4]");
    expect(row).not.toHaveClass("text-md");
  });

  it("raises the sentence to 20px, the blanks with it, and the answer field to 17px", () => {
    localStorage.setItem("quizzivy.testText", "large");
    const { unmount } = render(
      <QuestionBody
        question={question({
          type: "fill_blank",
          prompt: "If it {{1}} tomorrow.",
          blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
        })}
        answer={undefined}
        onAnswer={vi.fn()}
      />,
    );
    expect(screen.getByRole("textbox")).toHaveClass("text-[length:inherit]");
    expect(screen.getByRole("textbox").closest(".text-xl")).toHaveClass(
      "leading-[2.1]!",
    );
    unmount();
    renderQuestion(question({ type: "short_answer" }));
    expect(screen.getByRole("textbox")).toHaveClass("text-lg", "py-3", "min-h-13");
    expect(screen.getByRole("textbox")).not.toHaveClass("py-[13px]");
  });

  it("ignores a stored value that is not the setting", () => {
    localStorage.setItem("quizzivy.testText", "default");
    renderQuestion(question({ type: "single_choice", options }));
    expect(screen.getByText("Prompt").parentElement).toHaveClass("text-lg");
  });
});

describe("a fill-in's blanks", () => {
  const fill = question({
    type: "fill_blank",
    prompt: "If it {{1}} tomorrow.",
    blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
  });

  it("sit in the sentence as the deck's underlined fields, never under 16px where a phone would zoom", () => {
    renderQuestion(fill);
    const blank = screen.getByRole("textbox", { name: "Chỗ trống 1" });
    expect(blank).toHaveClass(
      "inline-block",
      "h-11",
      "lg:h-9.5",
      "border-0",
      "border-b-2",
      "border-ring",
      "bg-muted",
      "rounded-t-[6px]",
      "rounded-b-none",
      "px-2",
      "text-left",
      "font-medium",
      "text-[length:var(--text-input)]",
      "lg:text-[length:inherit]",
    );
    expect(blank).not.toHaveClass(
      "rounded-ctl",
      "border-[1.5px]",
      "bg-card",
      "text-center",
    );
    expect(blank).toHaveAttribute("placeholder", "nhập ở đây");
    expect(blank).toHaveAttribute("autocomplete", "off");
    expect(blank).toHaveAttribute("spellcheck", "false");
    expect(blank.closest("p")).toHaveTextContent("If it tomorrow.");
    expect(blank.closest(".text-\\[1\\.125rem\\]")).toHaveClass(
      "leading-[2.1]!",
      "font-medium",
    );
  });

  it("is up to 340px wide alone in its sentence and 128px beside another blank", () => {
    renderQuestion(fill);
    expect(screen.getByRole("textbox", { name: "Chỗ trống 1" })).toHaveClass(
      "w-[min(100%,340px)]",
    );
    cleanup();
    renderQuestion(
      question({
        type: "fill_blank",
        prompt: "If it {{1}} tomorrow, we {{2}} home.",
        blanks: [
          { id: "b1", ordinal: 1, caseSensitive: false },
          { id: "b2", ordinal: 2, caseSensitive: false },
        ],
      }),
    );
    for (const blank of screen.getAllByRole("textbox")) {
      expect(blank).toHaveClass("w-32");
      expect(blank).not.toHaveClass("w-[min(100%,340px)]");
    }
  });

  it("is 128px in a table, even as the only blank", () => {
    renderQuestion(
      question({
        type: "fill_blank",
        prompt: "| Verb | Past |\n| --- | --- |\n| go | {{1}} |",
        blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
      }),
    );
    const blank = screen.getByRole("textbox", { name: "Chỗ trống 1" });
    expect(blank.closest("table")).not.toBeNull();
    expect(blank).toHaveClass("w-32");
  });

  it("underlines in the primary tone once typed, and still writes the fill-in answer", async () => {
    const onAnswer = renderQuestion(fill, {
      type: "fill_blank",
      values: { b1: "rains" },
    });
    const blank = screen.getByRole("textbox", { name: "Chỗ trống 1" });
    expect(blank).toHaveClass("border-primary");
    expect(blank).not.toHaveClass("border-ring");
    await userEvent.type(blank, "!");
    expect(onAnswer).toHaveBeenLastCalledWith({
      type: "fill_blank",
      values: { b1: "rains!" },
    });
  });

  it("stay legible in a rich table on the paper surface", () => {
    const { container } = render(
      <QuestionBody question={fill} answer={undefined} onAnswer={vi.fn()} />,
    );
    expect(container.firstElementChild).toHaveClass(
      "[&_.content-table-scroll_input]:bg-paper",
      "[&_.content-table-scroll_input]:text-paper-fg",
      "[&_.content-table-scroll_input:focus]:border-paper-fg",
    );
  });

  it("states the case rule as the hint under the sentence", () => {
    renderQuestion(fill);
    expect(screen.getByRole("note")).toHaveClass("text-muted-fg", "text-sm", "-mt-2");
  });
});

describe("a type this page has no renderer for", () => {
  const unknown = question({
    type: "matching" as StudentQuestion["type"],
    options,
    blanks: [{ id: "b1", ordinal: 1, caseSensitive: false }],
  });

  it("is told apart from the five the page knows", () => {
    expect(questionKind(unknown)).toBe("unknown");
    expect(
      (
        [
          "single_choice",
          "multiple_choice",
          "true_false",
          "fill_blank",
          "short_answer",
        ] as const
      ).map((type) => questionKind({ type })),
    ).toEqual(["choice", "choice", "choice", "fill_blank", "short_answer"]);
    expect(questionKind({ type: "toString" as StudentQuestion["type"] })).toBe(
      "unknown",
    );
  });

  it("labels each of the five types in English", async () => {
    await i18n.changeLanguage("en");
    try {
      expect(
        (
          [
            "single_choice",
            "multiple_choice",
            "true_false",
            "fill_blank",
            "short_answer",
          ] as const
        ).map((type) => questionLine({ type }, 4, 8, i18n.t)),
      ).toEqual([
        "Question 4 of 8 · Single choice",
        "Question 4 of 8 · Multiple choice",
        "Question 4 of 8 · True / False",
        "Question 4 of 8 · Fill in the blank",
        "Question 4 of 8 · Short answer",
      ]);
      expect(questionLine(unknown, 4, 8, i18n.t)).toBe("Question 4 of 8");
    } finally {
      await i18n.changeLanguage("vi");
    }
  });

  it("draws no answer control in the body and writes nothing", async () => {
    const onAnswer = vi.fn();
    const { container } = render(
      <QuestionBody question={unknown} answer={undefined} onAnswer={onAnswer} />,
    );
    expect(container).toBeEmptyDOMElement();
    expect(onAnswer).not.toHaveBeenCalled();
  });

  it("gets the block that names the question and offers to reload, and claims no worth", async () => {
    const onAnswer = vi.fn();
    render(
      <QuestionSheet
        question={unknown}
        number={3}
        total={8}
        answer={{ type: "choice", optionIds: ["o1"] }}
        onAnswer={onAnswer}
      />,
    );
    expect(screen.getByText("Câu 3 trên 8")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Câu 3 thuộc dạng câu hỏi mới mà trang này chưa hiển thị được. Hãy tải lại trang để làm câu này; các câu trả lời của bạn vẫn được giữ.",
    );
    expect(screen.getByRole("button", { name: "Tải lại trang" })).toBeEnabled();
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByText("Prompt")).toBeNull();
    expect(screen.queryByText(/điểm/)).toBeNull();
    expect(onAnswer).not.toHaveBeenCalled();
  });

  it("writes nothing to the store through the card, which keeps the flag toggle", () => {
    renderWithPaper(
      <QuestionCard question={unknown} number={3} total={8} onAudioExpired={vi.fn()} />,
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      "Câu 3 thuộc dạng câu hỏi mới",
    );
    expect(screen.getByRole("button", { name: "Đánh dấu xem lại" })).toBeEnabled();
    expect(useTakeTestStore.getState().answers).toEqual({});
    expect(useTakeTestStore.getState().dirty.size).toBe(0);
  });

  it("reloads from its one action", async () => {
    const onReload = vi.fn();
    render(<UnknownType number={3} onReload={onReload} />);
    await userEvent.click(screen.getByRole("button", { name: "Tải lại trang" }));
    expect(onReload).toHaveBeenCalledOnce();
  });

  it("reloads the page from the sheet, which gives the block no handler", async () => {
    const reload = vi.fn();
    vi.stubGlobal("location", { ...window.location, reload });
    try {
      render(
        <QuestionSheet
          question={unknown}
          number={3}
          total={8}
          answer={undefined}
          onAnswer={vi.fn()}
        />,
      );
      await userEvent.click(screen.getByRole("button", { name: "Tải lại trang" }));
      expect(reload).toHaveBeenCalledOnce();
    } finally {
      vi.unstubAllGlobals();
    }
  });
});

describe("the question sheet", () => {
  it("has no flag toggle and no heading id unless the caller gives them", () => {
    render(
      <QuestionSheet
        question={question({ type: "single_choice", options })}
        number={2}
        total={5}
        answer={undefined}
        onAnswer={vi.fn()}
      />,
    );
    expect(screen.queryByRole("button")).toBeNull();
    const line = screen.getByText("Câu 2 trên 5 · Một đáp án");
    expect(line).not.toHaveAttribute("id");
    expect(line).not.toHaveAttribute("tabindex");
  });

  it("says the worth under the answer at every width", () => {
    render(
      <QuestionSheet
        question={question({ type: "single_choice", points: 2, options })}
        number={1}
        total={1}
        answer={undefined}
        onAnswer={vi.fn()}
      />,
    );
    const worth = screen.getByText("2 điểm");
    expect(worth).toHaveClass("text-meta", "text-muted-fg");
    expect(worth.className).not.toMatch(/hidden/);
    expect(
      screen.getByRole("radiogroup").compareDocumentPosition(worth) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("is read-only when disabled", () => {
    render(
      <QuestionSheet
        question={question({ type: "short_answer" })}
        number={1}
        total={1}
        answer={{ type: "text", value: "kept" }}
        onAnswer={vi.fn()}
        disabled
      />,
    );
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(screen.getByRole("textbox")).toHaveValue("kept");
  });
});
