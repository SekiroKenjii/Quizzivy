import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  OutlineTree,
  type OutlineQuestion,
} from "@/features/tests/components/OutlineTree";
import type { OutlineSection } from "@/features/tests/outline";
import "@/lib/i18n";

const initial: OutlineSection[] = [
  {
    id: "s1",
    clientId: "client-1",
    title: "Reading",
    instructions: "Read the passage first",
    questionIds: ["q1"],
  },
  {
    id: "s2",
    clientId: "client-2",
    title: "A very long listening section",
    instructions: null,
    questionIds: ["q2"],
  },
  {
    id: "s3",
    clientId: "client-3",
    title: "Empty",
    instructions: null,
    questionIds: [],
  },
];
const questions = new Map<string, OutlineQuestion>([
  [
    "q1",
    { id: "q1", prompt: "First question", points: 2, hasAudio: false, problem: null },
  ],
  [
    "q2",
    { id: "q2", prompt: "Second question", points: 3, hasAudio: true, problem: null },
  ],
]);
afterEach(() => vi.restoreAllMocks());
function setup(selectedId: string | null = "q2") {
  const changed = vi.fn(),
    add = vi.fn();
  function Harness() {
    const [sections, setSections] = useState(initial);
    return (
      <OutlineTree
        sections={sections}
        questions={questions}
        selectedId={selectedId}
        creating={false}
        onSelect={vi.fn()}
        onCreateQuestion={add}
        onPickFromBank={vi.fn()}
        onAddSection={vi.fn()}
        onChange={(next) => {
          changed(next);
          setSections(next);
        }}
      />
    );
  }
  return { ...render(<Harness />), user: userEvent.setup(), changed, add };
}
describe("the deck outline", () => {
  it("exposes existing instructions under the open header and edits them through the real dialog", async () => {
    const { user, changed } = setup();
    await user.click(screen.getByRole("button", { name: "Read the passage first" }));
    const dialog = screen.getByRole("dialog");
    await user.clear(within(dialog).getByRole("textbox"));
    await user.type(within(dialog).getByRole("textbox"), "New instructions");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    expect(changed).toHaveBeenCalledWith([
      { ...initial[0], instructions: "New instructions" },
      initial[1],
      initial[2],
    ]);
    expect(
      screen.getByRole("button", { name: "New instructions" }),
    ).toBeInTheDocument();
  });
  it("shows the real selected section as the abbreviated add-question destination", async () => {
    const { user, add } = setup();
    const button = screen.getByRole("button", {
      name: /Thêm câu hỏi.*A very long lis…/,
    });
    await user.click(button);
    expect(add).toHaveBeenCalledOnce();
    expect(screen.getByText("2 câu · 5 điểm")).toBeInTheDocument();
  });
  it("provides section grips and keeps a collapsed client section attached after a menu reorder", async () => {
    const { user, changed, container } = setup();
    expect(
      screen.getByRole("button", { name: "Kéo để đổi vị trí phần Reading" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Reading.*1 · 2đ/ }));
    await user.click(screen.getAllByRole("button", { name: "Thao tác với phần" })[0]!);
    await user.click(screen.getByRole("menuitem", { name: "Di chuyển xuống" }));
    expect(changed).toHaveBeenCalledWith([initial[1], initial[0], initial[2]]);
    expect(
      container.querySelector('[data-outline-section="s1"]'),
    ).not.toHaveTextContent("First question");
    expect(container.querySelector('[data-outline-section="s2"]')).toHaveTextContent(
      "Second question",
    );
  });
});

