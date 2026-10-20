import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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
  expect(screen.getByText("Đã chọn 1")).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "Thử lại 1 mục chưa thành công" }),
  );
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(run.mock.calls.map(([item]) => item.id)).toEqual(["a", "b", "b"]);
});

type Item = (typeof items)[number];

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
  actions = [remove],
  onClear = nothing,
  ...rest
}: Readonly<{
  actions?: readonly BulkAction<Item>[];
  onClear?: () => void;
  selected?: readonly Item[];
  selectionLabel?: string;
  hideOnPhone?: boolean;
  className?: string;
  children?: ReactNode;
}>) {
  return (
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
}

const barOf = (count: string) => screen.getByText(count).parentElement!;

afterEach(async () => {
  vi.unstubAllGlobals();
  await act(() => i18n.changeLanguage("vi"));
});

describe("the bar", () => {
  it("writes the deck's selection count and clear name in English", async () => {
    render(<Bar />);
    await act(() => i18n.changeLanguage("en"));
    expect(screen.getByText("2 selected")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear selection" })).toBeInTheDocument();
  });

  it("is the inverted bar with the deck's count", () => {
    render(<Bar />);
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
    render(<Bar />);
    expect(barOf("Đã chọn 2").className.split(" ")).toContain(
      "[--focus:var(--primary-fg)]",
    );
  });

  it("keeps a page's own label", () => {
    render(<Bar selectionLabel="Đã chọn 2 câu" />);
    expect(screen.getByText("Đã chọn 2 câu")).toBeInTheDocument();
    expect(screen.queryByText("Đã chọn 2")).toBeNull();
  });

  it("draws the label, then a page's own controls, then the actions and the clear button", () => {
    render(
      <Bar
        selectionLabel="Đã chọn 2 câu"
        actions={[{ label: "Archive", description: "Later", run: done }, remove]}
      >
        <button type="button">Add tag</button>
      </Bar>,
    );
    expect(
      [...barOf("Đã chọn 2 câu").children].map(
        (child) => child.textContent || child.getAttribute("aria-label"),
      ),
    ).toEqual(["Đã chọn 2 câu", "Add tag", "Archive", "Delete selected", "Bỏ chọn"]);
  });

  it("draws an action's icon before its label, and none for an action without one", () => {
    render(
      <Bar actions={[{ ...remove, label: "Close now", icon: CircleStop }, remove]} />,
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
    render(<Bar onClear={onClear} />);
    const clear = screen.getByRole("button", { name: "Bỏ chọn" });
    expect(clear).toHaveTextContent("");
    expect(clear.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    await userEvent.setup().click(clear);
    expect(onClear).toHaveBeenCalledTimes(1);
  });

  it("keeps its buttons focusable while an action runs, and ignores them (T-35)", async () => {
    let finish: () => void = nothing;
    const held: BulkAction<Item> = {
      label: "Remove selected",
      description: "Their work is kept",
      run: () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    };
    const onClear = vi.fn();
    render(<Bar actions={[held]} onClear={onClear} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Remove selected" }));
    await user.click(screen.getByRole("button", { name: "Xác nhận 2 mục" }));
    const action = screen.getByRole("button", {
      name: "Remove selected",
      hidden: true,
    });
    const clear = screen.getByRole("button", { name: "Bỏ chọn", hidden: true });
    for (const button of [action, clear]) {
      expect(button).not.toBeDisabled();
      expect(button).toHaveAttribute("aria-disabled", "true");
      expect(button.className).not.toMatch(/(^|\s)(aria-disabled:)?opacity-/);
      button.focus();
    }
    fireEvent.click(clear);
    fireEvent.click(action);
    expect(onClear).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Xác nhận 2 mục" })).toHaveFocus();
    await act(async () => finish());
  });

  it("takes the caller's class on the bar", () => {
    render(<Bar className="mx-3 mb-2.5" />);
    expect(barOf("Đã chọn 2").className.split(" ")).toEqual(
      expect.arrayContaining(["mx-3", "mb-2.5", "bg-primary"]),
    );
  });

  it("draws a page's own button with its icon and passes the click on", async () => {
    const open = vi.fn();
    render(
      <Bar>
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
      <Bar>
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
        <>
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
        </>
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
  it("is drawn unless the list asks for it to hide", () => {
    viewport("phone");
    const view = render(<Bar />);
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();
    view.unmount();
    render(<Bar hideOnPhone />);
    expect(screen.queryByText("Đã chọn 2")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("is drawn from 768px with hideOnPhone, and at the default width", () => {
    const first = render(<Bar hideOnPhone />);
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();
    first.unmount();
    const width = viewport(768);
    render(<Bar hideOnPhone />);
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();
    act(() => width.resize(767));
    expect(screen.queryByText("Đã chọn 2")).toBeNull();
    act(() => width.resize(800));
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();
  });

  it("keeps an open confirmation when the bar hides, and still runs it", async () => {
    const width = viewport("desktop");
    const run = vi.fn<(item: Item) => Promise<void>>().mockResolvedValue(undefined);
    render(<Bar hideOnPhone actions={[{ ...remove, run }]} />);
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
  it("is destructive unless the action says otherwise", async () => {
    render(
      <Bar
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
  });
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
