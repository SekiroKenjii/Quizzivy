import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Kbd } from "@/components/ui/kbd";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

const SCOPE = "in-data-[scale=deck]:";

function classesOf(element: ReactElement) {
  const { container } = render(<div data-scale="deck">{element}</div>);
  const classes = (
    container.firstElementChild!.firstElementChild as HTMLElement
  ).className.split(" ");
  return {
    unscoped: classes.filter((c) => !c.startsWith(SCOPE)),
    scoped: classes
      .filter((c) => c.startsWith(SCOPE))
      .map((c) => c.slice(SCOPE.length)),
  };
}

describe("deck geometry applies only on a deck surface", () => {
  it.each([
    ["default", ["h-9", "px-4"], ["px-3.5", "text-ui"]],
    ["xs", ["h-7", "px-2"], ["h-6.5"]],
    ["sm", ["h-8", "px-3"], ["h-7.5", "rounded-seg"]],
    ["lg", ["h-11", "px-6"], ["h-10.5", "rounded-lg", "font-semibold"]],
    ["icon", ["size-9"], ["size-8.5"]],
  ] as const)(
    "keeps today's %s button outside and adds the deck's inside",
    (size, today, deck) => {
      const { unscoped, scoped } = classesOf(<Button size={size}>Lưu</Button>);
      for (const c of today) expect(unscoped).toContain(c);
      for (const c of deck) expect(scoped).toContain(c);
    },
  );

  it("gives the new deck-only sizes their geometry everywhere", () => {
    expect(classesOf(<Button size="md">Lưu</Button>).unscoped).toEqual(
      expect.arrayContaining(["h-9.5", "rounded-ctl", "text-ui"]),
    );
    expect(classesOf(<Button size="xl">Đăng nhập</Button>).unscoped).toEqual(
      expect.arrayContaining(["h-11.5", "rounded-lg", "font-semibold"]),
    );
    expect(classesOf(<Button size="icon-xl" aria-label="Menu" />).unscoped).toContain(
      "size-10",
    );
  });

  it("keeps today's field outside and the deck's 38/42/46px fields inside", () => {
    const plain = classesOf(<Input aria-label="Email" />);
    expect(plain.unscoped).toContain("h-9");
    expect(plain.scoped).toContain("h-9.5");
    expect(classesOf(<Input aria-label="Email" size="lg" />).scoped).toContain(
      "h-10.5",
    );
    expect(classesOf(<Input aria-label="Email" size="xl" />).scoped).toContain(
      "h-11.5",
    );
    expect(classesOf(<Textarea aria-label="Ghi chú" />).scoped).toContain("lg:text-ui");
    expect(classesOf(<Label>Email</Label>).scoped).toContain("gap-1.5");
  });

  it("never passes the deck size to the input element", () => {
    const { container } = render(<Input aria-label="Email" size="xl" />);
    expect(container.querySelector("input")).not.toHaveAttribute("size");
  });

  it("turns a status badge into the deck's soft pill only on a deck surface", () => {
    const { unscoped, scoped } = classesOf(<Badge variant="success">Đã nộp</Badge>);
    expect(unscoped).toContain("rounded-sm");
    expect(scoped).toEqual(
      expect.arrayContaining(["rounded-full", "bg-success-soft", "text-success-ink"]),
    );
    expect(classesOf(<Badge variant="count-brand">3</Badge>).unscoped).toContain(
      "bg-brand",
    );
  });

  it("draws the status dot only when asked", () => {
    const { container } = render(
      <Badge variant="info" dot>
        Đang làm
      </Badge>,
    );
    expect(container.querySelector('[aria-hidden="true"]')).toHaveClass(
      "size-1.5",
      "rounded-full",
    );
    expect(
      render(<Badge variant="info">Đang làm</Badge>).container.querySelector(
        '[aria-hidden="true"]',
      ),
    ).toBeNull();
  });

  it("gives cards and key caps the deck's radius and type only on a deck surface", () => {
    expect(classesOf(<Card />).scoped).toEqual(
      expect.arrayContaining(["shadow-card", "rounded-xl"]),
    );
    expect(classesOf(<Kbd>K</Kbd>).scoped).toContain("text-caption");
  });
});
