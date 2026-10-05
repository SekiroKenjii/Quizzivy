import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TagCombobox } from "@/components/shared/form/TagCombobox";
import type { TagSuggestion } from "@/components/shared/form/tagOptions";
import "@/lib/i18n";

const KNOWN: readonly TagSuggestion[] = [
  { tag: "grammar", meta: "2 câu hỏi" },
  { tag: "reading", meta: "2 câu hỏi" },
  { tag: "present perfect", meta: "1 câu hỏi" },
  { tag: "prepositions", meta: "1 câu hỏi" },
  { tag: "Nghé con" },
  { tag: "reported speech" },
];

function Tags({
  start = ["grammar", "present perfect"],
  onChange = () => {},
  placeholder,
  known = KNOWN,
}: Readonly<{
  start?: readonly string[];
  onChange?: (tags: string[]) => void;
  placeholder?: string;
  known?: readonly TagSuggestion[];
}>) {
  const [tags, setTags] = useState(start);
  return (
    <>
      <TagCombobox
        label="Thẻ"
        tags={tags}
        suggestions={known}
        placeholder={placeholder}
        onChange={(next) => {
          onChange(next);
          setTags(next);
        }}
      />
      <button type="button">Lưu câu hỏi</button>
    </>
  );
}

const input = () => screen.getByRole<HTMLInputElement>("combobox", { name: "Thẻ" });
const list = () => screen.getByRole("listbox", { name: "Gợi ý thẻ" });
const options = () => within(list()).getAllByRole("option");
const labels = () => options().map((option) => option.textContent);
const active = () =>
  document.getElementById(input().getAttribute("aria-activedescendant")!);
const chips = () =>
  [...document.querySelectorAll('[data-slot="chip"]')].map((chip) => chip.textContent);

describe("the tag field", () => {
  it("is a combobox with its name, its chosen tags and no list until something is typed", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    expect(input()).toHaveAttribute("aria-expanded", "false");
    expect(input()).toHaveAttribute("aria-autocomplete", "list");
    expect(input()).toHaveAttribute("placeholder", "Thêm thẻ…");
    expect(input()).not.toHaveAttribute("aria-activedescendant");
    expect(chips()).toEqual(["grammar", "present perfect"]);
    await user.click(input());
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input()).toHaveAttribute("aria-expanded", "false");
  });

  it("takes its placeholder from the caller when it is given one", () => {
    render(<Tags placeholder="Thêm chủ đề…" />);
    expect(input()).toHaveAttribute("placeholder", "Thêm chủ đề…");
  });

  it("opens a list of the matching suggestions and the row that creates the draft", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re");
    expect(input()).toHaveAttribute("aria-expanded", "true");
    expect(input()).toHaveAttribute("aria-controls", list().id);
    expect(labels()).toEqual([
      "reading2 câu hỏi",
      "reported speech",
      "prepositions1 câu hỏi",
      "Tạo “re”Thẻ mới",
    ]);
  });

  it("sets the matched part of each suggestion in bold, on the suggestion's own spelling", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re");
    const bold = options().map((option) => option.querySelector("b")?.textContent);
    expect(bold).toEqual(["re", "re", "re", "re"]);
    expect(options()[2]!.querySelector("b")!.previousSibling).toHaveTextContent("p");
    expect(options()[0]!.querySelector("b")).toHaveClass("font-semibold");

    await user.clear(input());
    await user.type(input(), "nghe");
    expect(options()[0]!.querySelector("b")).toHaveTextContent("Nghé");
    expect(options()[0]).toHaveTextContent("Nghé con");
  });

  it("explains its keys under the options, outside the listbox", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re");
    const keys = screen.getByText("↑ ↓ để di chuyển · Enter để thêm · Esc để đóng");
    expect(list()).not.toContainElement(keys);
    expect(list().parentElement).toContainElement(keys);
  });

  it("closes the list when nothing is left to list", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re");
    expect(list()).toBeInTheDocument();
    await user.clear(input());
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input()).toHaveAttribute("aria-expanded", "false");
  });
});

