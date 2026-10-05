import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CircleStop, Clock, Tag } from "lucide-react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  BulkActions,
  BulkBarButton,
  type BulkAction,
} from "@/components/shared/BulkActions";
import { BulkSelectAll } from "@/components/shared/BulkSelection";
import { DeckScale } from "@/components/ui/deck-scale";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { ApiError } from "@/lib/api/errors";
import { viewport } from "@tests/support/viewport";
import i18n from "@/lib/i18n";

const items = [
  { id: "a", name: "First" },
  { id: "b", name: "Referenced" },
];

it("confirms the selection, retains failed items and retries only those items", async () => {
  const run = vi
    .fn<(item: (typeof items)[number]) => Promise<void>>()
    .mockResolvedValueOnce(undefined)
    .mockRejectedValueOnce(
      new ApiError({
        status: 409,
        code: "RESOURCE_REFERENCED",
        message: "Still assigned",
      }),
    )
    .mockResolvedValueOnce(undefined);
  function Harness() {
    const selection = useBulkSelection<(typeof items)[number]>();
    return (
      <>
        <button onClick={() => selection.selectPage(items, true)}>Select page</button>
        <BulkActions
          selected={[...selection.selected.values()]}
          name={(item) => item.name}
          actions={[{ label: "Delete selected", description: "Cannot undo", run }]}
          onRemoved={selection.remove}
          onClear={selection.clear}
          onSettled={() => Promise.resolve()}
        />
      </>
    );
  }
  render(<Harness />);
  const user = userEvent.setup();
  await user.click(screen.getByText("Select page"));
  await user.click(screen.getByText("Delete selected"));
  expect(run).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Xác nhận 2 mục" }));
  await screen.findByText("Referenced: Still assigned");
  expect(screen.getByText("Đã chọn 1 mục")).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "Thử lại 1 mục chưa thành công" }),
  );
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(run.mock.calls.map(([item]) => item.id)).toEqual(["a", "b", "b"]);
});

type Item = (typeof items)[number];

const BUTTON =
  "inline-flex shrink-0 items-center justify-center text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:border-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4";
const OUTLINE =
  "border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50 in-data-[scale=deck]:bg-card in-data-[scale=deck]:hover:bg-muted dark:in-data-[scale=deck]:border-border dark:in-data-[scale=deck]:bg-card dark:in-data-[scale=deck]:hover:bg-muted";
const GHOST = "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50";
const SM =
  "h-8 gap-1.5 rounded-md px-3 in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:text-sm in-data-[scale=deck]:[&_svg:not([class*='size-'])]:size-3.5";

const todayAction = (label: string) =>
  `<button data-slot="button" data-variant="outline" data-size="sm" class="${BUTTON} ${OUTLINE} ${SM}">${label}</button>`;
const TODAY_CLEAR = `<button data-slot="button" data-variant="ghost" data-size="sm" class="${BUTTON} ${GHOST} ${SM}">Bỏ chọn</button>`;
const todayBar = (count: string, inside: string) =>
  `<div class="bg-secondary flex flex-wrap items-center gap-2 rounded-md px-3 py-2"><span class="mr-auto text-sm font-medium">${count}</span>${inside}${TODAY_CLEAR}</div>`.replaceAll(
    "&",
    "&amp;",
  );

const MIXED = [
  "indeterminate:bg-primary",
  "indeterminate:border-primary",
  "indeterminate:after:bg-primary-foreground",
  "indeterminate:after:h-0.5",
  "indeterminate:after:w-2",
  "indeterminate:after:content-['']",
];

const nothing = () => {};
const settled = () => Promise.resolve();
const done = () => Promise.resolve();

const remove: BulkAction<Item> = {
  label: "Delete selected",
  description: "Cannot undo",
  run: done,
};

function Bar({
  deck = false,
  actions = [remove],
  onClear = nothing,
  ...rest
}: Readonly<{
  deck?: boolean;
  actions?: readonly BulkAction<Item>[];
  onClear?: () => void;
  selected?: readonly Item[];
  selectionLabel?: string;
  hideOnPhone?: boolean;
  className?: string;
  children?: ReactNode;
}>) {
  const bar = (
    <BulkActions
      selected={items}
      name={(item) => item.name}
      actions={actions}
      onRemoved={nothing}
      onClear={onClear}
      onSettled={settled}
      {...rest}
    />
  );
  return deck ? <DeckScale>{bar}</DeckScale> : bar;
}

const barOf = (count: string) => screen.getByText(count).parentElement!;

afterEach(async () => {
  vi.unstubAllGlobals();
  await act(() => i18n.changeLanguage("vi"));
});

