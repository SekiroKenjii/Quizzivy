import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QuestionBody } from "@/features/take-test/components/QuestionBody";
import { QuestionCard } from "@/features/take-test/components/QuestionCard";
import { QuestionSheet } from "@/features/take-test/components/QuestionSheet";
import { questionKind, questionLine } from "@/features/take-test/questionType";
import { useTakeTestStore } from "@/features/take-test/store";
import { PaperSourceContext, type PaperSource } from "@/features/take-test/paper";
import type { Answer, StudentQuestion } from "@/features/take-test/api";
import { writeLargerTestText } from "@/lib/testText";
import i18n from "@/lib/i18n";
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

const places = [
  { id: "o1", key: "i", text: "Library" },
  { id: "o2", key: "ii", text: "Post office" },
  { id: "o3", key: "iii", text: "Pharmacy" },
];

function matching(over: Partial<StudentQuestion> = {}): StudentQuestion {
  return {
    id: "q1",
    sectionId: "s1",
    type: "matching",
    prompt: "Match the signs with the places.",
    points: 3,
    options: places,
    matchItems: [
      { id: "m1", key: "A", text: "QUIET PLEASE" },
      { id: "m2", key: "B", text: "STAMPS SOLD HERE" },
    ],
    matchAllowReuse: false,
    ...over,
  };
}

function renderMatching(q: StudentQuestion, answer?: Answer) {
  const onAnswer = vi.fn();
  render(<QuestionBody question={q} answer={answer} onAnswer={onAnswer} />);
  return onAnswer;
}

const card = (key: string) =>
  screen.getByRole("radiogroup", { name: new RegExp(`^${key}\\b`) });
const chip = (key: string, chipKey: string) =>
  within(card(key)).getByRole("radio", { name: new RegExp(`^${chipKey}\\b`) });

