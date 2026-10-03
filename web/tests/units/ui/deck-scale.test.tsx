import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DeckScale } from "@/components/ui/deck-scale";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Kbd } from "@/components/ui/kbd";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Segmented } from "@/components/ui/segmented";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { PasswordInput } from "@/components/shared/PasswordInput";
import "@/lib/i18n";

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
    expect(unscoped).toEqual(
      expect.arrayContaining(["rounded-sm", "text-success-ink"]),
    );
    expect(scoped).toEqual(expect.arrayContaining(["rounded-full", "bg-success-soft"]));
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

  it("keeps the large password field's show and hide button at the deck's 36px", () => {
    const { container } = render(
      <div data-scale="deck">
        <PasswordInput id="password" size="xl" />
      </div>,
    );
    const scoped = container
      .querySelector("button")!
      .className.split(" ")
      .filter((c) => c.startsWith(SCOPE))
      .map((c) => c.slice(SCOPE.length));
    expect(scoped).toContain("size-9");
    expect(scoped).not.toContain("size-8.5");
  });
});

function split(element: Element) {
  const classes = element.className.split(" ");
  return {
    unscoped: classes.filter((c) => !c.startsWith(SCOPE)),
    scoped: classes
      .filter((c) => c.startsWith(SCOPE))
      .map((c) => c.slice(SCOPE.length)),
  };
}

const OWN_SCOPE = "data-[scale=deck]:";

describe("the three controls the student screens use first", () => {
  it("keeps today's segmented buttons outside and the deck's 30px inside", () => {
    render(
      <Segmented
        label="Ngôn ngữ"
        value="vi"
        onChange={() => {}}
        options={[
          { value: "vi", label: "Tiếng Việt" },
          { value: "en", label: "English" },
        ]}
      />,
    );
    const track = split(screen.getByRole("group", { name: "Ngôn ngữ" }));
    expect(track.unscoped).toEqual(expect.arrayContaining(["rounded-lg", "gap-0.5"]));
    expect(track.scoped).toEqual(["rounded-ctl"]);

    const on = split(screen.getByRole("button", { name: "Tiếng Việt" }));
    expect(on.unscoped).toEqual(
      expect.arrayContaining(["h-7", "rounded-md", "bg-background", "shadow-card"]),
    );
    expect(on.scoped).toEqual(
      expect.arrayContaining([
        "h-7.5",
        "rounded-seg",
        "bg-card",
        "ring-1",
        "ring-border",
      ]),
    );

    const off = split(screen.getByRole("button", { name: "English" }));
    expect(off.unscoped).toContain("text-muted-foreground");
    expect(off.scoped).not.toContain("bg-card");
    expect(off.scoped).not.toContain("ring-1");
  });

  it("draws the deck's 32px row for a page's sections everywhere", () => {
    render(
      <Segmented
        label="Mục"
        size="lg"
        value="a"
        onChange={() => {}}
        options={[{ value: "a", label: "Hồ sơ" }]}
      />,
    );
    const button = split(screen.getByRole("button", { name: "Hồ sơ" }));
    expect(button.unscoped).toContain("h-8");
    expect(button.unscoped).not.toContain("h-7");
    expect(button.scoped).toContain("h-8");
    expect(button.scoped).not.toContain("h-7.5");
  });

  it("keeps today's switch outside and the deck's 38x22 in the primary colour inside", () => {
    render(<Switch aria-label="Âm thanh" />);
    const root = screen.getByRole("switch", { name: "Âm thanh" });
    const track = split(root);
    expect(track.unscoped).toEqual(
      expect.arrayContaining([
        "h-[1.15rem]",
        "w-8",
        "data-[state=checked]:bg-foreground",
        "data-[state=unchecked]:bg-foreground/22",
      ]),
    );
    expect(track.scoped).toEqual(
      expect.arrayContaining([
        "h-5.5",
        "w-9.5",
        "data-[state=checked]:bg-primary",
        "data-[state=unchecked]:bg-border",
      ]),
    );
    const thumb = split(root.firstElementChild!);
    expect(thumb.unscoped).toEqual(
      expect.arrayContaining([
        "size-[0.9rem]",
        "data-[state=checked]:translate-x-[0.975rem]",
      ]),
    );
    expect(thumb.scoped).toEqual(
      expect.arrayContaining([
        "size-4.5",
        "translate-x-0.5",
        "data-[state=checked]:translate-x-4.5",
        "bg-switch-thumb",
      ]),
    );
  });

  it.each([
    ["sm", ["h-5", "w-9"], ["size-4", "data-[state=checked]:translate-x-4.5"]],
    ["lg", ["h-6", "w-10.5"], ["size-5", "data-[state=checked]:translate-x-5"]],
  ] as const)(
    "gives the %s switch the deck's geometry everywhere",
    (size, track, thumb) => {
      render(<Switch size={size} aria-label="Chữ to" />);
      const root = screen.getByRole("switch", { name: "Chữ to" });
      const rootClasses = split(root);
      const thumbClasses = split(root.firstElementChild!);
      for (const c of track) {
        expect(rootClasses.unscoped).toContain(c);
        expect(rootClasses.scoped).toContain(c);
      }
      for (const c of thumb) {
        expect(thumbClasses.unscoped).toContain(c);
        expect(thumbClasses.scoped).toContain(c);
      }
      expect(rootClasses.unscoped).not.toContain("w-8");
      expect(rootClasses.scoped).not.toContain("w-9.5");
      expect(thumbClasses.unscoped).not.toContain("size-[0.9rem]");
    },
  );

  it("keeps today's select trigger outside and the deck's 38px inside, with a 42px size", () => {
    render(
      <>
        <Select value="45">
          <SelectTrigger aria-label="Thời lượng">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="45">45 phút</SelectItem>
          </SelectContent>
        </Select>
        <Select value="vi">
          <SelectTrigger aria-label="Ngôn ngữ" size="lg">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="vi">Tiếng Việt</SelectItem>
          </SelectContent>
        </Select>
      </>,
    );
    const standard = split(screen.getByRole("combobox", { name: "Thời lượng" }));
    expect(standard.unscoped).toEqual(
      expect.arrayContaining([
        "data-[size=default]:h-9",
        "data-[size=sm]:h-8",
        "shadow-xs",
      ]),
    );
    expect(standard.scoped).toEqual(
      expect.arrayContaining([
        "data-[size=default]:h-9.5",
        "shadow-none",
        "bg-bg",
        "text-ui",
      ]),
    );
    const large = screen.getByRole("combobox", { name: "Ngôn ngữ" });
    expect(large.dataset["size"]).toBe("lg");
    expect(split(large).unscoped).toEqual(
      expect.arrayContaining([
        "data-[size=lg]:h-10.5",
        "data-[size=lg]:rounded-ctl",
        "data-[size=lg]:text-body",
      ]),
    );
  });

  it("draws the deck's list only when the list is on a deck surface", () => {
    render(
      <DeckScale>
        <OpenSelect />
      </DeckScale>,
    );
    const content = screen
      .getByRole("listbox")
      .closest("[data-slot='select-content']")!;
    const own = content.className
      .split(" ")
      .filter((c) => c.startsWith(OWN_SCOPE))
      .map((c) => c.slice(OWN_SCOPE.length));
    expect(own).toEqual(
      expect.arrayContaining([
        "bg-card",
        "shadow-float",
        "rounded-lg",
        "text-ui",
        "z-(--z-popover)",
      ]),
    );
    expect(content.className.split(" ")).toEqual(
      expect.arrayContaining(["bg-popover", "shadow-md", "z-50", "rounded-md"]),
    );

    const option = split(screen.getByRole("option", { name: "45 phút" }));
    expect(option.unscoped).toEqual(
      expect.arrayContaining(["pr-8", "pl-2", "focus:bg-accent"]),
    );
    expect(option.scoped).toEqual(
      expect.arrayContaining([
        "min-h-8",
        "pl-7.5",
        "pr-2.5",
        "focus:bg-hover",
        "data-[state=checked]:font-medium",
      ]),
    );
    const indicator = split(
      screen
        .getByRole("option", { name: "45 phút" })
        .querySelector("[data-slot='select-item-indicator']")!,
    );
    expect(indicator.unscoped).toContain("right-2");
    expect(indicator.scoped).toEqual(expect.arrayContaining(["left-2", "right-auto"]));
  });
});