describe("outside a deck surface", () => {
  it("draws today's bar, byte for byte", () => {
    render(<Bar />);
    const bar = barOf("Đã chọn 2 mục");
    expect(bar.outerHTML).toBe(
      todayBar("Đã chọn 2 mục", todayAction("Delete selected")),
    );
    expect(bar.className).not.toContain("scale=deck");
    expect(screen.getByText("Đã chọn 2 mục").className).not.toContain("scale=deck");
  });

  it("keeps a page's label and its own controls before the actions", () => {
    render(
      <Bar
        selectionLabel="Đã chọn 2 câu"
        actions={[{ label: "Archive", description: "Later", run: done }, remove]}
      >
        <button type="button">Add tag</button>
      </Bar>,
    );
    expect(barOf("Đã chọn 2 câu").outerHTML).toBe(
      todayBar(
        "Đã chọn 2 câu",
        `<button type="button">Add tag</button>${todayAction("Archive")}${todayAction("Delete selected")}`,
      ),
    );
  });

  it("draws no icon, takes no class and stays on a phone whatever the deck props say", () => {
    viewport("phone");
    render(
      <Bar
        hideOnPhone
        className="mx-3 mb-2.5"
        actions={[{ ...remove, icon: CircleStop }]}
      >
        <BulkBarButton icon={Tag} onClick={nothing}>
          Add tag
        </BulkBarButton>
      </Bar>,
    );
    expect(barOf("Đã chọn 2 mục").outerHTML).toBe(
      todayBar(
        "Đã chọn 2 mục",
        `${todayAction("Add tag")}${todayAction("Delete selected")}`,
      ),
    );
  });
});