describe("dnd-kit public keyboard actions", () => {
  it("moves the whole section with Space, ArrowDown and Space while retaining its client identity", async () => {
    const { user, changed } = setup();
    vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
      this: Element,
    ) {
      const section = this.closest("[data-outline-section]");
      const index = [...document.querySelectorAll("[data-outline-section]")].indexOf(
        section!,
      );
      const top = Math.max(0, index) * 120;
      const height = this.parentElement?.hasAttribute("data-outline-section")
        ? 100
        : 32;
      return {
        width: 300,
        height,
        top,
        left: 0,
        right: 300,
        bottom: top + height,
        x: 0,
        y: top,
        toJSON: () => ({}),
      };
    });
    const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí phần Reading" });
    expect(
      document.getElementById(grip.getAttribute("aria-describedby")!),
    ).toHaveTextContent("phím cách");
    grip.focus();
    await user.keyboard(" ");
    await waitFor(() => expect(grip).toHaveAttribute("aria-pressed", "true"));
    await user.keyboard("{ArrowDown}");
    await waitFor(() =>
      expect(document.querySelector("[data-outline-drop]")).toBeInTheDocument(),
    );
    await user.keyboard(" ");
    await waitFor(() =>
      expect(changed).toHaveBeenCalledWith([initial[1], initial[0], initial[2]]),
    );
    expect(
      screen.getByRole("button", { name: "Kéo để đổi vị trí phần Reading" }),
    ).toHaveFocus();
  });
  it("cancels section drag with Escape without changing the outline or leaving a drop indicator", async () => {
    const { user, changed } = setup();
    vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
      this: Element,
    ) {
      const section = this.closest("[data-outline-section]");
      const index = [...document.querySelectorAll("[data-outline-section]")].indexOf(
        section!,
      );
      const top = Math.max(0, index) * 120;
      const height = this.parentElement?.hasAttribute("data-outline-section")
        ? 100
        : 32;
      return {
        width: 300,
        height,
        top,
        left: 0,
        right: 300,
        bottom: top + height,
        x: 0,
        y: top,
        toJSON: () => ({}),
      };
    });
    const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí phần Reading" });
    grip.focus();
    await user.keyboard(" ");
    await waitFor(() => expect(grip).toHaveAttribute("aria-pressed", "true"));
    await user.keyboard("{ArrowDown}");
    await waitFor(() =>
      expect(document.querySelector("[data-outline-drop]")).toBeInTheDocument(),
    );
    await user.keyboard("{Escape}");
    expect(changed).not.toHaveBeenCalled();
    expect(document.querySelector("[data-outline-drop]")).toBeNull();
    expect(
      screen.getByRole("button", { name: "Kéo để đổi vị trí phần Reading" }),
    ).toHaveFocus();
  });
});

describe("dropping on a collapsed header", () => {
  it("moves a real question into that section, opens it and retains each section identity", async () => {
    const { user, changed, container } = setup("q1");
    await user.click(
      screen.getByRole("button", { name: /^A very long listening section / }),
    );
    vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
      this: Element,
    ) {
      const section = this.closest("[data-outline-section]");
      const index = [...document.querySelectorAll("[data-outline-section]")].indexOf(
        section!,
      );
      const top =
        Math.max(0, index) * 120 + (this.matches("[data-outline-row]") ? 40 : 0);
      const height = this.parentElement?.hasAttribute("data-outline-section")
        ? 100
        : 32;
      return {
        width: 300,
        height,
        top,
        left: 0,
        right: 300,
        bottom: top + height,
        x: 0,
        y: top,
        toJSON: () => ({}),
      };
    });
    const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí câu 1" });
    grip.focus();
    await user.keyboard(" ");
    await waitFor(() => expect(grip).toHaveAttribute("aria-pressed", "true"));
    await user.keyboard("{ArrowDown}");
    await waitFor(() =>
      expect(screen.getByText("Đã đến vị trí thả.")).toBeInTheDocument(),
    );
    await user.keyboard(" ");
    await waitFor(() => expect(changed).toHaveBeenCalledOnce());
    const result = changed.mock.calls[0]![0] as OutlineSection[];
    expect(result.map((section) => section.questionIds)).toEqual([
      [],
      ["q1", "q2"],
      [],
    ]);
    expect(result.map((section) => section.clientId)).toEqual(
      initial.map((section) => section.clientId),
    );
    expect(container.querySelector('[data-outline-section="s2"]')).toHaveTextContent(
      "First question",
    );
    expect(container.querySelector('[data-outline-section="s2"]')).toHaveTextContent(
      "Second question",
    );
  });
});

describe("an unresolved shared-context unit", () => {
  it("reports loading rather than pretending its unknown members are zero questions", () => {
    render(
      <OutlineTree
        sections={[
          { ...initial[0]!, questionIds: [], units: [{ kind: "group", id: "g1" }] },
        ]}
        questions={new Map()}
        selectedId={null}
        creating={false}
        onSelect={vi.fn()}
        onChange={vi.fn()}
        onCreateQuestion={vi.fn()}
        onPickFromBank={vi.fn()}
        onAddSection={vi.fn()}
      />,
    );
    expect(screen.queryByText("0 câu")).toBeNull();
    expect(screen.getAllByText("Đang tải…")).toHaveLength(2);
  });
});

