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

  it("the 26px and 30px buttons and the 24px icon button carry a 14px icon", () => {
    render(
      <>
        <Button size="xs">nhỏ</Button>
        <Button size="icon-xs" aria-label="thao tác">
          <Flag aria-hidden="true" />
        </Button>
        <Button size="sm">vừa</Button>
      </>,
    );
    expect(screen.getByRole("button", { name: "nhỏ" })).toHaveClass("h-6.5", SIZE_3_5);
    expect(screen.getByRole("button", { name: "thao tác" })).toHaveClass(
      "size-6",
      SIZE_3_5,
    );
    expect(screen.getByRole("button", { name: "vừa" })).toHaveClass("h-7.5", SIZE_3_5);
  });

  it("a bare icon in a table cell is 14px, and the row kebab is the deck's 30px button with a 16px icon", () => {
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
    const kebab = screen.getByRole("button", { name: "Thao tác" });
    expect(kebab).toHaveClass("size-7.5", "[&_svg:not([class*='size-'])]:size-4");
    expect(kebab).not.toHaveClass("size-6");
    expect(kebab).not.toHaveClass(SIZE_3_5);
  });
});
