import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
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

function classes(element: Element) {
  return element.className.split(" ").filter(Boolean);
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

describe("the table primitive", () => {
  it("draws the deck's classes on every part, class for class", () => {
    const part = parts(render(<Sample />).container);
    expect(classes(part.container)).toEqual(["relative", "w-full", "overflow-x-auto"]);
    expect(classes(part.table)).toEqual([
      "w-full",
      "caption-bottom",
      "text-ui",
      "leading-normal",
    ]);
    expect(classes(part.header)).toEqual(["[&_tr]:border-b-0"]);
    expect(classes(part.body)).toEqual(["[&_tr:last-child]:border-0"]);
    expect(classes(part.footer)).toEqual([
      "bg-muted/50",
      "border-t",
      "font-medium",
      "[&>tr]:last:border-b-0",
    ]);
    expect(classes(part.row)).toEqual([
      "has-aria-expanded:bg-muted/50",
      "data-[state=selected]:bg-muted",
      "transition-colors",
      "hover:bg-muted",
      "border-b-0",
    ]);
    expect(classes(part.head)).toEqual([
      "text-muted-foreground",
      "text-left",
      "align-middle",
      "font-medium",
      "whitespace-nowrap",
      "[&:has([role=checkbox])]:pr-0",
      "[&>[role=checkbox]]:translate-y-[2px]",
      "bg-muted",
      "text-meta",
      "h-10",
      "px-4",
      "leading-normal",
    ]);
    expect(classes(part.cell)).toEqual([
      "align-middle",
      "whitespace-nowrap",
      "[&_svg:not([class*='size-']):not([data-slot=badge]_svg):not(button_svg):not(a_svg)]:size-3.5",
      "[&:has([role=checkbox])]:pr-0",
      "[&>[role=checkbox]]:translate-y-[2px]",
      "h-auto",
      "border-t",
      "px-4",
      "py-2.5",
    ]);
    expect(classes(part.caption)).toEqual(["text-muted-foreground", "mt-4", "text-sm"]);
  });

  it("lets a caller's class replace the deck's", () => {
    render(
      <table>
        <tbody>
          <tr>
            <TableCell className="h-12 px-2">Trần Minh Anh</TableCell>
          </tr>
        </tbody>
      </table>,
    );
    const cell = classes(screen.getByRole("cell"));
    expect(cell).toEqual(
      expect.arrayContaining(["h-12", "px-2", "border-t", "py-2.5"]),
    );
    expect(cell).not.toContain("h-auto");
    expect(cell).not.toContain("px-4");
  });

  it("gives a header cell the deck's 40px, 12.5px, muted text on muted and 16px padding", () => {
    const head = classes(parts(render(<Sample />).container).head);
    expect(head).toEqual(
      expect.arrayContaining([
        "bg-muted",
        "text-meta",
        "h-10",
        "px-4",
        "leading-normal",
        "text-muted-foreground",
        "font-medium",
      ]),
    );
    expect(head).not.toContain("h-9");
    expect(head).not.toContain("px-3");
  });

  it("gives a body cell 10px 16px padding, a top border and no fixed height", () => {
    const cell = classes(parts(render(<Sample />).container).cell);
    expect(cell).toEqual(
      expect.arrayContaining(["h-auto", "border-t", "px-4", "py-2.5"]),
    );
    expect(cell).not.toContain("h-10");
    expect(cell).not.toContain("px-3");
  });

  it("sets the body text to 13.5px on the deck's line, drops the rows' bottom borders and hovers on muted", () => {
    const part = parts(render(<Sample />).container);
    expect(classes(part.table)).toEqual(
      expect.arrayContaining(["text-ui", "leading-normal"]),
    );
    expect(classes(part.table)).not.toContain("text-[0.8125rem]");
    expect(classes(part.header)).not.toContain("[&_tr]:border-b");
    expect(classes(part.row)).toEqual(
      expect.arrayContaining(["hover:bg-muted", "border-b-0"]),
    );
    expect(classes(part.row)).not.toContain("hover:bg-muted/50");
    expect(classes(part.row)).not.toContain("border-b");
  });

  it("writes no class scoped to a deck surface on any part", () => {
    const part = parts(render(<Sample />).container);
    for (const element of Object.values(part)) {
      expect(classes(element).filter((name) => name.includes("scale=deck"))).toEqual(
        [],
      );
    }
  });
});
