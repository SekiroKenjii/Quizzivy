import { useState, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import {
  DataTable,
  type DataColumn,
  type DataTableMenuContext,
  type DataTableProps,
  type RowSize,
} from "@/components/shared/data/DataTable";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useContentBand } from "@/layouts/shell/contentWidth";
import { previewCompactTables, setAccountCompactTables } from "@/lib/compactTables";
import { contentWidth } from "@tests/support/contentWidth";
import { viewport } from "@tests/support/viewport";
import "@/lib/i18n";

interface Assignment {
  id: string;
  title: string;
  meta: string;
  classes: string;
  when: string;
  submitted: number;
  target: number;
}

const ROWS: readonly Assignment[] = [
  {
    id: "a1",
    title: "Mid-term Reading Mock",
    meta: "Test v3 · 40 questions",
    classes: "IELTS 6.5 Evening",
    when: "Today, 21:00",
    submitted: 18,
    target: 24,
  },
  {
    id: "a2",
    title: "Unit 4 · Listening: Directions",
    meta: "Test v1 · 20 questions",
    classes: "IELTS Foundation A",
    when: "Tomorrow, 08:00",
    submitted: 9,
    target: 16,
  },
  {
    id: "a3",
    title: "Vocabulary Quiz: Travel",
    meta: "Test v2 · 25 questions",
    classes: "TOEIC 600 Weekend, Kids Starters B",
    when: "Fri 26 Sep",
    submitted: 11,
    target: 30,
  },
];

const PAGE_TWO: readonly Assignment[] = [
  {
    id: "a4",
    title: "Present Perfect vs Past Simple",
    meta: "Test v1 · 30 questions",
    classes: "IELTS Foundation A",
    when: "Opens Mon 29 Sep",
    submitted: 0,
    target: 16,
  },
];

const titleCell = vi.fn();

const COLUMNS: readonly DataColumn<Assignment>[] = [
  {
    id: "title",
    header: "Bài giao",
    track: "minmax(200px,2.2fr)",
    cell: (row, shown) => {
      titleCell(row.id);
      return (
        <span>
          <span>{row.title}</span>
          <span>{row.meta}</span>
          {shown.has("classes") && shown.has("when") ? null : (
            <span data-testid={`sub-${row.id}`}>
              {row.classes} · {row.when}
            </span>
          )}
        </span>
      );
    },
  },
  {
    id: "classes",
    header: "Giao cho",
    track: "minmax(130px,1.3fr)",
    showFrom: 860,
    cell: (row) => row.classes,
  },
  {
    id: "when",
    header: "Đóng lúc",
    track: "120px",
    showFrom: 700,
    cell: (row) => row.when,
  },
  {
    id: "submitted",
    header: "Đã nộp",
    track: "150px",
    showFrom: 560,
    cell: (row) => `${row.submitted}/${row.target}`,
  },
  {
    id: "status",
    header: "Trạng thái",
    track: "110px",
    showFrom: 760,
    cell: () => "Đang mở",
  },
];

const A_COLS = {
  all: "16px minmax(200px,2.2fr) minmax(130px,1.3fr) 120px 150px 110px 44px",
  from760: "16px minmax(200px,2.2fr) 120px 150px 110px 44px",
  from700: "16px minmax(200px,2.2fr) 120px 150px 44px",
  from560: "16px minmax(200px,2.2fr) 150px 44px",
  narrow: "16px minmax(200px,2.2fr) 44px",
};

const SELECT_PAGE = "Chọn tất cả mục trên trang này";
const MENU = "Thao tác";

const onMenuItem = vi.fn();

function Where() {
  const location = useLocation();
  return <output>{location.pathname}</output>;
}

type Overrides = Partial<
  Pick<
    DataTableProps<Assignment>,
    | "rows"
    | "rowSize"
    | "dense"
    | "padX"
    | "menuTrack"
    | "card"
    | "cardLayout"
    | "framed"
    | "empty"
    | "footer"
  >
>;

function Assignments({
  rows = ROWS,
  rowSize = { minHeight: 60 },
  plain = false,
  ...rest
}: Readonly<Overrides & { plain?: boolean }>) {
  const selection = useBulkSelection<Assignment>();
  if (plain) {
    return (
      <DataTable
        label="Bài giao"
        columns={COLUMNS}
        rows={rows}
        rowSize={rowSize}
        {...rest}
      />
    );
  }
  return (
    <>
      <DataTable
        label="Bài giao"
        columns={COLUMNS}
        rows={rows}
        rowSize={rowSize}
        rowHref={(row) => `/teacher/assignments/${row.id}`}
        selection={selection}
        rowName={(row) => row.title}
        menu={() => <DropdownMenuItem onSelect={onMenuItem}>Nhân bản</DropdownMenuItem>}
        {...rest}
      />
      <p data-testid="chosen">
        {[...selection.selected.values()].map((item) => item.title).join(" | ")}
      </p>
    </>
  );
}

function renderAssignments(props: Parameters<typeof Assignments>[0] = {}) {
  return render(
    <MemoryRouter initialEntries={["/teacher/assignments"]}>
      <Routes>
        <Route path="/teacher/assignments" element={<Assignments {...props} />} />
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  );
}

const headers = () =>
  screen.getAllByRole("columnheader").map((header) => header.textContent);

const bodyRows = () => screen.getAllByRole("row").slice(1);

const templates = () =>
  screen.getAllByRole("row").map((row) => row.style.gridTemplateColumns);

const opened = () => document.querySelector("output")?.textContent ?? null;

const frame = (container: HTMLElement) =>
  container.querySelector<HTMLElement>('[data-slot="data-table"]')!;