describe("the tag field's keys", () => {
  it("start on the first option and move with the arrows, wrapping at both ends", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re");
    expect(active()).toBe(options()[0]);
    expect(options()[0]).toHaveAttribute("aria-selected", "true");
    expect(options()[1]).toHaveAttribute("aria-selected", "false");

    await user.keyboard("{ArrowDown}");
    expect(active()).toBe(options()[1]);
    expect(options()[1]).toHaveClass("bg-hover");
    expect(options()[0]).not.toHaveClass("bg-hover");
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(active()).toBe(options()[3]);
    await user.keyboard("{ArrowDown}");
    expect(active()).toBe(options()[0]);
    await user.keyboard("{ArrowUp}");
    expect(active()).toBe(options()[3]);
    await user.keyboard("{ArrowUp}");
    expect(active()).toBe(options()[2]);
    expect(input()).toHaveValue("re");
  });

  it("go back to the first option when the draft changes", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re");
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(active()).toBe(options()[2]);
    await user.keyboard("p");
    expect(active()).toBe(options()[0]);
    expect(active()).toHaveTextContent("reported speech");
  });

  it("take the active option on Enter, in the suggestion's own spelling", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "REP{Enter}");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith([
      "grammar",
      "present perfect",
      "reported speech",
    ]);
    expect(input()).toHaveValue("");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input()).toHaveFocus();
  });

  it("take the option the arrows moved to", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "re{ArrowDown}{ArrowDown}{Enter}");
    expect(onChange).toHaveBeenCalledWith([
      "grammar",
      "present perfect",
      "prepositions",
    ]);
  });

  it("take the active option on Tab and keep the focus in the field", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "rea");
    await user.tab();
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "reading"]);
    expect(input()).toHaveFocus();
    expect(input()).toHaveValue("");

    await user.tab();
    expect(screen.getByRole("button", { name: "Bỏ thẻ grammar" })).not.toHaveFocus();
    expect(input()).not.toHaveFocus();
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("create the draft as a new tag from the creating row", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "unit 4");
    expect(labels()).toEqual(["Tạo “unit 4”Thẻ mới"]);
    expect(options()[0]!.querySelector("b")).toHaveTextContent("unit 4");
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "unit 4"]);
    expect(chips()).toEqual(["grammar", "present perfect", "unit 4"]);
  });

  it("close the list on Escape and keep the draft, which Enter then adds", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "re{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input()).toHaveAttribute("aria-expanded", "false");
    expect(input()).toHaveValue("re");
    expect(input()).toHaveFocus();
    expect(onChange).not.toHaveBeenCalled();

    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "re"]);
    expect(input()).toHaveValue("");
  });

  it("add a suggestion's spelling for a draft that equals it, with the list closed", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "nghe CON{Escape}{Enter}");
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "Nghé con"]);
  });

  it("open the list again when the teacher types after Escape", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    await user.keyboard("a");
    expect(labels()).toEqual(["reading2 câu hỏi", "Tạo “rea”Thẻ mới"]);
  });

  it("add what precedes a comma", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "READING,");
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "reading"]);
    expect(input()).toHaveValue("");
  });

  it("remove the last tag on Backspace in the empty input", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.click(input());
    await user.keyboard("{Backspace}");
    expect(onChange).toHaveBeenCalledWith(["grammar"]);
    await user.keyboard("re{Backspace}");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(input()).toHaveValue("r");
  });
});

