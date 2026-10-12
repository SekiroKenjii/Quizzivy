import { useState } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OutlineTree } from "@/features/tests/components/OutlineTree";
import type { OutlineSection } from "@/features/tests/outline";
import "@/lib/i18n";

const tall: OutlineSection = {
  id: "tall",
  clientId: "client-tall",
  title: "Tall",
  instructions: null,
  questionIds: [],
};
const middle: OutlineSection = {
  id: "middle",
  clientId: "client-middle",
  title: "Middle",
  instructions: null,
  questionIds: [],
};
const last: OutlineSection = {
  id: "last",
  clientId: "client-last",
  title: "Last",
  instructions: null,
  questionIds: [],
};

afterEach(() => vi.restoreAllMocks());

function setup(initial: OutlineSection[]) {
  const changed = vi.fn();
  function Harness() {
    const [sections, setSections] = useState(initial);
    return (
      <OutlineTree
        sections={sections}
        questions={new Map()}
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
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
    this: Element,
  ) {
    const section = this.closest("[data-outline-section]");
    const ordered = [...document.querySelectorAll("[data-outline-section]")];
    const index = ordered.indexOf(section!);
    const top = ordered
      .slice(0, Math.max(0, index))
      .reduce(
        (sum, node) =>
          sum + (node.getAttribute("data-outline-section") === "tall" ? 234 : 40),
        0,
      );
    const whole = this.parentElement?.hasAttribute("data-outline-section");
    const isTall = section?.getAttribute("data-outline-section") === "tall";
    const sectionHeight = isTall ? 234 : 40;
    const height = whole ? sectionHeight : 32;
    const width = isTall ? 320 : 280;
    return {
      width,
      height,
      top,
      left: 10,
      right: 10 + width,
      bottom: top + height,
      x: 10,
      y: top,
      toJSON: () => ({}),
    };
  });
  return { ...render(<Harness />), changed, user: userEvent.setup() };
}

it.each([
  {
    name: "down",
    initial: [tall, middle, last],
    arrow: "{ArrowDown}",
    expected: [middle, tall, last],
  },
  {
    name: "up",
    initial: [middle, last, tall],
    arrow: "{ArrowUp}",
    expected: [middle, tall, last],
  },
])(
  "targets the immediately adjacent unequal section with one Arrow $name",
  async ({ initial, arrow, expected }) => {
    const { user, changed, container } = setup(initial);
    const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí phần Tall" });
    grip.focus();
    await user.keyboard(" ");
    await waitFor(() => expect(grip).toHaveAttribute("aria-pressed", "true"));
    await user.keyboard(arrow);
    const target = arrow === "{ArrowDown}" ? "middle" : "last";
    await waitFor(() =>
      expect(
        container.querySelector(
          '[data-outline-section="' + target + '"] [data-outline-drop]',
        ),
      ).toBeInTheDocument(),
    );
    expect(container.querySelectorAll("[data-outline-drop]")).toHaveLength(1);
    await user.keyboard(" ");
    await waitFor(() => expect(changed).toHaveBeenCalledExactlyOnceWith(expected));
    expect(
      screen.getByRole("button", { name: "Kéo để đổi vị trí phần Tall" }),
    ).toHaveFocus();
  },
);

it("keeps the first-section upper boundary and cancels a valid unequal-height move", async () => {
  const { user, changed, container } = setup([tall, middle, last]);
  const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí phần Tall" });
  grip.focus();
  await user.keyboard(" ");
  await waitFor(() => expect(grip).toHaveAttribute("aria-pressed", "true"));
  await user.keyboard("{ArrowUp}");
  expect(changed).not.toHaveBeenCalled();
  expect(
    container.querySelector('[data-outline-section="tall"] [data-outline-drop]'),
  ).toBeInTheDocument();
  expect(container.querySelectorAll("[data-outline-drop]")).toHaveLength(1);
  await user.keyboard("{ArrowDown}");
  await waitFor(() =>
    expect(
      container.querySelector('[data-outline-section="middle"] [data-outline-drop]'),
    ).toBeInTheDocument(),
  );
  await user.keyboard("{Escape}");
  expect(changed).not.toHaveBeenCalled();
  expect(container.querySelector("[data-outline-drop]")).toBeNull();
  expect(grip).toHaveFocus();
});
