import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const LONG = "Trưởng bộ môn tiếng Anh học thuật cho các lớp luyện thi IELTS buổi tối";

function RoleSelect() {
  return (
    <Select defaultValue="long">
      <SelectTrigger aria-label="Vai trò">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="short">Giáo viên</SelectItem>
        <SelectItem value="long">{LONG}</SelectItem>
      </SelectContent>
    </Select>
  );
}

describe("a select's list", () => {
  it("is never wider than the viewport less 2rem", async () => {
    render(<RoleSelect />);
    await userEvent.setup().click(screen.getByRole("combobox", { name: "Vai trò" }));
    expect(await screen.findByRole("listbox")).toHaveClass("max-w-[calc(100vw-2rem)]");
  });

  it("cuts a long label with an ellipsis and keeps the whole label as its name", async () => {
    render(<RoleSelect />);
    await userEvent.setup().click(screen.getByRole("combobox", { name: "Vai trò" }));
    const option = await screen.findByRole("option", { name: LONG });
    expect(option).toHaveClass("*:[span]:last:min-w-0");
    const label = [...option.querySelectorAll("span")].filter(
      (span) => span.textContent === LONG,
    );
    expect(label.at(-1)).toHaveClass("min-w-0", "truncate");
  });
});
