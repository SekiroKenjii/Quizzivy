import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { RowMenu } from "@/components/shared/RowMenu";
import "@/lib/i18n";

const INSIDE = "in-data-[scale=deck]:";
const OWN = "data-[scale=deck]:";

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);
const plain = (element: Element) => classes(element).filter((c) => !c.includes(OWN));
const inside = (element: Element) =>
  classes(element)
    .filter((c) => c.startsWith(INSIDE))
    .map((c) => c.slice(INSIDE.length));
const own = (element: Element) =>
  classes(element)
    .filter((c) => c.startsWith(OWN))
    .map((c) => c.slice(OWN.length));

const TODAY_TRIGGER = [
  "inline-flex",
  "shrink-0",
  "items-center",
  "justify-center",
  "gap-2",
  "text-sm",
  "font-medium",
  "whitespace-nowrap",
  "transition-all",
  "outline-none",
  "focus-visible:border-ring",
  "disabled:pointer-events-none",
  "disabled:opacity-50",
  "aria-invalid:border-destructive",
  "aria-invalid:ring-destructive/20",
  "dark:aria-invalid:ring-destructive/40",
  "[&_svg]:pointer-events-none",
  "[&_svg]:shrink-0",
  "hover:bg-accent",
  "hover:text-accent-foreground",
  "dark:hover:bg-accent/50",
  "size-7",
  "rounded-sm",
  "[&_svg:not([class*='size-'])]:size-3.5",
  "data-[state=open]:bg-accent",
];
const TODAY_CONTENT = [
  "bg-popover",
  "text-popover-foreground",
  "z-50",
  "min-w-40",
  "overflow-hidden",
  "rounded-md",
  "border",
  "p-1",
  "shadow-md",
  "data-[state=closed]:animate-out",
  "data-[state=closed]:fade-out-0",
  "data-[state=open]:animate-in",
  "data-[state=open]:fade-in-0",
];

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

  it("is today's ghost button off a deck surface", () => {
    render(
      <RowMenu>
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    expect(plain(trigger())).toEqual(TODAY_TRIGGER);
    expect(trigger().closest("[data-scale]")).toBeNull();
  });

  it("is the deck's 30px square with a 16px icon on a deck surface", () => {
    render(
      <DeckScale>
        <RowMenu>
          <DropdownMenuItem>Mở</DropdownMenuItem>
        </RowMenu>
      </DeckScale>,
    );
    expect(plain(trigger())).toEqual(TODAY_TRIGGER);
    expect(inside(trigger())).toEqual([
      "rounded-seg",
      "hover:bg-hover",
      "size-7.5",
      "[&_svg:not([class*='size-'])]:size-4",
    ]);
    expect(classes(trigger())).toContain("dark:in-data-[scale=deck]:hover:bg-hover");
    expect(classes(trigger()).filter((c) => c.startsWith(OWN))).toEqual([]);
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

  it("is today's width off a deck surface, or the width the caller gives", async () => {
    const user = userEvent.setup();
    const first = render(
      <RowMenu>
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    await user.click(trigger());
    expect(plain(screen.getByRole("menu"))).toEqual([...TODAY_CONTENT, "w-52"]);
    expect(screen.getByRole("menu")).toHaveAttribute("data-align", "end");
    first.unmount();

    render(
      <RowMenu className="w-60">
        <DropdownMenuItem>Mở</DropdownMenuItem>
      </RowMenu>,
    );
    await user.click(trigger());
    expect(plain(screen.getByRole("menu"))).toEqual([...TODAY_CONTENT, "w-60"]);
  });

  it("is 220px wide on a deck surface, written for the element that carries the scale", async () => {
    const user = userEvent.setup();
    render(
      <DeckScale>
        <RowMenu>
          <DropdownMenuItem>Mở</DropdownMenuItem>
        </RowMenu>
      </DeckScale>,
    );
    await user.click(trigger());
    const menu = screen.getByRole("menu");
    expect(menu).toHaveAttribute("data-scale", "deck");
    expect(own(menu).filter((c) => c.startsWith("w-"))).toEqual(["w-55"]);
    expect(classes(menu).filter((c) => c.includes(INSIDE))).toEqual([]);
  });

  it("takes another deck width from the caller", async () => {
    const user = userEvent.setup();
    render(
      <DeckScale>
        <RowMenu className="data-[scale=deck]:w-60">
          <DropdownMenuItem>Mở</DropdownMenuItem>
        </RowMenu>
      </DeckScale>,
    );
    await user.click(trigger());
    expect(own(screen.getByRole("menu")).filter((c) => c.startsWith("w-"))).toEqual([
      "w-60",
    ]);
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
});
