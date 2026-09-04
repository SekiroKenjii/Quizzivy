import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  OutlineTree,
  type OutlineQuestion,
} from "@/features/tests/components/OutlineTree";
import type { OutlineSection } from "@/features/tests/outline";
import "@/lib/i18n";

function sections(): OutlineSection[] {
  return [
    { id: "s1", title: "Ngữ pháp", instructions: null, questionIds: ["q1", "q2"] },
    { id: "s2", title: "Nghe", instructions: null, questionIds: ["q3"] },
  ];
}

const questions = new Map<string, OutlineQuestion>([
  ["q1", { id: "q1", prompt: "Câu một", points: 1, hasAudio: false, problem: null }],
  ["q2", { id: "q2", prompt: "Câu hai", points: 2, hasAudio: false, problem: null }],
  ["q3", { id: "q3", prompt: "Câu ba", points: 2, hasAudio: true, problem: null }],
]);

function renderTree(onChange = vi.fn()) {
  function Harness() {
    const [value, setValue] = useState(sections());
    return (
      <OutlineTree
        sections={value}
        questions={questions}
        selectedId="q1"
        creating={false}
        onCreateQuestion={vi.fn()}
        onPickFromBank={vi.fn()}
        onAddSection={vi.fn()}
        onSelect={vi.fn()}
        onChange={(next) => {
          onChange(next);
          setValue(next);
        }}
      />
    );
  }
  render(<Harness />);
  return { user: userEvent.setup(), onChange };
}

const order = (value: OutlineSection[]) => value.map((s) => s.questionIds);

describe("reordering the outline", () => {
  it("is achievable with the keyboard alone", async () => {
    const { user, onChange } = renderTree();

    // Tab until the move control is focused, then activate it.
    await user.tab();
    let guard = 0;
    while (
      document.activeElement?.getAttribute("aria-label") !==
        "Chuyển câu 1 xuống dưới" &&
      guard < 20
    ) {
      await user.tab();
      guard += 1;
    }
    expect(document.activeElement).toHaveAttribute(
      "aria-label",
      "Chuyển câu 1 xuống dưới",
    );

    await user.keyboard("{Enter}");

    expect(order(onChange.mock.calls[0]![0] as OutlineSection[])).toEqual([
      ["q2", "q1"],
      ["q3"],
    ]);
  });

  it("crosses a section boundary, which is the reason the outline moves at all", async () => {
    const { user, onChange } = renderTree();

    await user.click(screen.getByLabelText("Chuyển câu 2 xuống dưới"));

    expect(order(onChange.mock.calls[0]![0] as OutlineSection[])).toEqual([
      ["q1"],
      ["q2", "q3"],
    ]);
  });

  it("does nothing at the ends rather than wrapping around", async () => {
    const { user, onChange } = renderTree();

    await user.click(screen.getByLabelText("Chuyển câu 1 lên trên"));
    expect(onChange).not.toHaveBeenCalled();

    await user.click(screen.getByLabelText("Chuyển câu 3 xuống dưới"));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("renumbers across sections, the way a student counts", () => {
    renderTree();

    expect(screen.getByLabelText("Chuyển câu 3 lên trên")).toBeInTheDocument();
    expect(screen.getByText("Câu ba")).toBeInTheDocument();
  });

  it("shows a publish problem on the offending row instead of its prompt", () => {
    const withProblem = new Map(questions);
    withProblem.set("q2", {
      ...questions.get("q2")!,
      problem: "Câu 2 chưa có đáp án đúng",
    });

    render(
      <OutlineTree
        sections={sections()}
        questions={withProblem}
        selectedId={null}
        creating={false}
        onCreateQuestion={vi.fn()}
        onPickFromBank={vi.fn()}
        onAddSection={vi.fn()}
        onSelect={vi.fn()}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByText("Câu 2 chưa có đáp án đúng")).toBeInTheDocument();
    expect(screen.queryByText("Câu hai")).toBeNull();
  });
});

describe("taking a question out of the test", () => {
  it("removes it from its section and leaves the rest in order", async () => {
    const { user, onChange } = renderTree();

    await user.click(screen.getByRole("button", { name: "Gỡ câu 2 khỏi đề" }));

    expect(order(onChange.mock.calls[0]![0])).toEqual([["q1"], ["q3"]]);
    expect(screen.queryByText("Câu hai")).toBeNull();
    expect(screen.getByText("Câu một")).toBeInTheDocument();
    expect(screen.getByText("Câu ba")).toBeInTheDocument();
  });
});
