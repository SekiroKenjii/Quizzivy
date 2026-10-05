import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { Sheet } from "@/components/shared/Sheet";
import { NumberStepper } from "@/components/shared/form/NumberStepper";
import { TagCombobox } from "@/components/shared/form/TagCombobox";
import "@/lib/i18n";

function Editor({
  onOpenChange,
  onNumberChange,
  onTagsChange,
}: Readonly<{
  onOpenChange: (open: boolean) => void;
  onNumberChange: (value: number) => void;
  onTagsChange: (tags: string[]) => void;
}>) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState(45);
  const [tags, setTags] = useState<string[]>([]);
  return (
    <main tabIndex={-1}>
      <button type="button" onClick={() => setOpen(true)}>
        Sửa bài kiểm tra
      </button>
      <Sheet
        open={open}
        title="Sửa bài kiểm tra"
        onOpenChange={(next) => {
          onOpenChange(next);
          setOpen(next);
        }}
      >
        <label>
          Tên bài
          <input defaultValue="Bài đọc" />
        </label>
        <TagCombobox
          label="Thẻ"
          tags={tags}
          suggestions={[{ tag: "Reading" }]}
          onChange={(next) => {
            onTagsChange(next);
            setTags(next);
          }}
        />
        <NumberStepper
          label="Thời gian"
          value={value}
          min={1}
          max={600}
          step={5}
          onChange={(next) => {
            onNumberChange(next);
            setValue(next);
          }}
        />
      </Sheet>
    </main>
  );
}

function editor() {
  const calls = {
    onOpenChange: vi.fn(),
    onNumberChange: vi.fn(),
    onTagsChange: vi.fn(),
  };
  render(<Editor {...calls} />);
  return { user: userEvent.setup(), ...calls };
}

describe("local Escape inside a sheet", () => {
  it("closes tag suggestions first, retaining focus and drafts, then closes the sheet", async () => {
    const { user, onOpenChange, onTagsChange } = editor();
    const opener = screen.getByRole("button", { name: "Sửa bài kiểm tra" });
    await user.click(opener);
    const name = screen.getByRole("textbox", { name: "Tên bài" });
    await user.type(name, " chưa lưu");
    const input = screen.getByRole("combobox", { name: "Thẻ" });
    await user.type(input, "re");
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    await user.keyboard("{Escape}");
    expect(screen.getByRole("dialog", { name: "Sửa bài kiểm tra" })).toBeVisible();
    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input).toHaveAttribute("aria-expanded", "false");
    expect(input).toHaveFocus();
    expect(input).toHaveValue("re");
    expect(name).toHaveValue("Bài đọc chưa lưu");
    expect(onTagsChange).not.toHaveBeenCalled();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
    expect(opener).toHaveFocus();
  });

  it.each(["47", ""])(
    "restores the numeric draft %j before the next Escape closes the sheet",
    async (draft) => {
      const { user, onOpenChange, onNumberChange } = editor();
      const opener = screen.getByRole("button", { name: "Sửa bài kiểm tra" });
      await user.click(opener);
      const name = screen.getByRole("textbox", { name: "Tên bài" });
      await user.type(name, " chưa lưu");
      const input = screen.getByRole("spinbutton", { name: "Thời gian" });
      await user.clear(input);
      if (draft !== "") await user.type(input, draft);
      expect(input).toHaveValue(draft);

      await user.keyboard("{Escape}");
      expect(screen.getByRole("dialog", { name: "Sửa bài kiểm tra" })).toBeVisible();
      expect(onOpenChange).not.toHaveBeenCalled();
      expect(onNumberChange).not.toHaveBeenCalled();
      expect(input).toHaveValue("45");
      expect(input).toHaveFocus();
      expect(name).toHaveValue("Bài đọc chưa lưu");

      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
      expect(opener).toHaveFocus();
    },
  );

  it.each(["combobox", "spinbutton"] as const)(
    "closes the sheet immediately from an unchanged %s",
    async (role) => {
      const { user, onOpenChange } = editor();
      await user.click(screen.getByRole("button", { name: "Sửa bài kiểm tra" }));
      await user.click(screen.getByRole(role));
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
    },
  );
});
