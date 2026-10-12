import { useState, type RefObject } from "react";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import { DataTable, type DataColumn } from "@/components/shared/data/DataTable";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { contentWidth } from "@tests/support/contentWidth";
import { viewport } from "@tests/support/viewport";
import "@/lib/i18n";

type Row = { id: string; name: string };
const rows = [
  { id: "a", name: "An" },
  { id: "b", name: "Bình" },
];
const columns: DataColumn<Row>[] = [
  { id: "name", header: "Tên", track: "1fr", cell: (row) => row.name },
];
beforeEach(() => {
  viewport("desktop");
  contentWidth(1100);
});
it("exposes committed stable per-row trigger refs across reorder and removal without opening the row", async () => {
  const refs = new Map<string, RefObject<HTMLButtonElement | null>>();
  const selected = vi.fn();
  const opened = vi.fn();
  let change!: (rows: Row[]) => void;
  const menu = vi.fn(
    (row: Row, context: { triggerRef: RefObject<HTMLButtonElement | null> }) => {
      if (context) refs.set(row.id, context.triggerRef);
      return (
        <DropdownMenuItem onSelect={() => selected(context?.triggerRef.current)}>
          Chọn {row.name}
        </DropdownMenuItem>
      );
    },
  );
  function Table() {
    const [items, setItems] = useState(rows);
    change = setItems;
    return (
      <DataTable
        label="Học viên"
        columns={columns}
        rows={items}
        rowSize={{ height: 54 }}
        menu={menu}
        onOpen={opened}
      />
    );
  }
  render(
    <MemoryRouter>
      <Table />
    </MemoryRouter>,
  );
  const a = within(screen.getByRole("row", { name: "An" })).getByRole("button", {
    name: "Thao tác",
  });
  const b = within(screen.getByRole("row", { name: "Bình" })).getByRole("button", {
    name: "Thao tác",
  });
  expect(refs.get("a")?.current).toBe(a);
  expect(refs.get("b")?.current).toBe(b);
  expect(menu.mock.calls.map(([row]) => row.id)).toEqual(["a", "b"]);
  const refA = refs.get("a");
  await act(() => change([...rows].reverse()));
  expect(refs.get("a")).toBe(refA);
  expect(refA?.current).toBe(a);
  const user = userEvent.setup();
  b.focus();
  await user.keyboard("{Enter}");
  await user.click(await screen.findByRole("menuitem", { name: "Chọn Bình" }));
  expect(selected).toHaveBeenCalledWith(b);
  expect(opened).not.toHaveBeenCalled();
  await act(() => change([rows[1]!]));
  expect(refA?.current).toBeNull();
  expect(a.isConnected).toBe(false);
});
it.each([null, undefined, false, []])(
  "keeps a one-argument empty menu %j and the same header/track",
  (empty) => {
    const menu = vi.fn((row: Row) =>
      row.id === "a" ? empty : <DropdownMenuItem>Thật</DropdownMenuItem>,
    );
    render(
      <MemoryRouter>
        <DataTable
          label="Danh sách"
          columns={columns}
          rows={rows}
          rowSize={{ height: 54 }}
          menu={menu}
        />
      </MemoryRouter>,
    );
    const a = screen.getByRole("row", { name: "An" });
    const b = screen.getByRole("row", { name: "Bình" });
    expect(within(a).queryByRole("button")).toBeNull();
    expect(within(b).getByRole("button", { name: "Thao tác" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Thao tác" })).toBeInTheDocument();
    expect(a.style.gridTemplateColumns).toBe(b.style.gridTemplateColumns);
    expect(within(a).getAllByRole("cell")).toHaveLength(2);
    expect(menu).toHaveBeenCalledTimes(2);
  },
);
