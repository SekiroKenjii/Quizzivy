import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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

function classesOf(element: ReactElement) {
  const { container } = render(element);
  return classList(container.firstElementChild!);
}

function classList(element: Element) {
  return element.className.split(" ");
}

describe("the primitives draw the deck's geometry with no ancestor to opt in", () => {
  it.each([
    ["default", ["h-9", "px-3.5", "text-ui"], ["px-4"]],
    ["xs", ["h-6.5", "px-2"], ["h-7"]],
    ["sm", ["h-7.5", "px-3", "rounded-seg"], ["h-8", "rounded-md"]],
    ["lg", ["h-10.5", "px-4", "rounded-lg", "font-semibold"], ["h-11", "px-6"]],
    ["icon", ["size-8.5"], ["size-9"]],
  ] as const)("draws the deck's %s button", (size, deck, replaced) => {
    const classes = classesOf(<Button size={size}>Lưu</Button>);
    for (const c of deck) expect(classes).toContain(c);
    for (const c of replaced) expect(classes).not.toContain(c);
  });

  it("gives the deck-only sizes their geometry", () => {
    expect(classesOf(<Button size="md">Lưu</Button>)).toEqual(
      expect.arrayContaining(["h-9.5", "rounded-ctl", "text-ui"]),
    );
    expect(classesOf(<Button size="xl">Đăng nhập</Button>)).toEqual(
      expect.arrayContaining(["h-11.5", "rounded-lg", "font-semibold"]),
    );
    expect(classesOf(<Button size="icon-xl" aria-label="Menu" />)).toContain("size-10");
  });

  it("draws the deck's 38/42/46px fields", () => {
    const plain = classesOf(<Input aria-label="Email" />);
    expect(plain).toContain("h-9.5");
    expect(plain).not.toContain("h-9");
    expect(classesOf(<Input aria-label="Email" size="lg" />)).toContain("h-10.5");
    expect(classesOf(<Input aria-label="Email" size="xl" />)).toContain("h-11.5");
    const note = classesOf(<Textarea aria-label="Ghi chú" />);
    expect(note).toContain("lg:text-ui");
    expect(note).not.toContain("lg:text-sm");
    const label = classesOf(<Label>Email</Label>);
    expect(label).toContain("gap-1.5");
    expect(label).not.toContain("gap-2");
  });

  it("never passes the deck size to the input element", () => {
    const { container } = render(<Input aria-label="Email" size="xl" />);
    expect(container.querySelector("input")).not.toHaveAttribute("size");
  });

  it("draws a status badge as the deck's soft pill", () => {
    const classes = classesOf(<Badge variant="success">Đã nộp</Badge>);
    expect(classes).toEqual(
      expect.arrayContaining(["rounded-full", "bg-success-soft", "text-success-ink"]),
    );
    expect(classes).not.toContain("rounded-sm");
    expect(classesOf(<Badge variant="count-brand">3</Badge>)).toContain("bg-brand");
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

  it("gives cards and key caps the deck's radius and type", () => {
    const card = classesOf(<Card />);
    expect(card).toEqual(expect.arrayContaining(["shadow-card", "rounded-xl"]));
    expect(card).not.toContain("shadow-xs");
    expect(card).not.toContain("rounded-lg");
    const key = classesOf(<Kbd>K</Kbd>);
    expect(key).toContain("text-caption");
    expect(key).not.toContain("text-[0.6875rem]");
    expect(key).not.toContain("font-mono");
  });

  it("keeps the large password field's show and hide button at the deck's 36px", () => {
    const { container } = render(<PasswordInput id="password" size="xl" />);
    const button = classList(container.querySelector("button")!);
    expect(button).toContain("size-9");
    expect(button).not.toContain("size-8.5");
    const field = classList(container.querySelector("input")!);
    expect(field).toEqual(expect.arrayContaining(["h-11.5", "px-3.5"]));
    expect(field).not.toContain("h-11");
    expect(field).not.toContain("pr-11.5");
  });

  it("centres the 42px password field's show and hide button at the deck's 34px", () => {
    const { container } = render(<PasswordInput id="password" size="lg" />);
    const button = classList(container.querySelector("button")!);
    expect(button).toEqual(
      expect.arrayContaining(["top-1/2", "-translate-y-1/2", "right-1", "size-8.5"]),
    );
    expect(button).not.toContain("inset-y-0");
    expect(button).not.toContain("h-11");
    const field = classList(container.querySelector("input")!);
    expect(field).toContain("h-10.5");
    expect(field).not.toContain("h-11");
    expect(field).toContain("pr-11");
    expect(field).not.toContain("pr-12");
  });

  it("keeps the default password field at the deck's 38px with its 34px button at the edge", () => {
    const { container } = render(<PasswordInput id="password" />);
    const button = classList(container.querySelector("button")!);
    expect(button).toEqual(expect.arrayContaining(["inset-y-0", "right-0", "size-8.5"]));
    expect(button).not.toContain("h-11");
    expect(button).not.toContain("w-11");
    expect(button).not.toContain("top-1/2");
    const field = classList(container.querySelector("input")!);
    expect(field).toEqual(expect.arrayContaining(["h-9.5", "px-3"]));
    expect(field).not.toContain("pr-12");
    expect(field).not.toContain("h-11");
  });
});

describe("the three controls the student screens use first", () => {
  it("draws the deck's 30px segmented buttons", () => {
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
    const track = classList(screen.getByRole("group", { name: "Ngôn ngữ" }));
    expect(track).toEqual(expect.arrayContaining(["rounded-ctl", "gap-0.5"]));
    expect(track).not.toContain("rounded-lg");

    const on = classList(screen.getByRole("button", { name: "Tiếng Việt" }));
    expect(on).toEqual(
      expect.arrayContaining([
        "h-7.5",
        "rounded-seg",
        "bg-card",
        "shadow-card",
        "ring-1",
        "ring-border",
      ]),
    );
    expect(on).not.toContain("h-7");
    expect(on).not.toContain("rounded-md");
    expect(on).not.toContain("bg-background");

    const off = classList(screen.getByRole("button", { name: "English" }));
    expect(off).toContain("text-muted-fg");
    expect(off).not.toContain("text-muted-foreground");
    expect(off).not.toContain("bg-card");
    expect(off).not.toContain("ring-1");
  });

  it("draws the deck's 32px row for a page's sections", () => {
    render(
      <Segmented
        label="Mục"
        size="lg"
        value="a"
        onChange={() => {}}
        options={[{ value: "a", label: "Hồ sơ" }]}
      />,
    );
    const button = classList(screen.getByRole("button", { name: "Hồ sơ" }));
    expect(button).toContain("h-8");
    expect(button).not.toContain("h-7");
    expect(button).not.toContain("h-7.5");
  });

  it("draws the deck's 38x22 switch in the primary colour", () => {
    render(<Switch aria-label="Âm thanh" />);
    const root = screen.getByRole("switch", { name: "Âm thanh" });
    const track = classList(root);
    expect(track).toEqual(
      expect.arrayContaining([
        "h-5.5",
        "w-9.5",
        "data-[state=checked]:bg-primary",
        "data-[state=unchecked]:bg-border",
      ]),
    );
    for (const c of [
      "h-[1.15rem]",
      "w-8",
      "data-[state=checked]:bg-foreground",
      "data-[state=unchecked]:bg-foreground/22",
    ]) {
      expect(track).not.toContain(c);
    }
    const thumb = classList(root.firstElementChild!);
    expect(thumb).toEqual(
      expect.arrayContaining([
        "size-4.5",
        "translate-x-0.5",
        "data-[state=checked]:translate-x-4.5",
        "bg-switch-thumb",
        "shadow-thumb",
      ]),
    );
    for (const c of [
      "size-[0.9rem]",
      "data-[state=checked]:translate-x-[0.975rem]",
      "shadow-sm",
    ]) {
      expect(thumb).not.toContain(c);
    }
  });

  it.each([
    ["sm", ["h-5", "w-9"], ["size-4", "data-[state=checked]:translate-x-4.5"]],
    ["lg", ["h-6", "w-10.5"], ["size-5", "data-[state=checked]:translate-x-5"]],
  ] as const)("gives the %s switch the deck's geometry", (size, track, thumb) => {
    render(<Switch size={size} aria-label="Chữ to" />);
    const root = screen.getByRole("switch", { name: "Chữ to" });
    const rootClasses = classList(root);
    const thumbClasses = classList(root.firstElementChild!);
    for (const c of track) expect(rootClasses).toContain(c);
    for (const c of thumb) expect(thumbClasses).toContain(c);
    expect(rootClasses).not.toContain("w-8");
    expect(rootClasses).not.toContain("w-9.5");
    expect(thumbClasses).not.toContain("size-[0.9rem]");
  });

  it("draws the deck's 38px select trigger, with a 42px size", () => {
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
    const standard = classList(screen.getByRole("combobox", { name: "Thời lượng" }));
    expect(standard).toEqual(
      expect.arrayContaining([
        "data-[size=default]:h-9.5",
        "data-[size=sm]:h-8",
        "shadow-none",
        "bg-bg",
        "text-ui",
      ]),
    );
    expect(standard).not.toContain("data-[size=default]:h-9");
    expect(standard).not.toContain("shadow-xs");
    const large = screen.getByRole("combobox", { name: "Ngôn ngữ" });
    expect(large.dataset["size"]).toBe("lg");
    expect(classList(large)).toEqual(
      expect.arrayContaining([
        "data-[size=lg]:h-10.5",
        "data-[size=lg]:rounded-ctl",
        "data-[size=lg]:text-body",
      ]),
    );
  });

  it("draws the deck's list", () => {
    render(<OpenSelect />);
    const content = classList(
      screen.getByRole("listbox").closest("[data-slot='select-content']")!,
    );
    expect(content).toEqual(
      expect.arrayContaining([
        "bg-card",
        "shadow-float",
        "rounded-lg",
        "text-ui",
        "z-(--z-popover)",
      ]),
    );
    for (const c of ["bg-popover", "shadow-md", "z-50", "rounded-md"]) {
      expect(content).not.toContain(c);
    }

    const option = classList(screen.getByRole("option", { name: "45 phút" }));
    expect(option).toEqual(
      expect.arrayContaining([
        "min-h-8",
        "pl-7.5",
        "pr-2.5",
        "focus:bg-hover",
        "data-[state=checked]:font-medium",
      ]),
    );
    for (const c of ["pr-8", "pl-2", "focus:bg-accent"]) {
      expect(option).not.toContain(c);
    }
    const indicator = classList(
      screen
        .getByRole("option", { name: "45 phút" })
        .querySelector("[data-slot='select-item-indicator']")!,
    );
    expect(indicator).toEqual(expect.arrayContaining(["left-2", "right-auto"]));
    expect(indicator).not.toContain("right-2");
  });

  it("draws the select's tick at the deck's size and ink", () => {
    render(<OpenSelect />);
    const tick = screen
      .getByRole("option", { name: "45 phút" })
      .querySelector("[data-slot='select-item-indicator'] svg")!;
    expect(tick).toHaveClass("size-3.5", "text-fg");
    expect(tick).not.toHaveClass("size-4");
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
