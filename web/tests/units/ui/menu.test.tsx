import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { ArrowUpRight } from "lucide-react";
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

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);
const lacks = (element: Element, replaced: readonly string[]) => {
  for (const c of replaced) expect(classes(element), c).not.toContain(c);
};

const CONTENT = [
  "text-popover-foreground",
  "min-w-40",
  "overflow-hidden",
  "border",
  "data-[state=closed]:animate-out",
  "data-[state=closed]:fade-out-0",
  "data-[state=open]:animate-in",
  "data-[state=open]:fade-in-0",
  "bg-card",
  "shadow-float",
  "z-(--z-popover)",
  "max-h-[min(60vh,var(--radix-dropdown-menu-content-available-height))]",
  "overflow-y-auto",
  "rounded-lg",
  "p-1.25",
];
const REPLACED_CONTENT = ["bg-popover", "z-50", "rounded-md", "p-1", "shadow-md"];
const ITEM = [
  "focus:bg-accent",
  "focus:text-accent-foreground",
  "relative",
  "flex",
  "cursor-default",
  "items-center",
  "rounded-sm",
  "px-2",
  "text-sm",
  "outline-none",
  "select-none",
  "data-[disabled]:pointer-events-none",
  "[&_svg]:shrink-0",
  "gap-2.25",
  "py-1.75",
  "leading-4",
  "data-[disabled]:opacity-45",
  "[&_svg:not([class*='size-'])]:size-3.5",
  "data-[variant=destructive]:text-danger-ink",
  "data-[variant=destructive]:focus:bg-hover",
  "data-[variant=destructive]:focus:text-danger-ink",
];
const REPLACED_ITEM = [
  "gap-2",
  "py-1.5",
  "data-[disabled]:opacity-50",
  "data-[variant=destructive]:text-destructive-ink",
  "data-[variant=destructive]:focus:bg-destructive/10",
  "[&_svg:not([class*='size-'])]:size-4",
];
const CHECK_ITEM = [
  "focus:bg-accent",
  "focus:text-accent-foreground",
  "relative",
  "flex",
  "cursor-default",
  "items-center",
  "rounded-sm",
  "pr-2",
  "text-sm",
  "outline-none",
  "select-none",
  "data-[disabled]:pointer-events-none",
  "gap-2.25",
  "py-1.75",
  "pl-2",
  "leading-4",
  "data-[disabled]:opacity-45",
];
const REPLACED_CHECK_ITEM = ["gap-2", "py-1.5", "pl-7", "data-[disabled]:opacity-50"];
const CHECK_BOX = [
  "left-2",
  "flex",
  "items-center",
  "justify-center",
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
];
const SEPARATOR = ["bg-border", "h-px", "mx-0", "my-0"];

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

describe("the menu's surface", () => {
  it("is the deck's container, and sets no width", () => {
    render(<Parts />);
    const menu = screen.getByRole("menu");
    expect(classes(menu)).toEqual(CONTENT);
    lacks(menu, REPLACED_CONTENT);
    expect(classes(menu).filter((c) => c.startsWith("w-"))).toEqual([]);
  });

  it("keeps 8px clear of the window's edges on each side", async () => {
    render(<Parts />);
    const wrapper = screen.getByRole("menu").parentElement!;
    await waitFor(() =>
      expect(wrapper.style.getPropertyValue("--radix-popper-available-width")).toBe(
        "-16px",
      ),
    );
  });
});

