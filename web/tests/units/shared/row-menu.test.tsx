import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { RowMenu } from "@/components/shared/RowMenu";
import "@/lib/i18n";

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);

const trigger = (name = "Thao tác") => screen.getByRole("button", { name });

describe("the row menu's button", () => {
  it("is named Actions unless the row names it", () => {
    const first = render(
      <RowMenu>
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    expect(trigger()).toHaveAttribute("aria-haspopup", "menu");
    first.unmount();

    render(
      <RowMenu label="Thao tác với Mid-term Reading Mock">
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    expect(trigger("Thao tác với Mid-term Reading Mock")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Thao tác" })).toBeNull();
  });

  it("is the deck's 30px square with a 16px icon", () => {
    render(
      <RowMenu>
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    expect(classes(trigger())).toEqual(
      expect.arrayContaining([
        "rounded-seg",
        "hover:bg-hover",
        "dark:hover:bg-hover",
        "size-7.5",
        "[&_svg:not([class*='size-'])]:size-4",
        "data-[state=open]:bg-accent",
      ]),
    );
    for (const replaced of [
      "size-6",
      "size-7",
      "rounded-sm",
      "hover:bg-accent",
      "dark:hover:bg-accent/50",
      "[&_svg:not([class*='size-'])]:size-3.5",
    ])
      expect(classes(trigger())).not.toContain(replaced);
    expect(trigger().querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });
});

describe("the row menu's menu", () => {
  it("is headed by its title when it has one", async () => {
    const user = userEvent.setup();
    const titled = render(
      <RowMenu title="Mid-term Reading Mock">
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    await user.click(trigger());
    const menu = screen.getByRole("menu");
    expect(menu.firstElementChild).toHaveTextContent("Mid-term Reading Mock");
    expect(menu.firstElementChild).toHaveAttribute("data-slot", "dropdown-menu-label");
    titled.unmount();

    render(
      <RowMenu>
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    await user.click(trigger());
    expect(screen.getByRole("menu").firstElementChild).toHaveAttribute(
      "role",
      "menuitem",
    );
  });

  it("is 220px wide, aligned to the button's end", async () => {
    const user = userEvent.setup();
    render(
      <RowMenu>
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    await user.click(trigger());
    const menu = screen.getByRole("menu");
    expect(classes(menu).filter((c) => c.startsWith("w-"))).toEqual(["w-55"]);
    expect(menu).toHaveAttribute("data-align", "end");
  });

  it("takes another width from the caller", async () => {
    const user = userEvent.setup();
    render(
      <RowMenu className="w-60">
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    await user.click(trigger());
    expect(classes(screen.getByRole("menu")).filter((c) => c.startsWith("w-"))).toEqual(
      ["w-60"],
    );
  });
});

function Row({
  onOpen,
  onPress = () => {},
  onKey = () => {},
  onDuplicate = () => {},
}: Readonly<{
  onOpen: () => void;
  onPress?: () => void;
  onKey?: (key: string) => void;
  onDuplicate?: () => void;
}>) {
  return (
    <div
      role="button"
      tabIndex={0}
      aria-label="Mid-term Reading Mock"
      onClick={onOpen}
      onPointerDown={onPress}
      onKeyDown={(event) => onKey(event.key)}
    >
      <RowMenu title="Mid-term Reading Mock">
        <DropdownMenuItem onSelect={onDuplicate}>Nhân bản</DropdownMenuItem>
      </RowMenu>
    </div>
  );
}

describe("a row menu inside a row that opens on click", () => {
  it("opens the row when the row itself is clicked", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    render(<Row onOpen={onOpen} />);
    await user.click(screen.getByRole("button", { name: "Mid-term Reading Mock" }));
    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it("keeps the row shut when the menu's button is clicked", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    render(<Row onOpen={onOpen} />);
    await user.click(trigger());
    expect(screen.getByRole("menu")).toBeInTheDocument();
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("keeps the row shut when an item, the title or the menu's edge is clicked", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onDuplicate = vi.fn();
    render(<Row onOpen={onOpen} onDuplicate={onDuplicate} />);
    await user.click(trigger());
    const menu = screen.getByRole("menu");
    await user.click(within(menu).getByText("Mid-term Reading Mock"));
    await user.click(menu);
    expect(onOpen).not.toHaveBeenCalled();

    await user.click(within(menu).getByRole("menuitem", { name: "Nhân bản" }));
    expect(onDuplicate).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("keeps the row shut when an item is chosen with Enter", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onDuplicate = vi.fn();
    render(<Row onOpen={onOpen} onDuplicate={onDuplicate} />);
    trigger().focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("menu")).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Nhân bản" })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onDuplicate).toHaveBeenCalledTimes(1);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("stops the click only: a press and a key on the button still reach the row", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onPress = vi.fn();
    const onKey = vi.fn();
    render(<Row onOpen={onOpen} onPress={onPress} onKey={onKey} />);
    await user.click(trigger());
    expect(onPress).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("menu")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).toBeNull();

    trigger().focus();
    await user.keyboard("{ArrowDown}");
    expect(onKey).toHaveBeenCalledWith("ArrowDown");
    expect(screen.getByRole("menu")).toBeInTheDocument();
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("lets a press and a key inside the menu reach the row too", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onPress = vi.fn();
    const onKey = vi.fn();
    render(<Row onOpen={onOpen} onPress={onPress} onKey={onKey} />);
    await user.click(trigger());
    onPress.mockClear();
    onKey.mockClear();
    await user.keyboard("{ArrowDown}");
    expect(onKey).toHaveBeenCalledWith("ArrowDown");
    await user.click(screen.getByRole("menuitem", { name: "Nhân bản" }));
    expect(onPress).toHaveBeenCalledTimes(1);
    expect(onOpen).not.toHaveBeenCalled();
  });
});
