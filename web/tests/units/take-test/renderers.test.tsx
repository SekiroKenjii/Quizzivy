import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QuestionBody } from "@/features/take-test/components/QuestionBody";
import { QuestionCard } from "@/features/take-test/components/QuestionCard";
import type { Answer, StudentQuestion } from "@/features/take-test/api";
import "@/lib/i18n";

function question(
  over: Partial<StudentQuestion> & { type: StudentQuestion["type"] },
): StudentQuestion {
  return { id: "q1", prompt: "Prompt", points: 1, ...over };
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
});

describe("multiple_choice", () => {
  it("uses checkboxes, because more than one may be right", () => {
    renderQuestion(question({ type: "multiple_choice", options }));
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
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
});

describe("fill_blank", () => {
  // T-3.11's named case.
  it("puts three labelled inputs in prompt order", () => {
    renderQuestion(
      question({
        type: "fill_blank",
        prompt: "If it {{1}} tomorrow, we {{2}} the trip, and {{3}} home.",
        blanks: [
          { id: "b1", ordinal: 1 },
          { id: "b2", ordinal: 2 },
          { id: "b3", ordinal: 3 },
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
        blanks: [{ id: "b1", ordinal: 1 }],
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
          blanks: [{ id: "b1", ordinal: 1 }],
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
        blanks: [{ id: "b1", ordinal: 1 }],
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
            { id: "third", ordinal: 3 },
            { id: "first", ordinal: 1 },
            { id: "second", ordinal: 2 },
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
  const render1 = (q: StudentQuestion) =>
    render(<QuestionCard question={q} onAudioExpired={vi.fn()} />);

  it("names the per-blank share, as S-05 writes it", () => {
    render1(
      question({
        type: "fill_blank",
        points: 2,
        prompt: "If it {{1}} tomorrow, we {{2}} the trip.",
        blanks: [
          { id: "b1", ordinal: 1 },
          { id: "b2", ordinal: 2 },
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
          { id: "b1", ordinal: 1 },
          { id: "b2", ordinal: 2 },
          { id: "b3", ordinal: 3 },
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
});
