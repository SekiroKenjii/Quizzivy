import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import {
  FacetFilter,
  type FacetFilterProps,
} from "@/components/shared/data/FacetFilter";
import i18n from "@/lib/i18n";

const OPTIONS = [
  { value: "a", label: "IELTS 6.5 Tối" },
  { value: "b", label: "IELTS Cơ bản A" },
  { value: "c", label: "TOEIC 600 Cuối tuần" },
];

function fixture(props: Partial<FacetFilterProps> = {}) {
  const onChange = vi.fn();
  const user = userEvent.setup({ pointerEventsCheck: 0 });
  function Controlled() {
    const [selected, setSelected] = useState<readonly string[]>(props.selected ?? []);
    return (
      <DeckScale>
        <FacetFilter
          label="Lớp"
          title="Lọc theo lớp"
          options={OPTIONS}
          {...props}
          selected={selected}
          onChange={(next) => {
            onChange(next);
            setSelected(next);
          }}
        />
      </DeckScale>
    );
  }
  const view = render(<Controlled />);
  return { ...view, user, onChange };
}

beforeEach(async () => {
  await i18n.changeLanguage("vi");
});

describe("FacetFilter", () => {
  it("names an unused filter without a count pill", () => {
    fixture();
    const trigger = screen.getByRole("button", { name: "Lớp" });
    expect(trigger).toHaveAttribute("aria-haspopup", "menu");
    expect(within(trigger).queryByText("0")).toBeNull();
    expect(trigger.querySelector('[aria-hidden="true"] span')).toBeNull();
  });

  it("counts known and unknown selections once in its accessible name", () => {
    fixture({ selected: ["a", "archived"] });
    const trigger = screen.getByRole("button", { name: "Lớp, đã chọn 2" });
    expect(trigger).toHaveAccessibleName("Lớp, đã chọn 2");
    const pill = within(trigger).getByText("2");
    expect(pill).toHaveAttribute("aria-hidden", "true");
    expect(pill).toHaveClass(
      "h-4.5",
      "min-w-4.5",
      "px-1.25",
      "text-2xs",
      "font-semibold",
      "tabular-nums",
    );
  });

  it("shows the title, ordered options and their controlled checks", async () => {
    const { user } = fixture({ selected: ["b"] });
    await user.click(screen.getByRole("button"));
    expect(screen.getByText("Lọc theo lớp")).toBeVisible();
    const rows = screen.getAllByRole("menuitemcheckbox");
    expect(rows.map((row) => row.textContent)).toEqual(
      OPTIONS.map((option) => option.label),
    );
    expect(rows[0]).toHaveAttribute("aria-checked", "false");
    expect(rows[1]).toHaveAttribute("aria-checked", "true");
    expect(rows[2]).toHaveAttribute("aria-checked", "false");
    expect(within(rows[0]!).getByText(OPTIONS[0]!.label)).toHaveClass("truncate");
  });

  it("appends a checked value while retaining unknown values and the open menu", async () => {
    const { user, onChange } = fixture({ selected: ["archived", "b"] });
    await user.click(screen.getByRole("button"));
    await user.click(screen.getByRole("menuitemcheckbox", { name: OPTIONS[0]!.label }));
    expect(onChange).toHaveBeenLastCalledWith(["archived", "b", "a"]);
    expect(screen.getByRole("menu")).toBeVisible();
    expect(
      screen.getByRole("menuitemcheckbox", { name: OPTIONS[0]!.label }),
    ).toHaveAttribute("aria-checked", "true");
  });

  it("unchecks a value without reordering the other selected values", async () => {
    const { user, onChange } = fixture({ selected: ["c", "a", "b", "archived"] });
    await user.click(screen.getByRole("button"));
    await user.click(screen.getByRole("menuitemcheckbox", { name: OPTIONS[0]!.label }));
    expect(onChange).toHaveBeenLastCalledWith(["c", "b", "archived"]);
    expect(screen.getByRole("menu")).toBeVisible();
    await user.click(screen.getByRole("menuitemcheckbox", { name: OPTIONS[0]!.label }));
    expect(onChange).toHaveBeenLastCalledWith(["c", "b", "archived", "a"]);
  });

  it("disables Clear with no selection and cannot invoke it", async () => {
    const { user, onChange } = fixture();
    await user.click(screen.getByRole("button"));
    const clear = screen.getByRole("menuitem", { name: "Bỏ lọc" });
    expect(clear).toHaveAttribute("aria-disabled", "true");
    await user.click(clear);
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("menu")).toBeVisible();
  });

  it("clears all values, closes the menu and returns focus", async () => {
    const { user, onChange } = fixture({ selected: ["a", "archived"] });
    const trigger = screen.getByRole("button");
    await user.click(trigger);
    await user.click(screen.getByRole("menuitem", { name: "Bỏ lọc" }));
    expect(onChange).toHaveBeenCalledWith([]);
    await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveAccessibleName("Lớp");
  });

  it("omits Clear and its separator for a nonclearable facet", async () => {
    const { user } = fixture({ clearable: false });
    await user.click(screen.getByRole("button"));
    expect(screen.queryByRole("menuitem")).toBeNull();
    expect(screen.queryByRole("separator")).toBeNull();
  });

  it("keeps an empty-options filter enabled and shows the localized empty line", async () => {
    const { user } = fixture({ options: [], title: "" });
    expect(screen.getByRole("button")).toBeEnabled();
    await user.click(screen.getByRole("button"));
    expect(screen.getByText("Không có lựa chọn nào")).toBeVisible();
    expect(screen.queryByRole("menuitemcheckbox")).toBeNull();
    expect(screen.queryByText("Lọc theo lớp")).toBeNull();
  });

  it("does not open a disabled filter", async () => {
    const { user } = fixture({ disabled: true });
    const trigger = screen.getByRole("button");
    expect(trigger).toBeDisabled();
    await user.click(trigger);
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("opens by Enter, ticks by ArrowDown and Space, then returns focus on Escape", async () => {
    const { user, onChange } = fixture();
    const trigger = screen.getByRole("button");
    trigger.focus();
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(screen.getAllByRole("menuitemcheckbox")[0]).toHaveFocus(),
    );
    await user.keyboard("{ArrowDown} ");
    expect(onChange).toHaveBeenCalledWith(["b"]);
    expect(screen.getByRole("menu")).toBeVisible();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
    expect(trigger).toHaveFocus();
  });

  it("uses a portalled deck width and end alignment", async () => {
    const { user } = fixture();
    await user.click(screen.getByRole("button"));
    expect(screen.getByRole("menu")).toHaveAttribute("data-scale", "deck");
    expect(screen.getByRole("menu")).toHaveClass("data-[scale=deck]:w-60");
    expect(screen.getByRole("menu")).toHaveAttribute("data-align", "end");
  });

  it("accepts a caller's deck width and start alignment", async () => {
    const { user } = fixture({
      menuClassName: "data-[scale=deck]:w-55",
      align: "start",
    });
    await user.click(screen.getByRole("button"));
    expect(screen.getByRole("menu")).toHaveClass("data-[scale=deck]:w-55");
    expect(screen.getByRole("menu")).not.toHaveClass("data-[scale=deck]:w-60");
    expect(screen.getByRole("menu")).toHaveAttribute("data-align", "start");
  });

  it("keeps the trigger dashed and 34px under deck scale without a fixed label width", () => {
    fixture();
    const trigger = screen.getByRole("button");
    expect(trigger.tagName).toBe("BUTTON");
    expect(trigger).toHaveClass(
      "border-dashed",
      "in-data-[scale=deck]:h-8.5",
      "px-3",
      "gap-1.5",
      "font-normal",
      "leading-4",
      "hover:bg-muted",
    );
    expect(trigger).toHaveAttribute("type", "button");
    expect(within(trigger).getByText("Lớp")).not.toHaveAttribute("style");
  });

  it("uses the English active, clear and empty copy when requested", async () => {
    await i18n.changeLanguage("en");
    const { user } = fixture({ selected: ["archived"], options: [] });
    await user.click(screen.getByRole("button", { name: "Lớp, 1 selected" }));
    expect(screen.getByText("Nothing to choose from")).toBeVisible();
    expect(screen.getByRole("menuitem", { name: "Clear filter" })).toBeVisible();
  });
});
