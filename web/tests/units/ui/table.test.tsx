import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DeckScale } from "@/components/ui/deck-scale";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const SCOPE = "in-data-[scale=deck]:";

function Sample() {
  return (
    <Table>
      <TableCaption>Danh sách học viên</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Học viên</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow>
          <TableCell>Trần Minh Anh</TableCell>
        </TableRow>
      </TableBody>
      <TableFooter>
        <TableRow>
          <TableCell>Tổng</TableCell>
        </TableRow>
      </TableFooter>
    </Table>
  );
}

function split(element: Element) {
  const classes = element.className.split(" ").filter(Boolean);
  return {
    today: classes.filter((name) => !name.includes("[scale=deck]")),
    deck: classes
      .filter((name) => name.startsWith(SCOPE))
      .map((name) => name.slice(SCOPE.length)),
    other: classes.filter(
      (name) => name.includes("[scale=deck]") && !name.startsWith(SCOPE),
    ),
  };
}

function parts(container: HTMLElement) {
  const slot = (name: string) => container.querySelector(`[data-slot="${name}"]`)!;
  return {
    container: slot("table-container"),
    table: slot("table"),
    header: slot("table-header"),
    body: slot("table-body"),
    footer: slot("table-footer"),
    row: slot("table-row"),
    head: slot("table-head"),
    cell: slot("table-cell"),
    caption: slot("table-caption"),
  };
}

describe("the table primitive outside deck scale", () => {
  it("keeps today's classes on every part, class for class", () => {
    const part = parts(render(<Sample />).container);
    expect(split(part.container).today).toEqual([
      "relative",
      "w-full",
      "overflow-x-auto",
    ]);
    expect(split(part.table).today).toEqual([
      "w-full",
      "caption-bottom",
      "text-[0.8125rem]",
    ]);
    expect(split(part.header).today).toEqual(["[&_tr]:border-b"]);
    expect(split(part.body).today).toEqual(["[&_tr:last-child]:border-0"]);
    expect(split(part.footer).today).toEqual([
      "bg-muted/50",
      "border-t",
      "font-medium",
      "[&>tr]:last:border-b-0",
    ]);
    expect(split(part.row).today).toEqual([
      "hover:bg-muted/50",
      "has-aria-expanded:bg-muted/50",
      "data-[state=selected]:bg-muted",
      "border-b",
      "transition-colors",
    ]);
    expect(split(part.head).today).toEqual([
      "text-muted-foreground",
      "h-9",
      "px-3",
      "text-left",
      "align-middle",
      "font-medium",
      "whitespace-nowrap",
      "[&:has([role=checkbox])]:pr-0",
      "[&>[role=checkbox]]:translate-y-[2px]",
    ]);
    expect(split(part.cell).today).toEqual([
      "h-10",
      "px-3",
      "align-middle",
      "whitespace-nowrap",
      "[&_svg:not([class*='size-']):not([data-slot=badge]_svg):not(button_svg):not(a_svg)]:size-3.5",
      "[&:has([role=checkbox])]:pr-0",
      "[&>[role=checkbox]]:translate-y-[2px]",
    ]);
    expect(split(part.caption).today).toEqual([
      "text-muted-foreground",
      "mt-4",
      "text-sm",
    ]);
  });

  it("lets a caller's class replace a plain one and leaves the deck's alone", () => {
    render(
      <table>
        <tbody>
          <tr>
            <TableCell className="h-12 px-2">Trần Minh Anh</TableCell>
          </tr>
        </tbody>
      </table>,
    );
    const { today, deck } = split(screen.getByRole("cell"));
    expect(today).toContain("h-12");
    expect(today).toContain("px-2");
    expect(today).not.toContain("h-10");
    expect(today).not.toContain("px-3");
    expect(deck).toEqual(["h-auto", "border-t", "px-4", "py-2.5"]);
  });
});

describe("the table primitive on a deck surface", () => {
  it("renders inside the element that carries the scale, never as one", () => {
    const { container } = render(
      <DeckScale>
        <Sample />
      </DeckScale>,
    );
    const table = screen.getByRole("table");
    expect(table.closest('[data-scale="deck"]')).toBe(container.firstElementChild);
    expect(container.querySelectorAll("[data-scale]")).toHaveLength(1);
  });

  it("gives a header cell the deck's 40px, 12.5px, muted text on muted and 16px padding", () => {
    const part = parts(
      render(
        <DeckScale>
          <Sample />
        </DeckScale>,
      ).container,
    );
    expect(split(part.head).deck).toEqual([
      "bg-muted",
      "text-meta",
      "h-10",
      "px-4",
      "leading-normal",
    ]);
    expect(split(part.head).today).toEqual(
      expect.arrayContaining(["text-muted-foreground", "font-medium"]),
    );
  });

  it("gives a body cell 10px 16px padding, a top border and no fixed height", () => {
    const part = parts(
      render(
        <DeckScale>
          <Sample />
        </DeckScale>,
      ).container,
    );
    expect(split(part.cell).deck).toEqual(["h-auto", "border-t", "px-4", "py-2.5"]);
  });

  it("sets the body text to 13.5px on the deck's line, drops the rows' bottom borders and hovers on muted", () => {
    const part = parts(
      render(
        <DeckScale>
          <Sample />
        </DeckScale>,
      ).container,
    );
    expect(split(part.table).deck).toEqual(["text-ui", "leading-normal"]);
    expect(split(part.header).deck).toEqual(["[&_tr]:border-b-0"]);
    expect(split(part.row).deck).toEqual(["hover:bg-muted", "border-b-0"]);
    expect(split(part.body).deck).toEqual([]);
    expect(split(part.footer).deck).toEqual([]);
    expect(split(part.caption).deck).toEqual([]);
    expect(split(part.container).deck).toEqual([]);
  });

  it("writes every deck class for a descendant of the scale, none for the element that carries it", () => {
    const part = parts(render(<Sample />).container);
    for (const element of Object.values(part)) {
      expect(split(element).other).toEqual([]);
    }
  });
});
