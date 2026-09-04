import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  OutlineTree,
  type OutlineQuestion,
} from "@/features/tests/components/OutlineTree";
import type { OutlineSection } from "@/features/tests/outline";
import "@/lib/i18n";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

function sections(): OutlineSection[] {
  return [
    { id: "s1", title: "Ngữ pháp", instructions: null, questionIds: ["q1", "q2"] },
    {
      id: "s2",
      title: "Nghe",
      instructions: "Nghe kỹ trước khi chọn.",
      questionIds: ["q3"],
    },
    { id: "s3", title: "Viết", instructions: null, questionIds: [] },
  ];
}

const questions = new Map<string, OutlineQuestion>([
  ["q1", { id: "q1", prompt: "Câu một", points: 1, hasAudio: false, problem: null }],
  ["q2", { id: "q2", prompt: "Câu hai", points: 2, hasAudio: false, problem: null }],
  ["q3", { id: "q3", prompt: "Câu ba", points: 2, hasAudio: true, problem: null }],
]);

function renderTree() {
  const onChange = vi.fn();
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

async function openMenu(user: ReturnType<typeof userEvent.setup>, index: number) {
  const triggers = screen.getAllByRole("button", { name: "Thao tác với phần" });
  await user.click(triggers[index]!);
  return screen.findByRole("menu");
}

const titles = (value: OutlineSection[]) => value.map((s) => s.title);

describe("the section menu", () => {
  it("carries A-04a's five items, in the deck's order", async () => {
    const { user } = renderTree();

    const menu = await openMenu(user, 1);

    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => (item.textContent ?? "").trim()),
    ).toEqual([
      "Đổi tên",
      "Hướng dẫn phần",
      "Di chuyển lên",
      "Di chuyển xuống",
      "Xoá phần",
    ]);
  });

  it("renames a section in place, committing on Enter", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 1);
    await user.click(await screen.findByRole("menuitem", { name: "Đổi tên" }));

    const field = await screen.findByLabelText("Tên phần");
    expect(field).toHaveFocus();
    await user.clear(field);
    await user.keyboard("Nghe hiểu{Enter}");

    expect(titles(onChange.mock.calls[0]![0] as OutlineSection[])).toEqual([
      "Ngữ pháp",
      "Nghe hiểu",
      "Viết",
    ]);
    expect(screen.getByText("Nghe hiểu")).toBeInTheDocument();
    expect(screen.queryByLabelText("Tên phần")).toBeNull();
  });

  it("throws the rename away on Escape", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 1);
    await user.click(await screen.findByRole("menuitem", { name: "Đổi tên" }));

    const field = await screen.findByLabelText("Tên phần");
    await user.clear(field);
    await user.keyboard("Nghe hiểu{Escape}");

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("Nghe")).toBeInTheDocument();
  });

  it("refuses to commit an empty name", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 1);
    await user.click(await screen.findByRole("menuitem", { name: "Đổi tên" }));

    await user.clear(await screen.findByLabelText("Tên phần"));
    await user.keyboard("{Enter}");

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("Nghe")).toBeInTheDocument();
  });

  it("cannot move the first section up, and moves the second", async () => {
    const { user, onChange } = renderTree();

    const first = await openMenu(user, 0);
    expect(
      within(first).getByRole("menuitem", { name: "Di chuyển lên" }),
    ).toHaveAttribute("aria-disabled", "true");
    await user.keyboard("{Escape}");

    await openMenu(user, 1);
    await user.click(await screen.findByRole("menuitem", { name: "Di chuyển lên" }));

    expect(titles(onChange.mock.calls[0]![0] as OutlineSection[])).toEqual([
      "Nghe",
      "Ngữ pháp",
      "Viết",
    ]);
  });

  it("takes the questions along when a section moves", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 0);
    await user.click(await screen.findByRole("menuitem", { name: "Di chuyển xuống" }));

    const next = onChange.mock.calls[0]![0] as OutlineSection[];
    expect(next.map((s) => s.questionIds)).toEqual([["q3"], ["q1", "q2"], []]);
  });

  it("cannot move the last section down", async () => {
    const { user } = renderTree();

    const last = await openMenu(user, 2);
    expect(
      within(last).getByRole("menuitem", { name: "Di chuyển xuống" }),
    ).toHaveAttribute("aria-disabled", "true");
  });

  it("removes an empty section without asking", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 2);
    await user.click(await screen.findByRole("menuitem", { name: "Xoá phần" }));

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(titles(onChange.mock.calls[0]![0] as OutlineSection[])).toEqual([
      "Ngữ pháp",
      "Nghe",
    ]);
    expect(screen.queryByText("Viết")).toBeNull();
  });

  it("asks first when the section still holds questions", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 0);
    await user.click(await screen.findByRole("menuitem", { name: "Xoá phần" }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/Ngữ pháp/)).toBeInTheDocument();
    expect(within(dialog).getByText(/2 câu/)).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();

    await user.click(within(dialog).getByRole("button", { name: "Xoá phần" }));

    expect(titles(onChange.mock.calls[0]![0] as OutlineSection[])).toEqual([
      "Nghe",
      "Viết",
    ]);
    expect(screen.queryByText("Ngữ pháp")).toBeNull();
  });

  it("keeps the section, and its questions, when the confirm is dismissed", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 0);
    await user.click(await screen.findByRole("menuitem", { name: "Xoá phần" }));

    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("Ngữ pháp")).toBeInTheDocument();
  });

  it("edits a section's instructions through the same outline change", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 1);
    await user.click(await screen.findByRole("menuitem", { name: "Hướng dẫn phần" }));

    const dialog = await screen.findByRole("dialog");
    const field = within(dialog).getByLabelText("Hướng dẫn phần");
    expect(field).toHaveValue("Nghe kỹ trước khi chọn.");

    await user.clear(field);
    await user.type(field, "Nghe hai lần rồi trả lời.");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    const next = onChange.mock.calls[0]![0] as OutlineSection[];
    expect(next[1]!.instructions).toBe("Nghe hai lần rồi trả lời.");
  });

  it("leaves a section with no instructions at all when the field is emptied", async () => {
    const { user, onChange } = renderTree();

    await openMenu(user, 1);
    await user.click(await screen.findByRole("menuitem", { name: "Hướng dẫn phần" }));

    const dialog = await screen.findByRole("dialog");
    await user.clear(within(dialog).getByLabelText("Hướng dẫn phần"));
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    const next = onChange.mock.calls[0]![0] as OutlineSection[];
    expect(next[1]!.instructions).toBeNull();
  });

  it("still collapses a section from its header, without changing the outline", async () => {
    const { user, onChange } = renderTree();

    const headers = screen.getAllByRole("button", { expanded: true });
    expect(headers).toHaveLength(3);
    expect(screen.getByText("Câu ba")).toBeInTheDocument();

    await user.click(headers[1]!);

    expect(screen.queryByText("Câu ba")).toBeNull();
    expect(screen.getByText("Nghe")).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });
});