function OpenSelect() {
  return (
    <Select open value="45">
      <SelectTrigger aria-label="Thời lượng">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="45">45 phút</SelectItem>
      </SelectContent>
    </Select>
  );
}

function OpenMenu() {
  return (
    <DropdownMenu open>
      <DropdownMenuTrigger>Tài khoản</DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem>Cài đặt</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

describe("content that portals out of a deck surface stays on it", () => {
  it("marks the surface itself", () => {
    const { container } = render(<DeckScale className="shell">nội dung</DeckScale>);
    const root = container.firstElementChild as HTMLElement;
    expect(root.dataset["scale"]).toBe("deck");
    expect(root.className).toBe("shell");
  });

  it("carries the scale onto a select's list, which renders under the body", () => {
    const { container } = render(
      <DeckScale>
        <OpenSelect />
      </DeckScale>,
    );
    const list = screen.getByRole("listbox");
    expect(container.contains(list)).toBe(false);
    expect(list.closest("[data-scale='deck']")).not.toBeNull();
  });

  it("carries the scale onto a menu, which renders under the body", () => {
    const { container } = render(
      <DeckScale>
        <OpenMenu />
      </DeckScale>,
    );
    const menu = screen.getByRole("menu");
    expect(container.contains(menu)).toBe(false);
    expect(menu.dataset["scale"]).toBe("deck");
  });

  it("leaves a select's list unmarked off a deck surface", () => {
    render(<OpenSelect />);
    expect(screen.getByRole("listbox").closest("[data-scale]")).toBeNull();
  });

  it("leaves a menu unmarked off a deck surface", () => {
    render(<OpenMenu />);
    expect(screen.getByRole("menu").closest("[data-scale]")).toBeNull();
  });
});