beforeEach(() => {
  titleCell.mockClear();
  onMenuItem.mockClear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("the columns follow the content width", () => {
  it("shows every column and no sub-line at 1200", () => {
    contentWidth(1200);
    renderAssignments();
    expect(headers()).toEqual([
      "",
      "Bài giao",
      "Giao cho",
      "Đóng lúc",
      "Đã nộp",
      "Trạng thái",
      MENU,
    ]);
    expect(screen.queryByTestId("sub-a1")).toBeNull();
    expect(new Set(templates())).toEqual(new Set([A_COLS.all]));
  });

  it.each([
    [860, A_COLS.all, ["Giao cho", "Đóng lúc", "Đã nộp", "Trạng thái"]],
    [859, A_COLS.from760, ["Đóng lúc", "Đã nộp", "Trạng thái"]],
    [760, A_COLS.from760, ["Đóng lúc", "Đã nộp", "Trạng thái"]],
    [759, A_COLS.from700, ["Đóng lúc", "Đã nộp"]],
    [720, A_COLS.from700, ["Đóng lúc", "Đã nộp"]],
    [700, A_COLS.from700, ["Đóng lúc", "Đã nộp"]],
    [699, A_COLS.from560, ["Đã nộp"]],
    [560, A_COLS.from560, ["Đã nộp"]],
    [559, A_COLS.narrow, []],
  ])("draws the deck's columns at %i", (width, template, dropping) => {
    contentWidth(width);
    renderAssignments();
    expect(headers()).toEqual(["", "Bài giao", ...dropping, MENU]);
    expect(new Set(templates())).toEqual(new Set([template]));
  });

  it.each([859, 760, 720, 699, 559])(
    "reads classes and window, both and nothing else, under the title at %i",
    (width) => {
      contentWidth(width);
      renderAssignments();
      expect(screen.getByTestId("sub-a1").textContent).toBe(
        "IELTS 6.5 Evening · Today, 21:00",
      );
      expect(screen.getByTestId("sub-a3").textContent).toBe(
        "TOEIC 600 Weekend, Kids Starters B · Fri 26 Sep",
      );
    },
  );

  it("tells a cell the ids of the columns shown", () => {
    const seen = vi.fn();
    const columns: readonly DataColumn<Assignment>[] = [
      {
        id: "title",
        header: "Bài giao",
        track: "1fr",
        cell: (_, shown) => {
          seen([...shown]);
          return null;
        },
      },
      ...COLUMNS.slice(1),
    ];
    contentWidth(720);
    render(
      <DataTable
        label="Bài giao"
        columns={columns}
        rows={ROWS.slice(0, 1)}
        rowSize={{ height: 56 }}
      />,
    );
    expect(seen).toHaveBeenLastCalledWith(["title", "when", "submitted"]);
  });

  it("draws a cell again when a threshold is crossed and not inside a band", () => {
    const width = contentWidth(900);
    renderAssignments();
    expect(titleCell).toHaveBeenCalledTimes(3);
    width.resize(1000);
    expect(titleCell).toHaveBeenCalledTimes(3);
    width.resize(861);
    expect(titleCell).toHaveBeenCalledTimes(3);
    width.resize(859);
    expect(titleCell).toHaveBeenCalledTimes(6);
    expect(screen.getByTestId("sub-a1")).toBeInTheDocument();
    width.resize(761);
    expect(titleCell).toHaveBeenCalledTimes(6);
  });

  it("keeps the visible set's identity while the table renders again for another reason", async () => {
    contentWidth(1200);
    renderAssignments();
    expect(titleCell).toHaveBeenCalledTimes(3);
    await userEvent
      .setup()
      .click(screen.getByRole("checkbox", { name: "Chọn Mid-term Reading Mock" }));
    expect(screen.getByTestId("chosen").textContent).toBe("Mid-term Reading Mock");
    expect(titleCell).toHaveBeenCalledTimes(3);
  });
});

interface Member {
  id: string;
  name: string;
}

const MEMBERS: readonly Member[] = [
  { id: "m1", name: "Lê Hoàng Nam" },
  { id: "m2", name: "Trần Minh Anh" },
];

const MEMBER_COLUMNS: readonly DataColumn<Member>[] = [
  {
    id: "name",
    header: "Học viên",
    track: "minmax(180px,2fr)",
    cell: (row) => row.name,
  },
  {
    id: "via",
    header: "Tham gia qua",
    track: "120px",
    showFrom: 5000,
    cell: () => "Mã",
  },
  { id: "at", header: "Ngày tham gia", track: "80px", cell: () => "12 Aug" },
  { id: "sub", header: "Đã nộp", track: "80px", align: "end", cell: () => "4" },
  { id: "avg", header: "Điểm TB", track: "70px", align: "end", cell: () => "76%" },
];

const MEMBER_BANDS = [520, 620, 720, 1000, 1080] as const;

const MEMBER_SETS: readonly ReadonlySet<string>[] = [
  new Set(["name", "sub"]),
  new Set(["name", "sub", "avg"]),
  new Set(["name", "via", "sub", "avg"]),
  new Set(["name", "via", "at", "sub", "avg"]),
  new Set(["name", "via", "sub", "avg"]),
  new Set(["name", "via", "at", "sub", "avg"]),
];

function deckMembers(tw: number) {
  const left = tw >= 1000 ? (tw * 2) / 3 : tw;
  return [
    "name",
    left >= 620 && "via",
    left >= 720 && "at",
    "sub",
    left >= 520 && "avg",
  ].filter(Boolean);
}

function Members() {
  const band = useContentBand(MEMBER_BANDS);
  const selection = useBulkSelection<Member>();
  return (
    <DataTable
      label="Thành viên"
      columns={MEMBER_COLUMNS}
      rows={MEMBERS}
      rowSize={{ minHeight: 56 }}
      shown={MEMBER_SETS[band]!}
      selection={selection}
      rowName={(row) => row.name}
      menu={() => <DropdownMenuItem>Xóa khỏi lớp</DropdownMenuItem>}
      menuTrack="32px"
      padX={18}
      framed={false}
    />
  );
}

describe("a table that passes the visible set itself", () => {
  it("reproduces the deck's two-thirds arithmetic with five bands of the content width", () => {
    for (let tw = 300; tw <= 1500; tw += 1) {
      const band = MEMBER_BANDS.filter((threshold) => tw >= threshold).length;
      expect([tw, ...MEMBER_SETS[band]!]).toEqual([
        tw,
        ...MEMBER_COLUMNS.map((column) => column.id).filter((id) =>
          deckMembers(tw).includes(id),
        ),
      ]);
    }
  });

  it("shows Joined at 900, hides it at 1040 and shows it again at 1100", () => {
    const width = contentWidth(900);
    render(<Members />);
    expect(headers()).toContain("Ngày tham gia");
    expect(templates()[0]).toBe("16px minmax(180px,2fr) 120px 80px 80px 70px 32px");
    width.resize(1040);
    expect(headers()).not.toContain("Ngày tham gia");
    expect(headers()).toContain("Tham gia qua");
    expect(templates()[0]).toBe("16px minmax(180px,2fr) 120px 80px 70px 32px");
    width.resize(1100);
    expect(headers()).toContain("Ngày tham gia");
  });

  it("ignores showFrom: a column the set names shows below its own width, and one it leaves out is hidden", () => {
    contentWidth(600);
    render(
      <DataTable
        label="Thành viên"
        columns={MEMBER_COLUMNS}
        rows={MEMBERS}
        rowSize={{ minHeight: 56 }}
        shown={new Set(["name", "via"])}
      />,
    );
    expect(headers()).toEqual(["Học viên", "Tham gia qua"]);
    expect(new Set(templates())).toEqual(new Set(["minmax(180px,2fr) 120px"]));
  });

  it("does not read its own band: crossing a column's showFrom renders nothing again", () => {
    const menu = vi.fn(() => null);
    const shown: ReadonlySet<string> = new Set(["name", "via"]);
    const columns: readonly DataColumn<Member>[] = [
      MEMBER_COLUMNS[0]!,
      { ...MEMBER_COLUMNS[1]!, showFrom: 700 },
    ];
    const width = contentWidth(900);
    render(
      <DataTable
        label="Thành viên"
        columns={columns}
        rows={MEMBERS}
        rowSize={{ minHeight: 56 }}
        shown={shown}
        menu={menu}
      />,
    );
    expect(menu).toHaveBeenCalledTimes(2);
    width.resize(650);
    expect(menu).toHaveBeenCalledTimes(2);
    expect(headers()).toEqual(["Học viên", "Tham gia qua", MENU]);
  });

  it("carries the table's own padding and menu track to the header and every row", () => {
    contentWidth(1200);
    render(<Members />);
    const rows = screen.getAllByRole("row");
    expect(rows).toHaveLength(3);
    for (const row of rows) {
      expect(row).toHaveStyle({ paddingLeft: "18px", paddingRight: "18px" });
      expect(row.style.gridTemplateColumns.endsWith(" 32px")).toBe(true);
    }
  });

  it("right-aligns the header and the cells of a column that asks for it", () => {
    contentWidth(1200);
    render(<Members />);
    expect(screen.getByRole("columnheader", { name: "Đã nộp" })).toHaveClass(
      "text-right",
    );
    expect(screen.getByRole("columnheader", { name: "Học viên" })).not.toHaveClass(
      "text-right",
    );
    const cells = within(bodyRows()[0]!).getAllByRole("cell");
    expect(cells[4]).toHaveTextContent("4");
    expect(cells[4]).toHaveClass("text-right");
    expect(cells[1]).not.toHaveClass("text-right");
  });

  it("leaves out the card around the table when it sits inside a section", () => {
    const { container } = render(<Members />);
    for (const name of ["rounded-xl", "border", "shadow-card", "bg-card"]) {
      expect(frame(container)).not.toHaveClass(name);
    }
    expect(frame(container)).toHaveClass("overflow-x-auto");
  });
});

describe("geometry the deck draws", () => {
  it("pads the header and the rows 16px and gives the menu 44px by default", () => {
    renderAssignments();
    for (const row of screen.getAllByRole("row")) {
      expect(row).toHaveStyle({ paddingLeft: "16px", paddingRight: "16px" });
      expect(row.style.gridTemplateColumns.endsWith(" 44px")).toBe(true);
    }
  });

  it("has no menu track, header or button without a menu, and no checkbox track without selection", () => {
    renderAssignments({ plain: true });
    expect(new Set(templates())).toEqual(
      new Set(["minmax(200px,2.2fr) minmax(130px,1.3fr) 120px 150px 110px"]),
    );
    expect(headers()).not.toContain(MENU);
    expect(screen.queryByRole("button", { name: MENU })).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
  });

  it("is at least as wide as its tracks, so the header band and the row borders span a sideways scroll", () => {
    const { container } = renderAssignments();
    const table = screen.getByRole("table", { name: "Bài giao" });
    expect(table).toHaveClass("min-w-min");
    expect(table.parentElement).toBe(frame(container));
    for (const row of screen.getAllByRole("row")) {
      expect(row).not.toHaveClass("min-w-min");
    }
  });

  it("draws the card, the 40px header row and the body rows with the deck's classes", () => {
    const { container } = renderAssignments();
    expect(frame(container)).toHaveClass(
      "bg-card",
      "shadow-card",
      "rounded-xl",
      "border",
      "overflow-x-auto",
      "overflow-y-hidden",
    );
    const [header, first] = screen.getAllByRole("row");
    expect(header).toHaveClass(
      "grid",
      "h-10",
      "items-center",
      "gap-3",
      "bg-muted",
      "text-muted-fg",
      "text-meta",
      "leading-normal",
      "font-medium",
    );
    expect(first).toHaveClass(
      "grid",
      "items-center",
      "gap-3",
      "border-t",
      "text-ui",
      "leading-normal",
      "hover:bg-muted",
      "cursor-pointer",
    );
  });

  it.each<[string, RowSize, Record<string, string>, Record<string, string>]>([
    ["a fixed 56px", { height: 56 }, { height: "56px" }, { height: "40px" }],
    ["a 60px minimum", { minHeight: 60 }, { minHeight: "60px" }, { minHeight: "44px" }],
    [
      "padded",
      { padY: 10 },
      { paddingTop: "10px", paddingBottom: "10px" },
      { paddingTop: "5px", paddingBottom: "5px" },
    ],
  ])("makes a body row %s, and compact when dense", (_, rowSize, drawn, compact) => {
    const { unmount } = renderAssignments({ rowSize });
    for (const row of bodyRows()) expect(row).toHaveStyle(drawn);
    expect(screen.getAllByRole("row")[0]!.style.height).toBe("");
    unmount();
    renderAssignments({ rowSize, dense: true });
    for (const row of bodyRows()) expect(row).toHaveStyle(compact);
    const header = screen.getAllByRole("row")[0]!;
    expect(header).toHaveClass("h-10");
    expect(header.style.height).toBe("");
    expect(header.style.paddingTop).toBe("");
  });

  describe("under the account's Compact tables (DG-37)", () => {
    afterEach(() => {
      act(() => setAccountCompactTables(null));
    });

    it("draws compact rows when the account chose them, and follows the choice without a reload", () => {
      renderAssignments({ rowSize: { height: 56 } });
      for (const row of bodyRows()) expect(row).toHaveStyle({ height: "56px" });

      act(() => setAccountCompactTables(true));
      for (const row of bodyRows()) expect(row).toHaveStyle({ height: "40px" });

      act(() => previewCompactTables(false));
      for (const row of bodyRows()) expect(row).toHaveStyle({ height: "56px" });
    });

    it("lets a table's own dense prop win over the account's choice", () => {
      act(() => setAccountCompactTables(true));
      const { unmount } = renderAssignments({ rowSize: { height: 56 }, dense: false });
      for (const row of bodyRows()) expect(row).toHaveStyle({ height: "56px" });
      unmount();

      act(() => setAccountCompactTables(false));
      renderAssignments({ rowSize: { height: 56 }, dense: true });
      for (const row of bodyRows()) expect(row).toHaveStyle({ height: "40px" });
    });
  });

  it("lets every data cell and its header shrink, so a long title truncates", () => {
    contentWidth(1200);
    renderAssignments();
    const cells = within(bodyRows()[0]!).getAllByRole("cell");
    for (const cell of cells.slice(1, 6)) expect(cell).toHaveClass("min-w-0");
    for (const header of screen.getAllByRole("columnheader").slice(1, 6)) {
      expect(header).toHaveClass("min-w-0");
    }
  });

  it("sets no height on a padded row and no padding on a fixed one", () => {
    const { unmount } = renderAssignments({ rowSize: { padY: 10 } });
    expect(bodyRows()[0]!.style.height).toBe("");
    expect(bodyRows()[0]!.style.minHeight).toBe("");
    unmount();
    renderAssignments({ rowSize: { height: 54 } });
    expect(bodyRows()[0]!.style.paddingTop).toBe("");
    expect(bodyRows()[0]!.style.minHeight).toBe("");
  });
});

describe("roles", () => {
  it("is a named table of rows, column headers and cells", () => {
    contentWidth(1200);
    renderAssignments();
    const table = screen.getByRole("table", { name: "Bài giao" });
    expect(within(table).getAllByRole("rowgroup")).toHaveLength(2);
    expect(within(table).getAllByRole("row")).toHaveLength(4);
    expect(within(table).getAllByRole("columnheader")).toHaveLength(7);
    for (const row of bodyRows()) {
      expect(within(row).getAllByRole("cell")).toHaveLength(7);
    }
    expect(screen.getByRole("columnheader", { name: MENU })).toBeInTheDocument();
  });

  it("hides the menu column's header text and lines the checkboxes and the menu up in their cells", () => {
    contentWidth(1200);
    renderAssignments();
    expect(
      screen.getByRole("columnheader", { name: MENU }).firstElementChild,
    ).toHaveClass("sr-only");
    expect(screen.getAllByRole("columnheader")[0]).toHaveClass("flex");
    const cells = within(bodyRows()[0]!).getAllByRole("cell");
    expect(cells[0]).toHaveClass("flex");
    expect(cells[6]).toHaveClass("flex");
  });

  it("positions the menu column's header, so its hidden text scrolls with the table and never widens the page", () => {
    contentWidth(1200);
    renderAssignments();
    const header = screen.getByRole("columnheader", { name: MENU });
    expect(header).toHaveClass("relative");
    expect(header.firstElementChild).toHaveClass("sr-only");
  });

  it("gives each row's menu its own row", () => {
    const menu = vi.fn((row: Assignment, _context: DataTableMenuContext) => (
      <DropdownMenuItem>Nhân bản {row.title}</DropdownMenuItem>
    ));
    render(
      <DataTable
        label="Bài giao"
        columns={COLUMNS}
        rows={ROWS}
        rowSize={{ minHeight: 60 }}
        menu={menu}
      />,
    );
    expect(menu.mock.calls.map(([row]) => row)).toEqual(ROWS);
    expect(menu).toHaveBeenCalledTimes(ROWS.length);
    const bodyRows = screen.getAllByRole("row").slice(1);
    const refs = menu.mock.calls.map(([, context], index) => {
      expect(Object.keys(context)).toEqual(["triggerRef"]);
      expect(context.triggerRef.current).toBe(
        within(bodyRows[index]!).getByRole("button", { name: "Thao tác" }),
      );
      return context.triggerRef;
    });
    expect(new Set(refs).size).toBe(ROWS.length);
  });

  it("names a row by its first cell", () => {
    contentWidth(1200);
    renderAssignments();
    expect(
      screen.getByRole("row", { name: "Mid-term Reading MockTest v3 · 40 questions" }),
    ).toBe(bodyRows()[0]);
  });

  it("keeps the row out of the tab order and gives it no button role", () => {
    renderAssignments();
    for (const row of bodyRows()) {
      expect(row).not.toHaveAttribute("tabindex");
      expect(row).toHaveAttribute("role", "row");
    }
  });
});

describe("opening a row", () => {
  it("wraps the first cell in a real link to the row", () => {
    renderAssignments();
    const link = within(bodyRows()[0]!).getByRole("link", {
      name: /Mid-term Reading Mock/,
    });
    expect(link).toHaveAttribute("href", "/teacher/assignments/a1");
    expect(link.closest('[role="cell"]')).toBe(
      within(bodyRows()[0]!).getAllByRole("cell")[1],
    );
    expect(within(bodyRows()[0]!).getAllByRole("link")).toHaveLength(1);
    expect(link).toHaveClass("block", "w-full", "min-w-0");
  });

  it("opens on Enter from the link", async () => {
    const user = userEvent.setup();
    renderAssignments();
    within(bodyRows()[1]!).getByRole("link").focus();
    await user.keyboard("{Enter}");
    expect(opened()).toBe("/teacher/assignments/a2");
  });

  it("opens on a click anywhere else on the row", async () => {
    const user = userEvent.setup();
    renderAssignments();
    await user.click(within(bodyRows()[2]!).getByText("Fri 26 Sep"));
    expect(opened()).toBe("/teacher/assignments/a3");
  });

  it.each(["Control", "Meta", "Shift", "Alt"])(
    "stays shut on a click off the link while %s is held",
    async (key) => {
      const user = userEvent.setup();
      renderAssignments();
      await user.keyboard(`{${key}>}`);
      await user.click(within(bodyRows()[2]!).getByText("Fri 26 Sep"));
      await user.keyboard(`{/${key}}`);
      expect(opened()).toBeNull();
      await user.click(within(bodyRows()[2]!).getByText("Fri 26 Sep"));
      expect(opened()).toBe("/teacher/assignments/a3");
    },
  );

  it("stays shut on the checkbox", async () => {
    const user = userEvent.setup();
    renderAssignments();
    await user.click(
      screen.getByRole("checkbox", { name: "Chọn Mid-term Reading Mock" }),
    );
    expect(opened()).toBeNull();
    expect(screen.getByTestId("chosen").textContent).toBe("Mid-term Reading Mock");
  });

  it("stays shut on the menu button and on a menu item", async () => {
    const user = userEvent.setup();
    renderAssignments();
    await user.click(screen.getAllByRole("button", { name: MENU })[0]!);
    expect(opened()).toBeNull();
    await user.click(await screen.findByRole("menuitem", { name: "Nhân bản" }));
    expect(onMenuItem).toHaveBeenCalledTimes(1);
    expect(opened()).toBeNull();
    expect(document.querySelector('[role="table"]')).not.toBeNull();
  });

  it("stays shut on a control a cell draws", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onTag = vi.fn();
    const columns: readonly DataColumn<Assignment>[] = [
      COLUMNS[0]!,
      {
        id: "tag",
        header: "Nhãn",
        track: "120px",
        cell: () => (
          <button type="button" onClick={onTag}>
            <span>Gỡ nhãn</span>
          </button>
        ),
      },
    ];
    render(
      <DataTable
        label="Bài giao"
        columns={columns}
        rows={ROWS}
        rowSize={{ padY: 10 }}
        onOpen={onOpen}
      />,
    );
    await user.click(screen.getAllByText("Gỡ nhãn")[0]!);
    expect(onTag).toHaveBeenCalledTimes(1);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("stays shut on a control in the first column's aside and opens on its text", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onRemove = vi.fn();
    const aside = vi.fn((row: Assignment) => (
      <span>
        <span>reading</span>
        <button type="button" aria-label={`Gỡ nhãn ${row.id}`} onClick={onRemove} />
      </span>
    ));
    const columns: readonly DataColumn<Assignment>[] = [
      { ...COLUMNS[0]!, aside },
      COLUMNS[2]!,
    ];
    const table = (label: string) => (
      <DataTable
        label={label}
        columns={columns}
        rows={ROWS}
        rowSize={{ padY: 10 }}
        onOpen={onOpen}
      />
    );
    contentWidth(600);
    const { rerender } = render(table("Bài giao"));
    expect(aside.mock.calls).toEqual(ROWS.map((row) => [row, new Set(["title"])]));
    rerender(table("Bài giao đang mở"));
    expect(screen.getByRole("table", { name: "Bài giao đang mở" })).toBeInTheDocument();
    expect(aside).toHaveBeenCalledTimes(3);
    const opener = screen.getByRole("button", { name: /Mid-term Reading Mock/ });
    const remove = screen.getByRole("button", { name: "Gỡ nhãn a1" });
    expect(remove.closest('[data-slot="data-table-open"]')).toBeNull();
    expect(remove.closest('[role="cell"]')).toBe(opener.parentElement);
    expect(opener.nextElementSibling).toContainElement(remove);
    await user.click(remove);
    expect(onRemove).toHaveBeenCalledTimes(1);
    expect(onOpen).not.toHaveBeenCalled();
    await user.click(screen.getAllByText("reading")[0]!);
    expect(onOpen.mock.calls).toEqual([[ROWS[0]]]);
  });

  it("keeps the aside out of a row's link and draws it on a row that opens nothing", () => {
    const columns: readonly DataColumn<Assignment>[] = [
      {
        ...COLUMNS[0]!,
        aside: (row) => (
          <button type="button" aria-label={`Gỡ nhãn ${row.id}`} onClick={() => {}} />
        ),
      },
    ];
    const { unmount } = render(
      <MemoryRouter>
        <DataTable
          label="Bài giao"
          columns={columns}
          rows={ROWS}
          rowSize={{ padY: 10 }}
          rowHref={(row) => `/teacher/assignments/${row.id}`}
        />
      </MemoryRouter>,
    );
    const link = screen.getByRole("link", { name: /Mid-term Reading Mock/ });
    expect(link).not.toContainElement(
      screen.getByRole("button", { name: "Gỡ nhãn a1" }),
    );
    unmount();
    render(
      <DataTable
        label="Bài giao"
        columns={columns}
        rows={ROWS}
        rowSize={{ padY: 10 }}
      />,
    );
    expect(screen.getAllByRole("button", { name: /Gỡ nhãn/ })).toHaveLength(3);
  });

  it("stays shut on a focusable element and on a label a cell draws", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const columns: readonly DataColumn<Assignment>[] = [
      COLUMNS[0]!,
      {
        id: "note",
        header: "Ghi chú",
        track: "160px",
        cell: (row) => (
          <>
            {/* eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- a scroll region a keyboard reaches, as ContentView draws one */}
            <span tabIndex={0}>Vùng cuộn {row.id}</span>
            <label>Nhãn {row.id}</label>
          </>
        ),
      },
    ];
    render(
      <DataTable
        label="Bài giao"
        columns={columns}
        rows={ROWS}
        rowSize={{ padY: 10 }}
        onOpen={onOpen}
      />,
    );
    await user.click(screen.getByText("Vùng cuộn a1"));
    await user.click(screen.getByText("Nhãn a2"));
    expect(onOpen).not.toHaveBeenCalled();
  });

  it.each<[string, ReactNode]>([
    [
      "a select",
      <select key="c" data-testid="control">
        <option>Học viên</option>
      </select>,
    ],
    ["a textarea", <textarea key="c" data-testid="control" />],
    [
      "a summary",
      <details key="c">
        <summary data-testid="control">Chi tiết</summary>
      </details>,
    ],
    [
      "an editable region",
      <div key="c" data-testid="control" contentEditable suppressContentEditableWarning>
        Ghi chú
      </div>,
    ],
    [
      "a link",
      <a key="c" data-testid="control" href="#ghi-chu">
        Ghi chú
      </a>,
    ],
    ...["button", "link", "checkbox", "switch", "combobox"].map(
      (role): [string, ReactNode] => [
        `an element with the ${role} role`,
        <span key="c" data-testid="control" role={role}>
          Bật
        </span>,
      ],
    ),
  ])("stays shut on %s a cell draws", async (_, control) => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const columns: readonly DataColumn<Assignment>[] = [
      COLUMNS[0]!,
      { id: "note", header: "Ghi chú", track: "160px", cell: () => control },
    ];
    render(
      <DataTable
        label="Bài giao"
        columns={columns}
        rows={ROWS.slice(0, 1)}
        rowSize={{ padY: 10 }}
        onOpen={onOpen}
      />,
    );
    await user.click(screen.getByTestId("control"));
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("opens inside a focusable region and from an element only a script can focus", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const columns: readonly DataColumn<Assignment>[] = [
      COLUMNS[0]!,
      {
        id: "note",
        header: "Ghi chú",
        track: "160px",
        cell: (row) => <span tabIndex={-1}>Ghi chú {row.id}</span>,
      },
    ];
    render(
      <main tabIndex={-1}>
        <section aria-label="Vùng cuộn" role="tabpanel" tabIndex={0}>
          <DataTable
            label="Bài giao"
            columns={columns}
            rows={ROWS}
            rowSize={{ padY: 10 }}
            onOpen={onOpen}
          />
        </section>
      </main>,
    );
    await user.click(screen.getByText("Ghi chú a2"));
    expect(onOpen.mock.calls).toEqual([[ROWS[1]]]);
  });

  it("calls onOpen once from the button, from Enter and from a click on a cell", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    render(
      <DataTable
        label="Bài giao"
        columns={COLUMNS}
        rows={ROWS}
        rowSize={{ minHeight: 60 }}
        onOpen={onOpen}
      />,
    );
    const button = within(bodyRows()[0]!).getByRole("button", {
      name: /Mid-term Reading Mock/,
    });
    expect(button).toHaveAttribute("type", "button");
    expect(button).toHaveClass(
      "block",
      "w-full",
      "min-w-0",
      "text-left",
      "cursor-pointer",
    );
    expect(screen.queryByRole("link")).toBeNull();
    await user.click(button);
    expect(onOpen.mock.calls).toEqual([[ROWS[0]]]);
    button.focus();
    await user.keyboard("{Enter}");
    expect(onOpen.mock.calls).toEqual([[ROWS[0]], [ROWS[0]]]);
    await user.click(within(bodyRows()[1]!).getByText("Tomorrow, 08:00"));
    expect(onOpen.mock.calls).toEqual([[ROWS[0]], [ROWS[0]], [ROWS[1]]]);
  });

  it("draws plain rows when the table opens nothing", async () => {
    const user = userEvent.setup();
    renderAssignments({ plain: true });
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
    expect(bodyRows()[0]).not.toHaveClass("cursor-pointer");
    await user.click(within(bodyRows()[0]!).getByText("Today, 21:00"));
    expect(opened()).toBeNull();
  });
});

