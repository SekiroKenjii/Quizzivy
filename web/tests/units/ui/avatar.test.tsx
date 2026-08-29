import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { Avatar } from "@/components/ui/avatar";

function initialsOf(name: string) {
  const { container } = render(<Avatar name={name} />);
  return container.textContent;
}

describe("the avatar's initials", () => {
  it("takes the family name and the given name, as the deck's examples do", () => {
    expect(initialsOf("Nguyễn Đức Minh")).toBe("NM");
    expect(initialsOf("Phạm Gia Hân")).toBe("PH");
    expect(initialsOf("Trần Bảo Linh")).toBe("TL");
    expect(initialsOf("Lê Khánh Vy")).toBe("LV");
  });

  it("uppercases Vietnamese letters without losing their diacritics", () => {
    expect(initialsOf("Đặng Thu Hà")).toBe("ĐH");
    expect(initialsOf("Ưng Hoàng Ân")).toBe("ƯÂ");
  });

  it("survives a one-word name and stray whitespace", () => {
    expect(initialsOf("Minh")).toBe("MI");
    expect(initialsOf("  Nguyễn   Minh  ")).toBe("NM");
    expect(initialsOf("   ")).toBe("");
  });

  it("is hidden from assistive tech, since the name is already beside it", () => {
    const { container } = render(<Avatar name="Nguyễn Đức Minh" />);
    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
  });
});