describe("on a deck surface", () => {
  it("writes the deck's selection count and clear name in English", async () => {
    render(<Bar deck />);
    await act(() => i18n.changeLanguage("en"));
    expect(screen.getByText("2 selected")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear selection" })).toBeInTheDocument();
  });

  it("is the inverted bar with the deck's count", () => {
    render(<Bar deck />);
    expect(screen.queryByText("Đã chọn 2 mục")).toBeNull();
    const bar = barOf("Đã chọn 2");
    const classes = bar.className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining(["bg-primary", "text-primary-fg", "rounded-lg"]),
    );
    expect(classes).not.toContain("bg-secondary");
    expect(bar.outerHTML).not.toContain("scale=deck");
  });

  it("recolours the focus ring for what is inside the bar", () => {
    render(<Bar deck />);
    expect(barOf("Đã chọn 2").className.split(" ")).toContain(
      "[--focus:var(--primary-fg)]",
    );
  });

  it("keeps a page's own label", () => {
    render(<Bar deck selectionLabel="Đã chọn 2 câu" />);
    expect(screen.getByText("Đã chọn 2 câu")).toBeInTheDocument();
    expect(screen.queryByText("Đã chọn 2")).toBeNull();
  });

  it("draws an action's icon before its label, and none for an action without one", () => {
    render(
      <Bar
        deck
        actions={[{ ...remove, label: "Close now", icon: CircleStop }, remove]}
      />,
    );
    const close = screen.getByRole("button", { name: "Close now" });
    const icon = close.querySelector("svg");
    expect(icon).not.toBeNull();
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(close.firstElementChild).toBe(icon);
    expect(
      screen.getByRole("button", { name: "Delete selected" }).querySelector("svg"),
    ).toBeNull();
  });

  it("clears through an icon button that carries the name", async () => {
    const onClear = vi.fn();
    render(<Bar deck onClear={onClear} />);
    const clear = screen.getByRole("button", { name: "Bỏ chọn" });
    expect(clear).toHaveTextContent("");
    expect(clear.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    await userEvent.setup().click(clear);
    expect(onClear).toHaveBeenCalledTimes(1);
  });

  it("takes the caller's class on the bar", () => {
    render(<Bar deck className="mx-3 mb-2.5" />);
    expect(barOf("Đã chọn 2").className.split(" ")).toEqual(
      expect.arrayContaining(["mx-3", "mb-2.5", "bg-primary"]),
    );
  });

  it("draws a page's own button with its icon and passes the click on", async () => {
    const open = vi.fn();
    render(
      <Bar deck>
        <BulkBarButton icon={Tag} onClick={open}>
          Add tag
        </BulkBarButton>
      </Bar>,
    );
    const add = screen.getByRole("button", { name: "Add tag" });
    expect(add.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    expect(add).not.toHaveAttribute("data-slot");
    expect(add.className).toBe(
      screen.getByRole("button", { name: "Delete selected" }).className,
    );
    expect(barOf("Đã chọn 2").children[1]).toBe(add);
    await userEvent.setup().click(add);
    expect(open).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("disables a page's own button when told to", async () => {
    const open = vi.fn();
    render(
      <Bar deck>
        <BulkBarButton onClick={open} disabled>
          Add tag
        </BulkBarButton>
      </Bar>,
    );
    const add = screen.getByRole("button", { name: "Add tag" });
    expect(add).toBeDisabled();
    expect(add.querySelector("svg")).toBeNull();
    await userEvent.setup().click(add);
    expect(open).not.toHaveBeenCalled();
  });

  it("runs the same confirmation, failure and retry", async () => {
    const run = vi
      .fn<(item: Item) => Promise<void>>()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(
        new ApiError({
          status: 409,
          code: "RESOURCE_REFERENCED",
          message: "Still assigned",
        }),
      )
      .mockResolvedValueOnce(undefined);
    function Harness() {
      const selection = useBulkSelection<Item>();
      return (
        <DeckScale>
          <button onClick={() => selection.selectPage(items, true)}>Select page</button>
          <BulkActions
            selected={[...selection.selected.values()]}
            name={(item) => item.name}
            actions={[
              {
                label: "Delete selected",
                description: "Cannot undo",
                run,
                icon: Clock,
              },
            ]}
            onRemoved={selection.remove}
            onClear={selection.clear}
            onSettled={settled}
          />
        </DeckScale>
      );
    }
    render(<Harness />);
    const user = userEvent.setup();
    expect(screen.queryByRole("button", { name: "Delete selected" })).toBeNull();
    await user.click(screen.getByText("Select page"));
    await user.click(screen.getByRole("button", { name: "Delete selected" }));
    expect(run).not.toHaveBeenCalled();
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("First")).toBeInTheDocument();
    expect(within(dialog).getByText("Referenced")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Xác nhận 2 mục" }));
    await screen.findByText("Referenced: Still assigned");
    expect(screen.getByText("Đã chọn 1")).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Thử lại 1 mục chưa thành công" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(run.mock.calls.map(([item]) => item.id)).toEqual(["a", "b", "b"]);
    expect(screen.queryByText(/^Đã chọn/)).toBeNull();
  });
});

describe("the bar on a phone", () => {
  it("is drawn on a deck surface unless the list asks for it to hide", () => {
    viewport("phone");
    const view = render(<Bar deck />);
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();
    view.unmount();
    render(<Bar deck hideOnPhone />);
    expect(screen.queryByText("Đã chọn 2")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("is drawn from 768px with hideOnPhone, and at the default width", () => {
    const first = render(<Bar deck hideOnPhone />);
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();
    first.unmount();
    const width = viewport(768);
    render(<Bar deck hideOnPhone />);
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();
    act(() => width.resize(767));
    expect(screen.queryByText("Đã chọn 2")).toBeNull();
    act(() => width.resize(800));
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();
  });

  it("keeps an open confirmation when the bar hides, and still runs it", async () => {
    const width = viewport("desktop");
    const run = vi.fn<(item: Item) => Promise<void>>().mockResolvedValue(undefined);
    render(<Bar deck hideOnPhone actions={[{ ...remove, run }]} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Delete selected" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    act(() => width.resize("phone"));
    expect(screen.queryByText("Đã chọn 2")).toBeNull();
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Referenced")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Xác nhận 2 mục" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(run.mock.calls.map(([item]) => item.id)).toEqual(["a", "b"]);
  });
});

describe("the confirmation's weight", () => {
  it.each([
    ["outside a deck surface", false],
    ["on a deck surface", true],
  ] as const)(
    "is destructive unless the action says otherwise, %s",
    async (_, deck) => {
      render(
        <Bar
          deck={deck}
          actions={[
            remove,
            {
              label: "Reset passwords",
              description: "Recorded",
              run: done,
              destructive: false,
            },
            {
              label: "Close now",
              description: "Submitted as is",
              run: done,
              destructive: true,
            },
          ]}
        />,
      );
      const user = userEvent.setup();
      const confirming = () => screen.getByRole("button", { name: "Xác nhận 2 mục" });
      const cancel = () => user.click(screen.getByRole("button", { name: "Huỷ" }));

      await user.click(screen.getByRole("button", { name: "Delete selected" }));
      expect(confirming()).toHaveAttribute("data-variant", "destructive");
      await cancel();
      await user.click(screen.getByRole("button", { name: "Reset passwords" }));
      expect(confirming()).toHaveAttribute("data-variant", "default");
      await cancel();
      await user.click(screen.getByRole("button", { name: "Close now" }));
      expect(confirming()).toHaveAttribute("data-variant", "destructive");
    },
  );
});

describe("the select-all box", () => {
  it("is mixed while some of the page is selected, with the primitive's bar once", async () => {
    function Page() {
      const selection = useBulkSelection<Item>();
      return (
        <>
          <button onClick={() => selection.toggle(items[0]!)}>Pick one</button>
          <BulkSelectAll items={items} selection={selection} />
        </>
      );
    }
    render(<Page />);
    const all = screen.getByRole<HTMLInputElement>("checkbox", {
      name: "Chọn tất cả mục trên trang này",
    });
    expect(all.indeterminate).toBe(false);
    expect(all).toHaveAttribute("aria-checked", "false");
    const user = userEvent.setup();
    await user.click(screen.getByText("Pick one"));
    expect(all.indeterminate).toBe(true);
    expect(all).toHaveAttribute("aria-checked", "mixed");
    const classes = all.className.split(" ");
    for (const name of MIXED) {
      expect(classes.filter((c) => c === name)).toHaveLength(1);
    }
    await user.click(all);
    expect(all.indeterminate).toBe(false);
    expect(all).toHaveAttribute("aria-checked", "true");
  });
});
