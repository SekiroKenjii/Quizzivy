import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { Avatar } from "@/components/ui/avatar";

function initialsOf(name: string) {
  const { container } = render(<Avatar name={name} />);
  return container.textContent;
}

describe("the avatar's initials", () => {
  it("takes the given name, then the family name, as the deck does", () => {
    expect(initialsOf("Hoàng Thương")).toBe("TH");
    expect(initialsOf("Nguyễn Gia Bảo")).toBe("BN");
    expect(initialsOf("Phan Minh Đức")).toBe("ĐP");
    expect(initialsOf("Trần Quang")).toBe("QT");
  });

  it("uppercases Vietnamese letters without losing their diacritics", () => {
    expect(initialsOf("Đặng Thu Hà")).toBe("HĐ");
    expect(initialsOf("Ưng Hoàng Ân")).toBe("ÂƯ");
  });

  it("survives a one-word name and stray whitespace", () => {
    expect(initialsOf("Minh")).toBe("MI");
    expect(initialsOf("  Nguyễn   Minh  ")).toBe("MN");
    expect(initialsOf("   ")).toBe("");
  });

  it("is hidden from assistive tech, since the name is already beside it", () => {
    const { container } = render(<Avatar name="Nguyễn Đức Minh" />);
    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
  });

  it("marks the signed-in user in the brand tone and offers the deck's square", () => {
    const { container } = render(
      <Avatar name="Hoàng Thương" size="56" shape="square" tone="self" />,
    );
    expect(container.firstElementChild).toHaveClass(
      "size-14",
      "rounded-2xl",
      "bg-brand-soft",
    );
  });
});
