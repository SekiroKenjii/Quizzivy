import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { ArrowUpRight } from "lucide-react";
import type { ReactNode } from "react";
import { DeckScale } from "@/components/ui/deck-scale";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemText,
  DropdownMenuLabel,
  DropdownMenuMeta,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { useAuthStore } from "@/stores/auth";
import { STUDENT } from "../student/support";
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
const TODAY_ITEM = [
  "focus:bg-accent",
  "focus:text-accent-foreground",
  "relative",
  "flex",
  "cursor-default",
  "items-center",
  "gap-2",
  "rounded-sm",
  "px-2",
  "py-1.5",
  "text-sm",
  "outline-none",
  "select-none",
  "data-[disabled]:pointer-events-none",
  "data-[disabled]:opacity-50",
  "data-[variant=destructive]:text-destructive-ink",
  "data-[variant=destructive]:focus:bg-destructive/10",
  "[&_svg]:shrink-0",
  "[&_svg:not([class*='size-'])]:size-4",
];
const TODAY_CHECK_ITEM = [
  "focus:bg-accent",
  "focus:text-accent-foreground",
  "relative",
  "flex",
  "cursor-default",
  "items-center",
  "gap-2",
  "rounded-sm",
  "py-1.5",
  "pr-2",
  "pl-7",
  "text-sm",
  "outline-none",
  "select-none",
  "data-[disabled]:pointer-events-none",
  "data-[disabled]:opacity-50",
];
const TODAY_CHECK_BOX = [
  "absolute",
  "left-2",
  "flex",
  "size-3.5",
  "items-center",
  "justify-center",
];
const TODAY_SEPARATOR = ["bg-border", "-mx-1", "my-1", "h-px"];