function Paged() {
  const [page, setPage] = useState(1);
  return (
    <>
      <button type="button" onClick={() => setPage(page === 1 ? 2 : 1)}>
        Đổi trang
      </button>
      <Assignments rows={page === 1 ? ROWS : PAGE_TWO} />
    </>
  );
}

describe("selection", () => {
  it("draws the header box unchecked, mixed and checked for none, some and all of the page", async () => {
    const user = userEvent.setup();
    renderAssignments();
    const all = screen.getByRole<HTMLInputElement>("checkbox", { name: SELECT_PAGE });
    expect(all).toHaveAttribute("aria-checked", "false");
    expect(all.indeterminate).toBe(false);
    await user.click(
      screen.getByRole("checkbox", { name: "Chọn Mid-term Reading Mock" }),
    );
    expect(all).toHaveAttribute("aria-checked", "mixed");
    expect(all.indeterminate).toBe(true);
    await user.click(all);
    expect(all).toHaveAttribute("aria-checked", "true");
    expect(all.checked).toBe(true);
    expect(all.indeterminate).toBe(false);
    for (const row of bodyRows()) {
      expect(within(row).getByRole<HTMLInputElement>("checkbox").checked).toBe(true);
    }
    await user.click(all);
    expect(all).toHaveAttribute("aria-checked", "false");
    expect(screen.getByTestId("chosen").textContent).toBe("");
  });

  it("marks the selected rows and no other", async () => {
    const user = userEvent.setup();
    renderAssignments();
    expect(bodyRows().map((row) => row.getAttribute("aria-selected"))).toEqual([
      "false",
      "false",
      "false",
    ]);
    await user.click(
      screen.getByRole("checkbox", { name: "Chọn Unit 4 · Listening: Directions" }),
    );
    expect(bodyRows().map((row) => row.getAttribute("aria-selected"))).toEqual([
      "false",
      "true",
      "false",
    ]);
  });

  it("keeps page 1's items while page 2 shows, and their boxes when page 1 returns", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <Paged />
      </MemoryRouter>,
    );
    await user.click(
      screen.getByRole("checkbox", { name: "Chọn Mid-term Reading Mock" }),
    );
    await user.click(
      screen.getByRole("checkbox", { name: "Chọn Vocabulary Quiz: Travel" }),
    );
    await user.click(screen.getByRole("button", { name: "Đổi trang" }));
    expect(bodyRows()).toHaveLength(1);
    expect(screen.getByRole("checkbox", { name: SELECT_PAGE })).toHaveAttribute(
      "aria-checked",
      "false",
    );
    expect(screen.getByTestId("chosen").textContent).toBe(
      "Mid-term Reading Mock | Vocabulary Quiz: Travel",
    );
    await user.click(
      screen.getByRole("checkbox", { name: "Chọn Present Perfect vs Past Simple" }),
    );
    await user.click(screen.getByRole("button", { name: "Đổi trang" }));
    expect(screen.getByTestId("chosen").textContent).toBe(
      "Mid-term Reading Mock | Vocabulary Quiz: Travel | Present Perfect vs Past Simple",
    );
    expect(bodyRows().map((row) => row.getAttribute("aria-selected"))).toEqual([
      "true",
      "false",
      "true",
    ]);
    expect(screen.getByRole("checkbox", { name: SELECT_PAGE })).toHaveAttribute(
      "aria-checked",
      "mixed",
    );
  });

  it("has no checkbox and no aria-selected without selection", () => {
    renderAssignments({ plain: true });
    expect(screen.queryByRole("checkbox")).toBeNull();
    for (const row of bodyRows()) expect(row).not.toHaveAttribute("aria-selected");
  });
});

