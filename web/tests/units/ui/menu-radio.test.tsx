import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

function Ranges({ onChange }: Readonly<{ onChange?: (value: string) => void }>) {
  const [value, setValue] = useState("7d");
  return (
    <DeckScale>
      <DropdownMenu>
        <DropdownMenuTrigger>Khoảng thời gian</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuRadioGroup
            value={value}
            onValueChange={(next) => {
              setValue(next);
              onChange?.(next);
            }}
          >
            <DropdownMenuRadioItem value="today">Hôm nay</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="7d">7 ngày qua</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="30d" disabled>
              30 ngày qua
            </DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem>Khoảng tuỳ chọn…</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </DeckScale>
  );
}

const open = async () => {
  const user = userEvent.setup();
  render(<Ranges onChange={onChange} />);
  await user.click(screen.getByRole("button", { name: "Khoảng thời gian" }));
  return { user, menu: await screen.findByRole("menu") };
};

const onChange = vi.fn();

describe("a radio group in a menu", () => {
  it("is a group of radio items, the chosen one checked and the others not", async () => {
    const { menu } = await open();

    const items = within(menu).getAllByRole("menuitemradio");
    expect(items.map((item) => item.textContent)).toEqual([
      "Hôm nay",
      "7 ngày qua",
      "30 ngày qua",
    ]);
    expect(
      within(menu).getByRole("menuitemradio", { name: "7 ngày qua" }),
    ).toHaveAttribute("aria-checked", "true");
    expect(
      within(menu).getByRole("menuitemradio", { name: "Hôm nay" }),
    ).toHaveAttribute("aria-checked", "false");
  });

  it("draws the tick in the chosen row's box only, hidden from the name", async () => {
    const { menu } = await open();

    const on = within(menu).getByRole("menuitemradio", { name: "7 ngày qua" });
    const off = within(menu).getByRole("menuitemradio", { name: "Hôm nay" });
    expect(on.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    expect(off.querySelector("svg")).toBeNull();
    expect(on.firstElementChild?.className).toContain(
      "in-data-[scale=deck]:in-data-[state=checked]:bg-primary",
    );
  });

  it("chooses a row, tells the caller and closes the menu", async () => {
    onChange.mockClear();
    const { user, menu } = await open();

    await user.click(within(menu).getByRole("menuitemradio", { name: "Hôm nay" }));

    expect(onChange).toHaveBeenCalledExactlyOnceWith("today");
    expect(screen.queryByRole("menu")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Khoảng thời gian" }));
    expect(
      within(await screen.findByRole("menu")).getByRole("menuitemradio", {
        name: "Hôm nay",
      }),
    ).toHaveAttribute("aria-checked", "true");
  });

  it("moves with the arrow keys and chooses on Enter", async () => {
    onChange.mockClear();
    const { user } = await open();

    await user.keyboard("{ArrowDown}{Enter}");

    expect(onChange).toHaveBeenCalledOnce();
  });

  it("does not choose a disabled row", async () => {
    onChange.mockClear();
    const { user, menu } = await open();

    await user.click(within(menu).getByRole("menuitemradio", { name: "30 ngày qua" }));

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("menu")).toBeVisible();
  });
});