describe("matching", () => {
  it("is drawn by the engine now, under its own type label", () => {
    expect(questionKind({ type: "matching" })).toBe("matching");
    expect(questionLine(matching(), 31, 34, i18n.t)).toBe("Câu 31 trên 34 · Nối");
  });

  it("draws one card per item with its key, its text and a chip per option key", () => {
    renderMatching(matching());
    expect(screen.getAllByRole("radiogroup")).toHaveLength(2);
    expect(screen.getByText("QUIET PLEASE")).toBeInTheDocument();
    expect(
      within(card("A"))
        .getAllByRole("radio")
        .map((radio) => radio.textContent),
    ).toEqual(["i", "ii", "iii"]);
    expect(screen.getByText("Đã nối 0/2")).toBeInTheDocument();
  });

  it("pairs a chip, says how many are matched, and borders a matched card in primary", async () => {
    const onAnswer = renderMatching(matching(), {
      type: "matching",
      pairs: { m1: "o1" },
    });
    expect(screen.getByText("Đã nối 1/2")).toBeInTheDocument();
    expect(chip("A", "i")).toHaveAttribute("aria-checked", "true");
    expect(chip("A", "i")).toHaveClass("bg-primary", "text-primary-fg");
    expect(card("A").parentElement).toHaveClass("border-primary", "rounded-[11px]");
    expect(card("B").parentElement).toHaveClass("border-border");
    await userEvent.click(chip("B", "ii"));
    expect(onAnswer).toHaveBeenCalledWith({
      type: "matching",
      pairs: { m1: "o1", m2: "o2" },
    });
  });

  it("clears a pair when its chosen chip is tapped again", async () => {
    const onAnswer = renderMatching(matching(), {
      type: "matching",
      pairs: { m1: "o1", m2: "o2" },
    });
    await userEvent.click(chip("A", "i"));
    expect(onAnswer).toHaveBeenCalledWith({ type: "matching", pairs: { m2: "o2" } });
  });

  it("without reuse, fades a chip another item holds, says where, and moves it when chosen", async () => {
    const onAnswer = renderMatching(matching(), {
      type: "matching",
      pairs: { m1: "o1" },
    });
    const held = within(card("B")).getByRole("radio", { name: "i, đã nối với A" });
    expect(held).toHaveClass("text-muted-fg", "border-border/50");
    expect(held).not.toHaveClass("opacity-50");
    await userEvent.click(held);
    expect(onAnswer).toHaveBeenCalledWith({ type: "matching", pairs: { m2: "o1" } });
    expect(screen.getByText("i đã chuyển từ A")).toHaveAttribute("aria-live", "polite");
  });

  it("with reuse, lets two items hold one chip and fades nothing", async () => {
    const onAnswer = renderMatching(matching({ matchAllowReuse: true }), {
      type: "matching",
      pairs: { m1: "o1" },
    });
    const same = chip("B", "i");
    expect(same).toHaveAccessibleName("i");
    expect(same).not.toHaveClass("text-muted-fg");
    await userEvent.click(same);
    expect(onAnswer).toHaveBeenCalledWith({
      type: "matching",
      pairs: { m1: "o1", m2: "o1" },
    });
  });

  it("is keyboard-operable: one stop per card, arrows move, Space chooses", async () => {
    const onAnswer = renderMatching(matching(), {
      type: "matching",
      pairs: { m2: "o2" },
    });
    const user = userEvent.setup();
    await user.tab();
    expect(chip("A", "i")).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(chip("A", "ii")).toHaveFocus();
    expect(onAnswer).not.toHaveBeenCalled();
    await user.keyboard("{ArrowLeft}{ArrowLeft}");
    expect(chip("A", "iii")).toHaveFocus();
    await user.keyboard(" ");
    expect(onAnswer).toHaveBeenCalledWith({
      type: "matching",
      pairs: { m2: "o2", m1: "o3" },
    });
    await user.tab();
    expect(chip("B", "ii")).toHaveFocus();
  });

  it("keeps the arrows from turning the page while a chip has focus", () => {
    renderMatching(matching());
    const event = new KeyboardEvent("keydown", {
      key: "ArrowRight",
      bubbles: true,
      cancelable: true,
    });
    act(() => {
      chip("A", "i").focus();
      chip("A", "i").dispatchEvent(event);
    });
    expect(event.defaultPrevented).toBe(true);
  });

  it("draws 44px chips below 1024 and 38px from 1024, radius 9", () => {
    viewport(768);
    renderMatching(matching());
    expect(chip("A", "i")).toHaveClass("h-11", "min-w-11", "rounded-[9px]");
    cleanup();
    viewport(1024);
    renderMatching(matching());
    expect(chip("A", "i")).toHaveClass("h-9.5", "min-w-9.5");
  });

  it("lists the options once under a title, with the items that use them, beside the items from 1000 wide", () => {
    viewport(1280);
    renderMatching(matching(), { type: "matching", pairs: { m1: "o2" } });
    const list = screen.getByRole("list", { name: "Các lựa chọn" });
    const rows = within(list).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      "iLibrary",
      "iiPost officeđược nối với A",
      "iiiPharmacy",
    ]);
    expect(screen.getByText("Các lựa chọn").tagName).toBe("P");
    expect(list.parentElement?.parentElement).toHaveClass(
      "grid-cols-[220px_minmax(0,1fr)]",
    );
  });

  it("stacks the options card above the items below 1000 and beside shared content", () => {
    viewport(768);
    renderMatching(matching());
    const list = screen.getByRole("list", { name: "Các lựa chọn" });
    expect(list.parentElement?.parentElement).toHaveClass("grid-cols-1");
    cleanup();
    viewport(1280);
    const source = {
      groups: [{ id: "g1", questionIds: ["q1"], stimuli: [{ id: "st" }] }],
    } as unknown as PaperSource;
    render(
      <PaperSourceContext value={source}>
        <QuestionBody question={matching()} answer={undefined} onAnswer={vi.fn()} />
      </PaperSourceContext>,
    );
    const beside = screen.getByRole("list", { name: "Các lựa chọn" });
    expect(beside.parentElement?.parentElement).toHaveClass("grid-cols-1");
  });

  it("draws no options card when the options have no text", () => {
    viewport(1280);
    renderMatching(
      matching({ options: places.map((place) => ({ ...place, text: "" })) }),
    );
    expect(screen.queryByRole("list")).toBeNull();
  });

  it("shows the keys as authored, with no nouns", () => {
    renderMatching(
      matching({
        options: [
          { id: "o1", key: "1", text: "" },
          { id: "o2", key: "2", text: "" },
        ],
      }),
    );
    expect(
      within(card("A"))
        .getAllByRole("radio")
        .map((r) => r.textContent),
    ).toEqual(["1", "2"]);
  });

  it("is the deck's wording in English", async () => {
    await i18n.changeLanguage("en");
    try {
      viewport(1280);
      renderMatching(matching(), { type: "matching", pairs: { m1: "o1" } });
      expect(screen.getByText("1 of 2 matched")).toBeInTheDocument();
      expect(
        within(card("B")).getByRole("radio", { name: "i, matched to A" }),
      ).toBeInTheDocument();
      expect(screen.getByRole("list", { name: "Options" })).toBeInTheDocument();
      expect(questionLine(matching(), 31, 34, i18n.t)).toBe(
        "Question 31 of 34 · Matching",
      );
    } finally {
      await i18n.changeLanguage("vi");
    }
  });

  it("raises the item text with larger text in tests", () => {
    act(() => writeLargerTestText(true));
    renderMatching(matching());
    expect(screen.getByText("QUIET PLEASE")).toHaveClass("text-lg");
  });

  it("is read-only on a locked paper", async () => {
    const onAnswer = vi.fn();
    render(
      <QuestionBody
        question={matching()}
        answer={{ type: "matching", pairs: { m1: "o1" } }}
        onAnswer={onAnswer}
        disabled
      />,
    );
    for (const radio of screen.getAllByRole("radio")) expect(radio).toBeDisabled();
    expect(chip("A", "i")).toHaveAttribute("aria-checked", "true");
  });

  it("is one question with one worth line, and saves through the card", async () => {
    render(
      <QuestionSheet
        question={matching()}
        number={31}
        total={34}
        answer={undefined}
        onAnswer={vi.fn()}
      />,
    );
    expect(screen.getByText("Câu 31 trên 34 · Nối")).toBeInTheDocument();
    expect(screen.getByText(/^3 điểm/)).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
    cleanup();
    renderWithPaper(
      <QuestionCard
        question={matching()}
        number={1}
        total={1}
        onAudioExpired={vi.fn()}
      />,
    );
    await userEvent.click(chip("B", "iii"));
    expect(useTakeTestStore.getState().answers["q1"]).toEqual({
      type: "matching",
      pairs: { m2: "o3" },
    });
    expect(useTakeTestStore.getState().dirty.has("q1")).toBe(true);
  });
});