function Parts() {
  return (
    <DropdownMenu open>
      <DropdownMenuTrigger>Lọc</DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuLabel>Lọc theo lớp</DropdownMenuLabel>
        <DropdownMenuItem>
          <ArrowUpRight aria-hidden="true" />
          <DropdownMenuItemText>Mở</DropdownMenuItemText>
          <DropdownMenuMeta>24</DropdownMenuMeta>
        </DropdownMenuItem>
        <DropdownMenuItem disabled>Gia hạn</DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive">Đóng sớm</DropdownMenuItem>
        <DropdownMenuCheckboxItem checked>IELTS 6.5 Tối</DropdownMenuCheckboxItem>
        <DropdownMenuCheckboxItem checked={false}>TOEIC 600</DropdownMenuCheckboxItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

const onDeck = (node: ReactNode) => render(<DeckScale>{node}</DeckScale>);

describe("the menu's surface", () => {
  it("is today's off a deck surface", () => {
    render(<Parts />);
    const menu = screen.getByRole("menu");
    expect(menu).not.toHaveAttribute("data-scale");
    expect(plain(menu)).toEqual(TODAY_CONTENT);
  });

  it("is the deck's container on a deck surface, and sets no width", () => {
    onDeck(<Parts />);
    const menu = screen.getByRole("menu");
    expect(menu).toHaveAttribute("data-scale", "deck");
    expect(plain(menu)).toEqual(TODAY_CONTENT);
    expect(own(menu)).toEqual([
      "bg-card",
      "shadow-float",
      "z-(--z-popover)",
      "max-h-[min(60vh,var(--radix-dropdown-menu-content-available-height))]",
      "overflow-y-auto",
      "rounded-lg",
      "p-1.25",
    ]);
  });

  it.each([
    ["none off a deck surface", false, "0px"] as const,
    ["8px on each side on one", true, "-16px"] as const,
  ])("keeps clear of the window's edges: %s", async (_, deck, room) => {
    if (deck) onDeck(<Parts />);
    else render(<Parts />);
    const wrapper = screen.getByRole("menu").parentElement!;
    await waitFor(() =>
      expect(wrapper.style.getPropertyValue("--radix-popper-available-width")).toBe(
        room,
      ),
    );
  });

  it("writes every deck class of the portalled element for the element itself", () => {
    onDeck(<Parts />);
    const menu = screen.getByRole("menu");
    const deck = classes(menu).filter((c) => c.includes("scale=deck"));
    expect(deck.length).toBeGreaterThan(0);
    expect(deck.filter((c) => !c.startsWith(OWN))).toEqual([]);
    expect(classes(menu).filter((c) => c.includes(INSIDE))).toEqual([]);
  });
});

describe("the menu's rows", () => {
  it.each([["off a deck surface", false] as const, ["on one", true] as const])(
    "keep today's classes and carry the deck's beside them, %s",
    (_, deck) => {
      if (deck) onDeck(<Parts />);
      else render(<Parts />);
      const menu = screen.getByRole("menu");

      const item = within(menu).getByRole("menuitem", { name: /Mở/ });
      expect(plain(item)).toEqual(TODAY_ITEM);
      expect(inside(item)).toEqual([
        "gap-2.25",
        "py-1.75",
        "leading-4",
        "data-[disabled]:opacity-45",
        "[&_svg:not([class*='size-'])]:size-3.5",
        "data-[variant=destructive]:text-danger-ink",
        "data-[variant=destructive]:focus:bg-hover",
        "data-[variant=destructive]:focus:text-danger-ink",
      ]);
      expect(classes(item).filter((c) => c.startsWith(OWN))).toEqual([]);

      const separator = within(menu).getByRole("separator");
      expect(plain(separator)).toEqual(TODAY_SEPARATOR);
      expect(inside(separator)).toEqual(["mx-0", "my-0"]);
    },
  );

  it("draws the title row in the deck's 11.5px muted line", () => {
    onDeck(<Parts />);
    const label = screen.getByText("Lọc theo lớp");
    expect(label).toHaveAttribute("data-slot", "dropdown-menu-label");
    expect(plain(label)).toEqual([
      "text-muted-foreground",
      "px-2",
      "py-1.5",
      "text-xs",
    ]);
    expect(inside(label)).toEqual([
      "text-muted-fg",
      "text-caption",
      "pt-1",
      "pb-1.5",
      "leading-normal",
    ]);
    expect(screen.queryByRole("menuitem", { name: "Lọc theo lớp" })).toBeNull();
  });

  it("puts the meta at the row's end, muted and tabular, and lets the label truncate", () => {
    onDeck(<Parts />);
    const item = screen.getByRole("menuitem", { name: /Mở/ });
    const meta = within(item).getByText("24");
    expect(meta).toHaveAttribute("data-slot", "dropdown-menu-meta");
    expect(classes(meta)).toEqual([
      "text-muted-fg",
      "ml-auto",
      "flex-none",
      "text-xs",
      "leading-[0.9375rem]",
      "tabular-nums",
    ]);
    expect(item.lastElementChild).toBe(meta);
    expect(classes(within(item).getByText("Mở"))).toEqual([
      "min-w-0",
      "flex-1",
      "truncate",
    ]);
    expect(item.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  it("marks a destructive row and a disabled row for the deck's ink and opacity", () => {
    onDeck(<Parts />);
    const danger = screen.getByRole("menuitem", { name: "Đóng sớm" });
    expect(danger).toHaveAttribute("data-variant", "destructive");
    expect(plain(danger)).toContain("data-[variant=destructive]:text-destructive-ink");
    expect(inside(danger)).toEqual(
      expect.arrayContaining([
        "data-[variant=destructive]:text-danger-ink",
        "data-[variant=destructive]:focus:text-danger-ink",
        "data-[variant=destructive]:focus:bg-hover",
      ]),
    );

    const disabled = screen.getByRole("menuitem", { name: "Gia hạn" });
    expect(disabled).toHaveAttribute("aria-disabled", "true");
    expect(disabled).toHaveAttribute("data-disabled");
    expect(plain(disabled)).toContain("data-[disabled]:opacity-50");
    expect(inside(disabled)).toContain("data-[disabled]:opacity-45");
    expect(inside(disabled)).not.toContain("data-[disabled]:opacity-50");
  });

  it.each([["off a deck surface", false] as const, ["on one", true] as const])(
    "draws a check row's box, ticked only when the row is on, %s",
    (_, deck) => {
      if (deck) onDeck(<Parts />);
      else render(<Parts />);
      const on = screen.getByRole("menuitemcheckbox", { name: "IELTS 6.5 Tối" });
      const off = screen.getByRole("menuitemcheckbox", { name: "TOEIC 600" });
      expect(on).toHaveAttribute("data-state", "checked");
      expect(off).toHaveAttribute("data-state", "unchecked");

      for (const row of [on, off]) {
        expect(plain(row)).toEqual(TODAY_CHECK_ITEM);
        expect(inside(row)).toEqual([
          "gap-2.25",
          "py-1.75",
          "pl-2",
          "leading-4",
          "data-[disabled]:opacity-45",
        ]);
        const box = row.firstElementChild!;
        expect(plain(box)).toEqual(TODAY_CHECK_BOX);
        expect(inside(box)).toEqual([
          "border-ring",
          "bg-card",
          "text-primary-fg",
          "static",
          "size-4",
          "flex-none",
          "rounded-[0.25rem]",
          "border",
          "in-data-[state=checked]:border-primary",
          "in-data-[state=checked]:bg-primary",
        ]);
      }

      const tick = on.querySelector("svg")!;
      expect(classes(tick)).toEqual(
        expect.arrayContaining(["size-3.5", "in-data-[scale=deck]:size-[0.6875rem]"]),
      );
      expect(tick).toHaveAttribute("aria-hidden", "true");
      expect(off.querySelector("svg")).toBeNull();
    },
  );
});

function Actions({
  onOpen = () => {},
  onExtend = () => {},
  onClass = () => {},
}: Readonly<{
  onOpen?: () => void;
  onExtend?: () => void;
  onClass?: (checked: boolean) => void;
}>) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger>Thao tác khác</DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuLabel>Bài giao</DropdownMenuLabel>
        <DropdownMenuItem onSelect={onOpen}>Mở</DropdownMenuItem>
        <DropdownMenuItem>Sửa cài đặt</DropdownMenuItem>
        <DropdownMenuItem disabled onSelect={onExtend}>
          Gia hạn
        </DropdownMenuItem>
        <DropdownMenuItem>Nhân bản</DropdownMenuItem>
        <DropdownMenuCheckboxItem checked={false} onCheckedChange={onClass}>
          Lớp tối
        </DropdownMenuCheckboxItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive">Đóng sớm</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

const trigger = () => screen.getByRole("button", { name: "Thao tác khác" });
const row = (name: string) => screen.getByRole("menuitem", { name });

describe("the menu from the keyboard", () => {
  it("moves with the arrow keys, passing the title and the disabled row", async () => {
    const user = userEvent.setup();
    onDeck(<Actions />);
    trigger().focus();
    await user.keyboard("{Enter}");

    expect(row("Mở")).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(row("Sửa cài đặt")).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(row("Nhân bản")).toHaveFocus();
    await user.keyboard("{ArrowUp}");
    expect(row("Sửa cài đặt")).toHaveFocus();
  });

  it("jumps to the last row on End and the first on Home", async () => {
    const user = userEvent.setup();
    onDeck(<Actions />);
    trigger().focus();
    await user.keyboard("{Enter}");
    await user.keyboard("{End}");
    expect(row("Đóng sớm")).toHaveFocus();
    await user.keyboard("{Home}");
    expect(row("Mở")).toHaveFocus();
  });

  it("finds a row by its first letters", async () => {
    const user = userEvent.setup();
    onDeck(<Actions />);
    trigger().focus();
    await user.keyboard("{Enter}");
    await user.keyboard("nh");
    await waitFor(() => expect(row("Nhân bản")).toHaveFocus());
  });

  it("closes on Escape and gives focus back to its button", async () => {
    const user = userEvent.setup();
    onDeck(<Actions />);
    trigger().focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("menu")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).toBeNull();
    expect(trigger()).toHaveFocus();
  });

  it("runs a row on Enter and closes", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    onDeck(<Actions onOpen={onOpen} />);
    trigger().focus();
    await user.keyboard("{Enter}");
    await user.keyboard("{Enter}");
    expect(onOpen).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menu")).toBeNull();
  });
});

describe("rows that do not close the menu", () => {
  it("does nothing for a disabled row", async () => {
    const user = userEvent.setup();
    const onExtend = vi.fn();
    onDeck(<Actions onExtend={onExtend} />);
    await user.click(trigger());
    await user.click(row("Gia hạn"));
    expect(onExtend).not.toHaveBeenCalled();
    expect(screen.getByRole("menu")).toBeInTheDocument();
  });

  it("toggles a check row and stays open", async () => {
    const user = userEvent.setup();
    const onClass = vi.fn();
    onDeck(<Actions onClass={onClass} />);
    await user.click(trigger());
    await user.click(screen.getByRole("menuitemcheckbox", { name: "Lớp tối" }));
    expect(onClass).toHaveBeenCalledWith(true);
    expect(screen.getByRole("menu")).toBeInTheDocument();
  });
});

function renderStudentMenu() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <DeckScale>
          <AccountMenu settingsTo="/app/settings" deck />
        </DeckScale>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

const startsWithAny = (prefixes: string[]) => (c: string) =>
  prefixes.some((prefix) => c.startsWith(prefix));

describe("the student's account menu keeps its own geometry", () => {
  beforeEach(() => {
    useAuthStore.setState({ user: STUDENT });
  });

  it("restates its width, radius and padding for the element that carries the scale", async () => {
    const user = renderStudentMenu();
    await user.click(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    );
    const menu = screen.getByRole("menu");
    expect(menu).toHaveAttribute("data-scale", "deck");

    const deck = own(menu);
    expect(deck).toEqual([
      "max-h-[min(60vh,var(--radix-dropdown-menu-content-available-height))]",
      "overflow-y-auto",
      "bg-card",
      "shadow-float",
      "z-(--z-popover)",
      "w-60",
      "rounded-lg",
      "p-1.5",
    ]);
    expect(deck.filter(startsWithAny(["w-"]))).toEqual(["w-60"]);
    expect(deck.filter(startsWithAny(["rounded-"]))).toEqual(["rounded-lg"]);
    expect(deck.filter(startsWithAny(["p-", "px-", "py-"]))).toEqual(["p-1.5"]);
    expect(deck).not.toContain("p-1.25");
    expect(classes(menu).filter((c) => c.includes(INSIDE))).toEqual([]);
    expect(plain(menu)).toEqual(TODAY_CONTENT);
  });

  it("restates each row's gap, padding, line height, icon size and disabled opacity", async () => {
    const user = renderStudentMenu();
    await user.click(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    );
    for (const name of ["Cài đặt", "Chế độ tối", "Đăng xuất"]) {
      const deck = inside(row(name));
      expect(deck.filter(startsWithAny(["gap-"]))).toEqual(["gap-2.5"]);
      expect(deck.filter(startsWithAny(["p-", "px-", "py-"]))).toEqual(["p-2"]);
      expect(deck.filter(startsWithAny(["leading-"]))).toEqual(["leading-4"]);
      expect(deck.filter(startsWithAny(["h-", "min-h-", "text-"]))).toEqual([]);
      expect(deck.filter(startsWithAny(["data-[disabled]:"]))).toEqual([
        "data-[disabled]:opacity-50",
      ]);
      expect(deck.filter(startsWithAny(["[&_svg"]))).toEqual([
        "[&_svg:not([class*='size-'])]:size-[0.9375rem]",
      ]);
      expect(plain(row(name)).filter(startsWithAny(["text-sm", "gap-"]))).toEqual([
        "gap-2",
        "text-sm",
      ]);
      expect(plain(row(name))).toContain("focus:bg-hover");
      expect(plain(row(name))).not.toContain("focus:bg-accent");
    }
  });

  it("keeps Sign out in the danger ink and the header's line", async () => {
    const user = renderStudentMenu();
    await user.click(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    );
    const signOut = row("Đăng xuất");
    expect(signOut).toHaveAttribute("data-variant", "default");
    expect(plain(signOut)).toEqual(
      expect.arrayContaining(["text-danger-ink", "focus:text-danger-ink", "border-t"]),
    );
    expect(plain(signOut)).not.toContain("focus:text-fg");
    expect(
      inside(signOut).filter(
        (c) => c.includes("text-") && !c.startsWith("data-[variant=destructive]:"),
      ),
    ).toEqual([]);
    expect(plain(row("Cài đặt"))).toContain("focus:text-fg");

    const header = screen.getByRole("menu").firstElementChild!;
    expect(header.getAttribute("class")).toBe(
      "mb-1 flex items-center gap-2.5 border-b px-2 pt-1.5 pb-2.5",
    );
    expect(
      within(header as HTMLElement).getByText("Nguyễn Văn An"),
    ).toBeInTheDocument();
  });
});
