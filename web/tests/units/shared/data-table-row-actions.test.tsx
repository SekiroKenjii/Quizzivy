import { useState } from "react";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import {
  DataTable,
  type DataColumn,
  type DataTableProps,
} from "@/components/shared/data/DataTable";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { contentWidth } from "@tests/support/contentWidth";
import { viewport } from "@tests/support/viewport";
import "@/lib/i18n";

type Row = { id: string; name: string };
const rows: Row[] = [
  { id: "plain", name: "Chưa bắt đầu" },
  { id: "started", name: "Đã bắt đầu" },
];
const columns: DataColumn<Row>[] = [
  { id: "name", header: "Học viên", track: "1fr", cell: (row) => row.name },
];
const opened = vi.fn();
function table(
  extra: Partial<
    Pick<DataTableProps<Row>, "rows" | "rowTone" | "canOpen" | "menu" | "card">
  > = {},
) {
  return (
    <DataTable
      label="Danh sách"
      columns={columns}
      rows={rows}
      rowSize={{ height: 54 }}
      onOpen={opened}
      {...extra}
    />
  );
}
function mount(
  extra: Partial<
    Pick<DataTableProps<Row>, "rows" | "rowTone" | "canOpen" | "menu" | "card">
  > = {},
) {
  render(<MemoryRouter>{table(extra)}</MemoryRouter>);
  return userEvent.setup();
}
beforeEach(() => {
  opened.mockClear();
  viewport("desktop");
  contentWidth(1100);
});

describe("additive per-row table contracts", () => {
  it("keeps forbidden rows without an opener or misleading pointer and permits real keyboard opening", async () => {
    const user = mount({ canOpen: (row) => row.id === "started" });
    const plain = screen.getByRole("row", { name: "Chưa bắt đầu" });
    expect(within(plain).queryByRole("button")).toBeNull();
    expect(plain).not.toHaveAttribute("tabindex");
    expect(plain).not.toHaveClass("cursor-pointer");
    fireEvent.click(plain);
    expect(opened).not.toHaveBeenCalled();
    const control = screen.getByRole("button", { name: "Đã bắt đầu" });
    control.focus();
    await user.keyboard("{Enter} ");
    expect(opened.mock.calls).toEqual([[rows[1]], [rows[1]]]);
  });
  it("leaves default consumers openable and untinted", () => {
    mount();
    fireEvent.click(screen.getByRole("row", { name: "Chưa bắt đầu" }));
    expect(opened).toHaveBeenCalledWith(rows[0]);
    for (const row of screen.getAllByRole("row").slice(1)) {
      expect(row).not.toHaveClass("bg-danger-soft");
      expect(row).not.toHaveClass("bg-muted");
    }
  });
  it("applies only explicit row tones and keeps keyed row identity across sorting", async () => {
    let reorder!: () => void;
    function Changing() {
      const [items, setItems] = useState(rows);
      reorder = () => setItems([...items].reverse());
      return table({
        rows: items,
        rowTone: (row) => (row.id === "plain" ? "danger" : "selected"),
      });
    }
    render(
      <MemoryRouter>
        <Changing />
      </MemoryRouter>,
    );
    const plain = screen.getByRole("row", { name: "Chưa bắt đầu" });
    const started = screen.getByRole("row", { name: "Đã bắt đầu" });
    expect(plain).toHaveClass("bg-danger-soft");
    expect(started).toHaveClass("bg-muted");
    await act(() => reorder());
    expect(screen.getByRole("row", { name: "Chưa bắt đầu" })).toBe(plain);
    expect(screen.getByRole("row", { name: "Đã bắt đầu" })).toBe(started);
  });
  it.each([null, undefined, false, []])(
    "omits a %j row menu while retaining its header and grid cell",
    (empty) => {
      const menu = vi.fn((row: Row) =>
        row.id === "plain" ? empty : <DropdownMenuItem>Thao tác thật</DropdownMenuItem>,
      );
      mount({ menu });
      const plain = screen.getByRole("row", { name: "Chưa bắt đầu" });
      const started = screen.getByRole("row", { name: "Đã bắt đầu" });
      expect(within(plain).queryByRole("button", { name: "Thao tác" })).toBeNull();
      expect(
        within(started).getByRole("button", { name: "Thao tác" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("columnheader", { name: "Thao tác" }),
      ).toBeInTheDocument();
      expect(within(plain).getAllByRole("cell")).toHaveLength(2);
      expect(plain.style.gridTemplateColumns).toBe(started.style.gridTemplateColumns);
      expect(menu.mock.calls.map(([row]) => row.id)).toEqual(["plain", "started"]);
    },
  );
  it("does not inspect component or fragment internals and keeps real menu propagation", async () => {
    const action = vi.fn();
    const user = mount({
      menu: (row) =>
        row.id === "plain" ? (
          <></>
        ) : (
          <DropdownMenuItem onSelect={action}>Thao tác thật</DropdownMenuItem>
        ),
    });
    expect(
      within(screen.getByRole("row", { name: "Chưa bắt đầu" })).getByRole("button", {
        name: "Thao tác",
      }),
    ).toBeInTheDocument();
    await user.click(
      within(screen.getByRole("row", { name: "Đã bắt đầu" })).getByRole("button", {
        name: "Thao tác",
      }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Thao tác thật" }));
    expect(action).toHaveBeenCalledOnce();
    expect(opened).not.toHaveBeenCalled();
  });
  it("honors canOpen in the existing card branch without changing card defaults", async () => {
    viewport("phone");
    const user = mount({
      canOpen: (row) => row.id === "started",
      card: (row) => <span>{row.name}</span>,
    });
    expect(screen.queryByRole("button", { name: "Chưa bắt đầu" })).toBeNull();
    await user.click(screen.getByText("Chưa bắt đầu"));
    expect(opened).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Đã bắt đầu" }));
    expect(opened).toHaveBeenCalledWith(rows[1]);
  });
});