it("tints the collapsed header under a whole-group drop and writes its ordered unit only on drop", async () => {
  const { emptyGroup, newGroupQuestion } =
    await import("@/features/question-groups/model");
  const i18n = (await import("@/lib/i18n")).default;
  const bundle = emptyGroup("Whole group");
  const first = { ...newGroupQuestion(i18n.t), id: "member-first" };
  const second = { ...newGroupQuestion(i18n.t), id: "member-second" };
  first.input.prompt = "First member";
  second.input.prompt = "Second member";
  bundle.questions = [first, second];
  bundle.group.members = [
    { questionId: first.id, optionOrder: "shuffle" },
    { questionId: second.id, optionOrder: "shuffle" },
  ];
  const start: OutlineSection[] = [
    {
      ...initial[0]!,
      questionIds: [],
      units: [{ kind: "group", id: bundle.group.id }],
    },
    initial[1]!,
  ];
  const changed = vi.fn();
  function Harness() {
    const [sections, setSections] = useState(start);
    return (
      <OutlineTree
        sections={sections}
        questions={questions}
        groups={new Map([[bundle.group.id, bundle]])}
        selectedId={null}
        creating={false}
        onSelect={vi.fn()}
        onCreateQuestion={vi.fn()}
        onPickFromBank={vi.fn()}
        onAddSection={vi.fn()}
        onChange={(next) => {
          changed(next);
          setSections(next);
        }}
      />
    );
  }
  const { container } = render(<Harness />);
  const user = userEvent.setup();
  const destination = screen.getByRole("button", {
    name: /^A very long listening section /,
  });
  await user.click(destination);
  expect(destination).toHaveAttribute("aria-expanded", "false");
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
    this: Element,
  ) {
    const section = this.closest("[data-outline-section]");
    const index = [...document.querySelectorAll("[data-outline-section]")].indexOf(
      section!,
    );
    const top =
      Math.max(0, index) * 120 + (this.closest("[data-outline-group]") ? 40 : 0);
    const height = this.parentElement?.hasAttribute("data-outline-section") ? 100 : 32;
    return {
      width: 300,
      height,
      top,
      left: 0,
      right: 300,
      bottom: top + height,
      x: 0,
      y: top,
      toJSON: () => ({}),
    };
  });
  const grip = screen.getByRole("button", { name: "Kéo nhóm Whole group" });
  grip.focus();
  await user.keyboard(" ");
  await waitFor(() => expect(grip).toHaveAttribute("aria-pressed", "true"));
  await user.keyboard("{ArrowDown}");
  await waitFor(() =>
    expect(screen.getByText("Đã đến vị trí thả.")).toBeInTheDocument(),
  );
  expect(changed).not.toHaveBeenCalled();
  const targetSection = container.querySelector<HTMLElement>(
    '[data-outline-section="s2"]',
  )!;
  await waitFor(() =>
    expect(targetSection.querySelector("[data-outline-drop-into]")).toBeInTheDocument(),
  );
  expect(container.querySelectorAll("[data-outline-drop-into]")).toHaveLength(1);
  expect(targetSection.querySelector("[data-outline-drop]")).toBeNull();
  expect(changed).not.toHaveBeenCalled();
  await user.keyboard(" ");
  await waitFor(() => expect(changed).toHaveBeenCalledOnce());
  const result = changed.mock.calls[0]![0] as OutlineSection[];
  expect(result[0]!.units).toEqual([]);
  expect(result[1]!.units).toEqual([
    { kind: "group", id: bundle.group.id },
    { kind: "question", id: "q2" },
  ]);
  expect(result.map((section) => section.clientId)).toEqual(
    start.map((section) => section.clientId),
  );
  expect(
    container.querySelectorAll(`[data-outline-group="${bundle.group.id}"]`),
  ).toHaveLength(1);
  expect(destination).toHaveAttribute("aria-expanded", "true");
  const members = within(targetSection).getAllByRole("button", {
    name: /First member|Second member/,
  });
  expect(members.map((member) => member.textContent)).toEqual([
    expect.stringMatching(/^1First member\d+đ$/),
    expect.stringMatching(/^2Second member\d+đ$/),
  ]);
});
