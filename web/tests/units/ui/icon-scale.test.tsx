import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Flag } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableRow } from "@/components/ui/table";
import { RowMenu } from "@/components/shared/RowMenu";
import "@/lib/i18n";

const SIZE_3 = "[&>svg:not([class*='size-'])]:size-3";
const SIZE_3_5 = "[&_svg:not([class*='size-'])]:size-3.5";

describe("the icon scale (F-14)", () => {
  it("a badge sizes its icon to 12px", () => {
    render(
      <Badge variant="secondary">
        <Flag aria-hidden="true" />
        IELTS
      </Badge>,
    );
    expect(screen.getByText("IELTS")).toHaveClass(SIZE_3);
  });

  it("the 28px button carries a 14px icon; the 32px one keeps 16", () => {
    render(
      <>
        <Button size="xs">nhỏ</Button>
        <Button size="icon-xs" aria-label="thao tác">
          <Flag aria-hidden="true" />
        </Button>
        <Button size="sm">vừa</Button>
      </>,
    );
    expect(screen.getByRole("button", { name: "nhỏ" })).toHaveClass("h-7", SIZE_3_5);
    expect(screen.getByRole("button", { name: "thao tác" })).toHaveClass(
      "size-7",
      SIZE_3_5,
    );
    expect(screen.getByRole("button", { name: "vừa" })).not.toHaveClass(SIZE_3_5);
  });

  it("a bare icon in a table cell is 14px, and the row kebab is the plain icon-xs button", () => {
    render(
      <Table>
        <TableBody>
          <TableRow>
            <TableCell>
              <Flag aria-hidden="true" />
            </TableCell>
            <TableCell>
              <RowMenu>
                <p>menu</p>
              </RowMenu>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );
    const [cell] = screen.getAllByRole("cell");
    expect(cell!.className).toContain(":size-3.5");
    expect(cell!.className).toContain(":not(button_svg)");
    expect(screen.getByRole("button", { name: "Thao tác" })).toHaveClass("size-7");
  });
});