describe("the tag field's mouse", () => {
  it("adds the option that is pressed and leaves the focus in the input", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "re");
    await user.click(options()[2]!);
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith([
      "grammar",
      "present perfect",
      "prepositions",
    ]);
    expect(input()).toHaveFocus();
    expect(input()).toHaveValue("");
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("takes the press itself, so the input never loses focus to the list", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "re");
    expect(fireEvent.mouseDown(options()[0]!)).toBe(false);
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "reading"]);
    expect(input()).toHaveFocus();
  });

  it("creates the draft from a press on the creating row", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "re");
    await user.click(screen.getByRole("option", { name: /Tạo “re”/ }));
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "re"]);
    expect(input()).toHaveFocus();
  });

  it("makes the option under the pointer the active one", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re");
    await user.hover(options()[1]!);
    expect(active()).toBe(options()[1]);
    await user.keyboard("{Enter}");
    expect(chips()).toEqual(["grammar", "present perfect", "reported speech"]);
  });

  it("removes a tag from its own button, named in Vietnamese", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: "Bỏ thẻ grammar" }));
    expect(onChange).toHaveBeenCalledWith(["present perfect"]);
    expect(input()).toHaveFocus();
  });
});

describe("the tag field when it loses focus", () => {
  it("adds the draft and closes the list", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "re");
    await user.click(screen.getByRole("button", { name: "Lưu câu hỏi" }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "re"]);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input()).toHaveValue("");
  });

  it("adds a suggestion's spelling for a draft that equals it", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags onChange={onChange} />);
    await user.type(input(), "READING");
    await user.click(screen.getByRole("button", { name: "Lưu câu hỏi" }));
    expect(onChange).toHaveBeenCalledWith(["grammar", "present perfect", "reading"]);
  });
});

describe("a tag that is already chosen", () => {
  it("is not added again when only its accents or its case differ", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Tags start={["nghé"]} known={[{ tag: "grammar" }]} onChange={onChange} />);
    await user.type(input(), "nghe");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input()).toHaveAttribute("aria-expanded", "false");
    await user.keyboard("{Enter}");
    expect(onChange).not.toHaveBeenCalled();
    expect(input()).toHaveValue("");

    await user.type(input(), "NGHÉ");
    await user.tab();
    expect(onChange).not.toHaveBeenCalled();
    expect(chips()).toEqual(["nghé"]);
  });

  it("is left out of the list", async () => {
    const user = userEvent.setup();
    render(<Tags start={["Reading"]} />);
    await user.type(input(), "re");
    expect(labels()).toEqual([
      "reported speech",
      "present perfect1 câu hỏi",
      "prepositions1 câu hỏi",
      "Tạo “re”Thẻ mới",
    ]);
  });
});

describe("the tag field's drawing", () => {
  it("is the framed chip field with neutral 22px chips and an 80px input", () => {
    render(<Tags />);
    const box = document.querySelector('[data-slot="chip-box"]')!;
    expect(box).toHaveClass("min-h-9", "rounded-md", "border", "border-input", "bg-bg");
    expect(document.querySelector('[data-slot="chip"]')).toHaveClass(
      "h-5.5",
      "text-xs",
      "bg-muted",
      "text-fg",
    );
    expect(input()).toHaveClass("h-5.5", "min-w-20", "flex-[1_1_80px]", "px-1");
    expect(input()).not.toHaveClass("flex-[1_1_140px]");
    expect(box.parentElement).toHaveClass("relative");
  });

  it("opens its list 4px under the field at the popover layer, in the page", async () => {
    const user = userEvent.setup();
    render(<Tags />);
    await user.type(input(), "re");
    const popup = list().parentElement!;
    expect(popup).toHaveClass(
      "absolute",
      "inset-x-0",
      "top-[calc(100%+4px)]",
      "z-(--z-popover)",
      "bg-card",
      "border",
      "rounded-lg",
      "shadow-float",
      "p-1",
    );
    expect(popup.parentElement).toContainElement(input());
    expect(list()).toHaveClass("flex", "flex-col", "gap-px");
    expect(options()[0]).toHaveClass(
      "min-h-8",
      "gap-2",
      "rounded-sm",
      "px-2",
      "py-1.5",
      "text-sm",
    );
    expect(options()[0]!.querySelector("svg")).toHaveClass("lucide-tag", "size-3.25");
    expect(options()[3]!.querySelector("svg")).toHaveClass("lucide-plus");
  });
});
