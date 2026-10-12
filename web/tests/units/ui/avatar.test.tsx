import { describe, expect, it } from "vitest";
import { fireEvent, render } from "@testing-library/react";
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

describe("the avatar's photo", () => {
  const PHOTO = "https://media.example.com/avatars/hoang-thuong.png";

  it("shows the photo, filling the avatar, when one is given", () => {
    const { container } = render(<Avatar name="Hoàng Thương" src={PHOTO} />);
    const avatar = container.firstElementChild!;
    const photo = avatar.querySelector("img")!;
    expect(photo).toHaveAttribute("src", PHOTO);
    expect(photo).toHaveAttribute("alt", "");
    expect(photo).toHaveClass("size-full", "object-cover");
    expect(avatar).toHaveClass("overflow-hidden");
    expect(avatar).toHaveAttribute("aria-hidden", "true");
    expect(avatar).not.toHaveTextContent("TH");
  });

  it("falls back to the initials when the photo fails to load", () => {
    const { container } = render(<Avatar name="Hoàng Thương" src={PHOTO} />);
    fireEvent.error(container.querySelector("img")!);
    expect(container.querySelector("img")).toBeNull();
    expect(container.firstElementChild).toHaveTextContent("TH");
  });

  it("tries a new photo again after an earlier one failed", () => {
    const { container, rerender } = render(<Avatar name="Hoàng Thương" src={PHOTO} />);
    fireEvent.error(container.querySelector("img")!);
    rerender(<Avatar name="Hoàng Thương" src={`${PHOTO}?v=2`} />);
    expect(container.querySelector("img")).toHaveAttribute("src", `${PHOTO}?v=2`);
  });

  it("keeps the initials when there is no photo", () => {
    for (const src of [undefined, null, ""]) {
      const { container, unmount } = render(<Avatar name="Hoàng Thương" src={src} />);
      expect(container.querySelector("img")).toBeNull();
      expect(container.firstElementChild).toHaveTextContent("TH");
      unmount();
    }
  });
});