const card = (row: Assignment): ReactNode => (
  <>
    <span>{row.title}</span>
    <span>
      {row.classes} · {row.when}
    </span>
  </>
);

describe("below 768", () => {
  it("renders a list of cards with no table, checkbox or menu, and keeps the footer", () => {
    viewport("phone");
    const { container } = renderAssignments({ card, footer: <p>1–3 trên 3</p> });
    expect(screen.queryByRole("table")).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.queryByRole("button", { name: MENU })).toBeNull();
    const list = screen.getByRole("list", { name: "Bài giao" });
    expect(within(list).getAllByRole("listitem")).toHaveLength(3);
    expect(list).toHaveClass("flex", "flex-col", "gap-2.5");
    const first = within(list).getAllByRole("link")[0]!;
    expect(first).toHaveTextContent("Mid-term Reading Mock");
    expect(first).toHaveTextContent("IELTS 6.5 Evening · Today, 21:00");
    expect(first).toHaveClass(
      "flex",
      "flex-col",
      "gap-2.5",
      "p-3.5",
      "rounded-xl",
      "border",
      "bg-card",
      "shadow-card",
      "w-full",
      "text-left",
      "leading-[normal]",
    );
    const footer = screen.getByText("1–3 trên 3");
    expect(list.compareDocumentPosition(footer)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(frame(container)).toHaveClass("flex", "flex-col", "gap-2.5");
    expect(frame(container)).not.toHaveClass("border");
    expect(titleCell).not.toHaveBeenCalled();
  });

  it("opens the row from its card", async () => {
    viewport("phone");
    const user = userEvent.setup();
    renderAssignments({ card });
    const link = screen.getByRole("link", { name: /Unit 4 · Listening: Directions/ });
    expect(link).toHaveAttribute("href", "/teacher/assignments/a2");
    await user.click(link);
    expect(opened()).toBe("/teacher/assignments/a2");
  });

  it("makes each card a button for a table that opens through onOpen", async () => {
    viewport("phone");
    const user = userEvent.setup();
    const onOpen = vi.fn();
    render(
      <DataTable
        label="Bài giao"
        columns={COLUMNS}
        rows={ROWS}
        rowSize={{ minHeight: 60 }}
        onOpen={onOpen}
        card={card}
      />,
    );
    await user.click(screen.getByRole("button", { name: /Vocabulary Quiz: Travel/ }));
    expect(onOpen.mock.calls).toEqual([[ROWS[2]]]);
  });

  it("joins the rows into one card, footer inside, when the table asks for it", () => {
    viewport("phone");
    const { container } = renderAssignments({
      card,
      cardLayout: "joined",
      footer: <p>1–3 trên 3</p>,
    });
    const joined = frame(container);
    expect(joined).toHaveClass("bg-card", "rounded-xl", "border", "overflow-hidden");
    expect(joined).not.toHaveClass("shadow-card");
    const list = screen.getByRole("list", { name: "Bài giao" });
    expect(joined).toContainElement(list);
    expect(joined).toContainElement(screen.getByText("1–3 trên 3"));
    for (const item of within(list).getAllByRole("listitem")) {
      expect(item).toHaveClass("border-t", "first:border-t-0");
    }
    const row = within(list).getAllByRole("link")[0]!;
    expect(row).toHaveClass(
      "flex",
      "w-full",
      "items-center",
      "gap-3",
      "px-3.5",
      "py-3",
      "text-base",
      "leading-normal",
      "-outline-offset-2!",
    );
    expect(row).not.toHaveClass("rounded-xl");
    expect(row).not.toHaveClass("shadow-card");
  });

  it("draws plain cards when the table opens nothing", () => {
    viewport("phone");
    renderAssignments({ plain: true, card });
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("stays a grid, checkboxes and menus included, for a table without cards", () => {
    viewport("phone");
    renderAssignments();
    expect(screen.getByRole("table", { name: "Bài giao" })).toBeInTheDocument();
    expect(screen.getAllByRole("checkbox")).toHaveLength(4);
    expect(screen.getAllByRole("button", { name: MENU })).toHaveLength(3);
  });

  it("is the table at 768 and the cards at 767", () => {
    const width = viewport(768);
    renderAssignments({ card });
    expect(screen.getByRole("table", { name: "Bài giao" })).toBeInTheDocument();
    act(() => width.resize(767));
    expect(screen.queryByRole("table")).toBeNull();
    expect(screen.getByRole("list", { name: "Bài giao" })).toBeInTheDocument();
    act(() => width.resize(768));
    expect(screen.getByRole("table", { name: "Bài giao" })).toBeInTheDocument();
  });

  it("says the list is empty in a card and keeps the footer", () => {
    viewport("phone");
    renderAssignments({
      rows: [],
      card,
      empty: "Không có bài giao nào ở đây.",
      footer: <p>0 trên 0</p>,
    });
    expect(screen.queryByRole("list")).toBeNull();
    const line = screen.getByText("Không có bài giao nào ở đây.");
    expect(line.parentElement).toHaveClass("bg-card", "rounded-xl", "border");
    expect(line.compareDocumentPosition(screen.getByText("0 trên 0"))).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it("shows no empty line on a phone while there are rows", () => {
    viewport("phone");
    renderAssignments({ card, empty: "Không có bài giao nào ở đây." });
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    expect(screen.queryByText("Không có bài giao nào ở đây.")).toBeNull();
  });

  it("says the list is empty inside the joined card", () => {
    viewport("phone");
    const { container } = renderAssignments({
      rows: [],
      card,
      cardLayout: "joined",
      empty: "Không có bài giao nào ở đây.",
    });
    const line = screen.getByText("Không có bài giao nào ở đây.");
    expect(frame(container)).toContainElement(line);
    expect(line.parentElement).toBe(frame(container));
    expect(line).not.toHaveClass("border-t");
  });
});

describe("the empty line and the footer", () => {
  it("puts the empty line in place of the rows and keeps the header", () => {
    renderAssignments({ rows: [], empty: "Không có bài giao nào ở đây." });
    const table = screen.getByRole("table", { name: "Bài giao" });
    expect(within(table).getAllByRole("row")).toHaveLength(1);
    expect(within(table).getAllByRole("rowgroup")).toHaveLength(1);
    expect(headers()).toContain("Bài giao");
    const line = screen.getByText("Không có bài giao nào ở đây.");
    expect(line).toHaveClass(
      "p-10",
      "text-center",
      "text-ui",
      "text-muted-fg",
      "leading-normal",
      "border-t",
    );
    expect(table).not.toContainElement(line);
    expect(table.compareDocumentPosition(line)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(screen.getByRole("checkbox", { name: SELECT_PAGE })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("shows no empty line while there are rows", () => {
    renderAssignments({ empty: "Không có bài giao nào ở đây." });
    expect(screen.queryByText("Không có bài giao nào ở đây.")).toBeNull();
  });

  it.each([null, false, ""] as const)(
    "draws no empty line for an empty of %j, in the grid or below 768",
    (blank) => {
      const grid = renderAssignments({ rows: [], empty: blank });
      expect(frame(grid.container).children).toHaveLength(1);
      expect(frame(grid.container).firstElementChild).toBe(screen.getByRole("table"));
      grid.unmount();
      viewport("phone");
      const stacked = renderAssignments({ rows: [], card, empty: blank });
      expect(frame(stacked.container)).toBeEmptyDOMElement();
      stacked.unmount();
      const joined = renderAssignments({
        rows: [],
        card,
        cardLayout: "joined",
        empty: blank,
      });
      expect(frame(joined.container)).toBeEmptyDOMElement();
    },
  );

  it("renders the footer after the rows, inside the card", () => {
    const { container } = renderAssignments({ footer: <p>1–3 trên 3</p> });
    const footer = screen.getByText("1–3 trên 3");
    expect(frame(container)).toContainElement(footer);
    expect(bodyRows()[2]!.compareDocumentPosition(footer)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it("renders the footer after the empty line", () => {
    renderAssignments({
      rows: [],
      empty: "Không có bài giao nào ở đây.",
      footer: <p>0 trên 0</p>,
    });
    expect(
      screen
        .getByText("Không có bài giao nào ở đây.")
        .compareDocumentPosition(screen.getByText("0 trên 0")),
    ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });
});

function Gated({ allowed }: Readonly<{ allowed: boolean }>) {
  const selection = useBulkSelection<Assignment>();
  return (
    <DataTable
      label="Bài giao"
      columns={COLUMNS}
      rows={ROWS}
      rowSize={{ minHeight: 60 }}
      selection={allowed ? selection : undefined}
      rowName={(row) => row.title}
      onOpen={allowed ? undefined : vi.fn()}
      menu={allowed ? () => <DropdownMenuItem>Nhân bản</DropdownMenuItem> : undefined}
      card={allowed ? undefined : card}
      dense={allowed ? undefined : true}
      shown={undefined}
      menuTrack={undefined}
      padX={undefined}
      cardLayout={undefined}
      framed={undefined}
    />
  );
}

describe("the props type", () => {
  const base = {
    label: "Bài giao",
    columns: COLUMNS,
    rows: ROWS,
    rowSize: { minHeight: 60 },
  } as const;

  it("takes undefined for whatever a permission or a preference may leave out", () => {
    const { unmount } = render(<Gated allowed />);
    expect(screen.getAllByRole("checkbox")).toHaveLength(4);
    expect(screen.getAllByRole("button", { name: MENU })).toHaveLength(3);
    expect(screen.getAllByRole("button")).toHaveLength(3);
    expect(screen.getAllByRole("row")[1]).toHaveStyle({
      minHeight: "60px",
      paddingLeft: "16px",
    });
    expect(templates()[0]!.endsWith(" 44px")).toBe(true);
    unmount();
    const { container } = render(<Gated allowed={false} />);
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.queryByRole("button", { name: MENU })).toBeNull();
    expect(screen.getAllByRole("button")).toHaveLength(3);
    expect(screen.getAllByRole("row")[1]).toHaveStyle({ minHeight: "44px" });
    expect(frame(container)).toHaveClass("rounded-xl", "border");
  });

  it("refuses both ways to open a row and a selection without names", () => {
    const selection = {} as ReturnType<typeof useBulkSelection<Assignment>>;
    // @ts-expect-error a row opens one way, never both
    const both: DataTableProps<Assignment> = {
      ...base,
      rowHref: () => "/teacher/assignments",
      onOpen: () => {},
    };
    // @ts-expect-error a selection needs rowName for its labels
    const unnamed: DataTableProps<Assignment> = { ...base, selection };
    const named: DataTableProps<Assignment> = {
      ...base,
      selection,
      rowName: (row) => row.title,
    };
    expect([both, unnamed, named]).toHaveLength(3);
  });
});
