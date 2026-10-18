import { useState } from "react";
import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DataTable, type DataColumn } from "@/components/shared/data/DataTable";
import { contentWidth } from "@tests/support/contentWidth";
import "@/lib/i18n";

interface Entry {
  id: string;
  what: string;
  who: string;
}

const ROWS: readonly Entry[] = [
  { id: "e1", what: "Đã tạo lớp", who: "Giàu Bùi" },
  { id: "e2", what: "Đã xoá bài kiểm tra", who: "Nam Nguyễn" },
];

const COLUMNS: readonly DataColumn<Entry>[] = [
  { id: "what", header: "Thao tác", track: "minmax(0,1fr)", cell: (row) => row.what },
  { id: "who", header: "Người thực hiện", track: "160px", cell: (row) => row.who },
  {
    id: "hidden",
    header: "Chi tiết",
    track: "60px",
    hideHeader: true,
    cell: () => "›",
  },
];

function Entries({ start = [] }: Readonly<{ start?: readonly string[] }>) {
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set(start));
  return (
    <DataTable
      label="Nhật ký"
      columns={COLUMNS}
      rows={ROWS}
      rowSize={{ minHeight: 48 }}
      onOpen={(row) =>
        setOpen((current) => {
          const next = new Set(current);
          if (!next.delete(row.id)) next.add(row.id);
          return next;
        })
      }
      expansion={{
        isOpen: (row) => open.has(row.id),
        detail: (row) => <p>Chi tiết của {row.what}</p>,
        label: (row) => `${row.what}, ${row.who}`,
      }}
    />
  );
}

beforeEach(() => contentWidth(1200));

describe("a row that opens a panel under itself", () => {
  it("names the opener by the row and leaves it collapsed, controlling nothing", () => {
    render(<Entries />);

    const opener = screen.getByRole("button", { name: "Đã tạo lớp, Giàu Bùi" });
    expect(opener).toHaveAttribute("aria-expanded", "false");
    expect(opener).not.toHaveAttribute("aria-controls");
    expect(screen.queryByText(/Chi tiết của/)).toBeNull();
  });

  it("draws the panel as a row of one cell right after its row, and points the opener at it", async () => {
    const user = userEvent.setup();
    render(<Entries />);

    const opener = screen.getByRole("button", { name: "Đã tạo lớp, Giàu Bùi" });
    await user.click(opener);

    expect(opener).toHaveAttribute("aria-expanded", "true");
    const panel = document.getElementById(opener.getAttribute("aria-controls")!)!;
    expect(panel).toHaveAttribute("role", "row");
    expect(within(panel).getAllByRole("cell")).toHaveLength(1);
    expect(within(panel).getByText("Chi tiết của Đã tạo lớp")).toBeVisible();
    const rows = screen.getAllByRole("row");
    expect(rows.indexOf(panel)).toBe(
      rows.indexOf(opener.closest("[role=row]") as HTMLElement) + 1,
    );
  });

  it("closes again from the same button, and leaves the other rows alone", async () => {
    const user = userEvent.setup();
    render(<Entries />);

    const first = screen.getByRole("button", { name: "Đã tạo lớp, Giàu Bùi" });
    const second = screen.getByRole("button", {
      name: "Đã xoá bài kiểm tra, Nam Nguyễn",
    });
    await user.click(first);
    await user.click(second);
    expect(screen.getAllByText(/Chi tiết của/)).toHaveLength(2);

    await user.click(first);

    expect(first).toHaveAttribute("aria-expanded", "false");
    expect(second).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByText(/Chi tiết của/)).toHaveLength(1);
    expect(screen.getByText("Chi tiết của Đã xoá bài kiểm tra")).toBeVisible();
  });

  it("opens rows that start open", () => {
    render(<Entries start={["e2"]} />);

    expect(
      screen.getByRole("button", { name: "Đã xoá bài kiểm tra, Nam Nguyễn" }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByText(/Chi tiết của/)).toHaveLength(1);
  });

  it("has no expanded state on a table without an expansion", () => {
    render(
      <DataTable
        label="Nhật ký"
        columns={COLUMNS}
        rows={ROWS}
        rowSize={{ minHeight: 48 }}
        onOpen={() => {}}
      />,
    );

    const opener = screen.getByRole("button", { name: /Đã tạo lớp/ });
    expect(opener).not.toHaveAttribute("aria-expanded");
    expect(opener).not.toHaveAttribute("aria-controls");
  });
});

describe("a column with a hidden header", () => {
  it("keeps the header's text for assistive technology and draws none", () => {
    render(<Entries />);

    const header = screen.getByRole("columnheader", { name: "Chi tiết" });
    const text = within(header).getByText("Chi tiết");
    expect(text).toHaveClass("sr-only");
    expect(
      screen.getByRole("columnheader", { name: "Thao tác" }).querySelector(".sr-only"),
    ).toBeNull();
  });
});