describe("the menu's rows", () => {
  it("are the deck's rows, and the separator is as wide as they are", () => {
    render(<Parts />);
    const menu = screen.getByRole("menu");

    const item = within(menu).getByRole("menuitem", { name: /Mở/ });
    expect(classes(item)).toEqual(ITEM);
    lacks(item, REPLACED_ITEM);

    const separator = within(menu).getByRole("separator");
    expect(classes(separator)).toEqual(SEPARATOR);
    lacks(separator, ["-mx-1", "my-1"]);
  });

  it("draws the title row in the deck's 11.5px muted line", () => {
    render(<Parts />);
    const label = screen.getByText("Lọc theo lớp");
    expect(label).toHaveAttribute("data-slot", "dropdown-menu-label");
    expect(classes(label)).toEqual([
      "px-2",
      "py-1.5",
      "text-muted-fg",
      "text-caption",
      "pt-1",
      "pb-1.5",
      "leading-normal",
    ]);
    lacks(label, ["text-muted-foreground", "text-xs"]);
    expect(screen.queryByRole("menuitem", { name: "Lọc theo lớp" })).toBeNull();
  });

  it("puts the meta at the row's end, muted and tabular, and lets the label truncate", () => {
    render(<Parts />);
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
    render(<Parts />);
    const danger = screen.getByRole("menuitem", { name: "Đóng sớm" });
    expect(danger).toHaveAttribute("data-variant", "destructive");
    expect(classes(danger)).toEqual(
      expect.arrayContaining([
        "data-[variant=destructive]:text-danger-ink",
        "data-[variant=destructive]:focus:text-danger-ink",
        "data-[variant=destructive]:focus:bg-hover",
      ]),
    );
    lacks(danger, ["data-[variant=destructive]:text-destructive-ink"]);

    const disabled = screen.getByRole("menuitem", { name: "Gia hạn" });
    expect(disabled).toHaveAttribute("aria-disabled", "true");
    expect(disabled).toHaveAttribute("data-disabled");
    expect(classes(disabled)).toContain("data-[disabled]:opacity-45");
    expect(classes(disabled)).not.toContain("data-[disabled]:opacity-50");
  });

  it("draws a check row's box, ticked only when the row is on", () => {
    render(<Parts />);
    const on = screen.getByRole("menuitemcheckbox", { name: "IELTS 6.5 Tối" });
    const off = screen.getByRole("menuitemcheckbox", { name: "TOEIC 600" });
    expect(on).toHaveAttribute("data-state", "checked");
    expect(off).toHaveAttribute("data-state", "unchecked");

    for (const row of [on, off]) {
      expect(classes(row)).toEqual(CHECK_ITEM);
      lacks(row, REPLACED_CHECK_ITEM);
      const box = row.firstElementChild!;
      expect(classes(box)).toEqual(CHECK_BOX);
      lacks(box, ["absolute", "size-3.5"]);
    }

    const tick = on.querySelector("svg")!;
    expect(classes(tick)).toContain("size-[0.6875rem]");
    expect(classes(tick)).not.toContain("size-3.5");
    expect(tick).toHaveAttribute("aria-hidden", "true");
    expect(off.querySelector("svg")).toBeNull();
  });
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
    render(<Actions />);
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
    render(<Actions />);
    trigger().focus();
    await user.keyboard("{Enter}");
    await user.keyboard("{End}");
    expect(row("Đóng sớm")).toHaveFocus();
    await user.keyboard("{Home}");
    expect(row("Mở")).toHaveFocus();
  });

  it("finds a row by its first letters", async () => {
    const user = userEvent.setup();
    render(<Actions />);
    trigger().focus();
    await user.keyboard("{Enter}");
    await user.keyboard("nh");
    await waitFor(() => expect(row("Nhân bản")).toHaveFocus());
  });

  it("closes on Escape and gives focus back to its button", async () => {
    const user = userEvent.setup();
    render(<Actions />);
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
    render(<Actions onOpen={onOpen} />);
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
    render(<Actions onExtend={onExtend} />);
    await user.click(trigger());
    await user.click(row("Gia hạn"));
    expect(onExtend).not.toHaveBeenCalled();
    expect(screen.getByRole("menu")).toBeInTheDocument();
  });

  it("toggles a check row and stays open", async () => {
    const user = userEvent.setup();
    const onClass = vi.fn();
    render(<Actions onClass={onClass} />);
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
        <AccountMenu settingsTo="/app/settings" />
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

  it("restates its width, radius and padding over the menu's", async () => {
    const user = renderStudentMenu();
    await user.click(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    );
    const menu = screen.getByRole("menu");
    const own = classes(menu);
    expect(own).toEqual(
      expect.arrayContaining([
        "bg-card",
        "shadow-float",
        "z-(--z-popover)",
        "max-h-[min(60vh,var(--radix-dropdown-menu-content-available-height))]",
        "overflow-y-auto",
      ]),
    );
    expect(own.filter(startsWithAny(["w-"]))).toEqual(["w-60"]);
    expect(own.filter(startsWithAny(["rounded-"]))).toEqual(["rounded-lg"]);
    expect(own.filter(startsWithAny(["p-", "px-", "py-"]))).toEqual(["p-1.5"]);
    expect(own).not.toContain("p-1.25");
  });

  it("restates each row's gap, padding, line height, icon size and disabled opacity", async () => {
    const user = renderStudentMenu();
    await user.click(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    );
    for (const name of ["Cài đặt", "Chế độ tối", "Đăng xuất"]) {
      const own = classes(row(name));
      expect(own.filter(startsWithAny(["gap-"]))).toEqual(["gap-2.5"]);
      expect(own.filter(startsWithAny(["p-", "px-", "py-"]))).toEqual(["p-2"]);
      expect(own.filter(startsWithAny(["leading-"]))).toEqual(["leading-4"]);
      expect(own.filter(startsWithAny(["h-", "min-h-"]))).toEqual([]);
      expect(own.filter(startsWithAny(["data-[disabled]:opacity-"]))).toEqual([
        "data-[disabled]:opacity-50",
      ]);
      expect(own.filter(startsWithAny(["[&_svg:not"]))).toEqual([
        "[&_svg:not([class*='size-'])]:size-[0.9375rem]",
      ]);
      expect(own).toContain("text-sm");
      expect(own).toContain("focus:bg-hover");
      expect(own).not.toContain("focus:bg-accent");
    }
  });

  it("keeps Sign out in the danger ink and the header's line", async () => {
    const user = renderStudentMenu();
    await user.click(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    );
    const signOut = row("Đăng xuất");
    expect(signOut).toHaveAttribute("data-variant", "default");
    expect(classes(signOut)).toEqual(
      expect.arrayContaining(["text-danger-ink", "focus:text-danger-ink", "border-t"]),
    );
    expect(classes(signOut)).not.toContain("focus:text-fg");
    expect(classes(signOut)).not.toContain("focus:text-accent-foreground");
    expect(classes(row("Cài đặt"))).toContain("focus:text-fg");

    const header = screen.getByRole("menu").firstElementChild!;
    expect(header.getAttribute("class")).toBe(
      "mb-1 flex items-center gap-2.5 border-b px-2 pt-1.5 pb-2.5",
    );
    expect(
      within(header as HTMLElement).getByText("Nguyễn Văn An"),
    ).toBeInTheDocument();
  });
});
