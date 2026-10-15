import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RoleCarousel } from "@/features/roles/components/RoleCarousel";
import i18n from "@/lib/i18n";
import { contentWidth } from "@tests/support/contentWidth";
import { ROLES, TEACHER_ID } from "@tests/support/roles";

const CARD = 300;

const scrollTo = vi.fn();

function dots() {
  return screen.getAllByRole("button", { name: /^Đến vị trí \d+$/ });
}

function track() {
  const element = document.querySelector<HTMLElement>("section > div > div");
  if (element === null) throw new Error("no track");
  return element;
}

function restAt(left: number, width = 900, total = 5 * CARD) {
  const element = track();
  Object.defineProperty(element, "scrollLeft", { configurable: true, value: left });
  Object.defineProperty(element, "clientWidth", { configurable: true, value: width });
  Object.defineProperty(element, "scrollWidth", { configurable: true, value: total });
  act(() => {
    element.dispatchEvent(new Event("scroll"));
  });
}

function show(props: Partial<Parameters<typeof RoleCarousel>[0]> = {}) {
  return render(
    <RoleCarousel roles={ROLES} onEditPermissions={() => undefined} {...props} />,
  );
}

beforeEach(() => {
  scrollTo.mockClear();
  Element.prototype.scrollTo = scrollTo;
  Object.defineProperty(HTMLElement.prototype, "offsetLeft", {
    configurable: true,
    get() {
      const parent = (this as HTMLElement).parentElement;
      return parent === null ? 0 : Array.from(parent.children).indexOf(this) * CARD;
    },
  });
});

afterEach(async () => {
  Reflect.deleteProperty(HTMLElement.prototype, "offsetLeft");
  vi.unstubAllGlobals();
  await i18n.changeLanguage("vi");
});

describe("how many cards show", () => {
  it.each([
    [1100, 3, "basis-[calc((100%-1.5rem)/3)]"],
    [700, 2, "basis-[calc((100%-0.75rem)/2)]"],
    [400, 1, "basis-[86%]"],
  ])(
    "at %ipx of content: %i to a view, one dot for each place it can rest",
    (px, per, basis) => {
      contentWidth(px);
      show();
      expect(dots()).toHaveLength(ROLES.length - per + 1);
      expect(screen.getAllByRole("article")[0]).toHaveClass(basis);
    },
  );

  it("follows the content when the sidebar collapses or the window narrows", () => {
    const { resize } = contentWidth(1100);
    show();
    expect(dots()).toHaveLength(3);
    resize(700);
    expect(dots()).toHaveLength(4);
    resize(400);
    expect(dots()).toHaveLength(5);
  });

  it("draws no dots when every card fits", () => {
    contentWidth(1100);
    show({ roles: ROLES.slice(0, 3) });
    expect(screen.queryAllByRole("button", { name: /^Đến vị trí/ })).toHaveLength(0);
    expect(screen.getByRole("button", { name: "Các vai trò trước" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Các vai trò tiếp theo" }),
    ).toBeDisabled();
  });
});

describe("moving along", () => {
  it("starts at the first card with Previous out of reach", () => {
    contentWidth(1100);
    show();
    expect(screen.getByRole("button", { name: "Các vai trò trước" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Các vai trò tiếp theo" })).toBeEnabled();
    expect(dots()[0]).toHaveAttribute("aria-current", "true");
  });

  it("scrolls to the next card's left edge, smoothly", async () => {
    contentWidth(1100);
    show();
    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Các vai trò tiếp theo" }));
    expect(scrollTo).toHaveBeenCalledWith({ left: CARD, behavior: "smooth" });
  });

  it("scrolls without animation under reduced motion", async () => {
    contentWidth(1100);
    vi.stubGlobal(
      "matchMedia",
      (query: string) =>
        ({
          matches: query.includes("prefers-reduced-motion"),
          media: query,
          onchange: null,
          addEventListener: () => undefined,
          removeEventListener: () => undefined,
          addListener: () => undefined,
          removeListener: () => undefined,
          dispatchEvent: () => false,
        }) as unknown as MediaQueryList,
    );
    show();
    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Các vai trò tiếp theo" }));
    expect(scrollTo).toHaveBeenCalledWith({ left: CARD, behavior: "auto" });
  });

  it("jumps to the card a dot stands for", async () => {
    contentWidth(1100);
    show();
    await userEvent.setup().click(screen.getByRole("button", { name: "Đến vị trí 3" }));
    expect(scrollTo).toHaveBeenCalledWith({ left: 2 * CARD, behavior: "smooth" });
  });

  it("marks the dot the track rests on, and enables Previous", () => {
    contentWidth(1100);
    show();
    restAt(CARD);
    expect(dots()[1]).toHaveAttribute("aria-current", "true");
    expect(dots()[0]).not.toHaveAttribute("aria-current");
    expect(screen.getByRole("button", { name: "Các vai trò trước" })).toBeEnabled();
  });

  it("takes the last dot and disables Next when the track reaches its end", () => {
    contentWidth(1100);
    show();
    restAt(2 * CARD + 10, 900, 5 * CARD);
    expect(dots()[2]).toHaveAttribute("aria-current", "true");
    expect(
      screen.getByRole("button", { name: "Các vai trò tiếp theo" }),
    ).toBeDisabled();
  });

  it("hands focus to Previous when pressing Next brought the track to its end", async () => {
    contentWidth(1100);
    show();
    const user = userEvent.setup();
    restAt(CARD);
    const next = screen.getByRole("button", { name: "Các vai trò tiếp theo" });
    await user.click(next);
    expect(next).toHaveFocus();
    restAt(2 * CARD);
    expect(screen.getByRole("button", { name: "Các vai trò trước" })).toHaveFocus();
  });

  it("leaves focus where the user put it when an arrow runs out", async () => {
    contentWidth(1100);
    show();
    const user = userEvent.setup();
    restAt(2 * CARD);
    await user.click(screen.getByRole("button", { name: "Các vai trò trước" }));
    await user.click(screen.getByRole("button", { name: "Đến vị trí 2" }));
    restAt(0);
    expect(screen.getByRole("button", { name: "Đến vị trí 2" })).toHaveFocus();
  });
});

describe("revealing a card", () => {
  it("scrolls the track to a card that was added at the end", () => {
    contentWidth(1100);
    const { rerender } = show({ roles: ROLES.slice(0, 4) });
    scrollTo.mockClear();
    rerender(
      <RoleCarousel
        roles={ROLES}
        reveal={ROLES[4]!.id}
        onEditPermissions={() => undefined}
      />,
    );
    expect(scrollTo).toHaveBeenCalledWith({ left: 2 * CARD, behavior: "smooth" });
  });

  it("scrolls to the card itself when it is early in the list", () => {
    contentWidth(1100);
    show({ reveal: TEACHER_ID });
    expect(scrollTo).toHaveBeenCalledWith({ left: CARD, behavior: "smooth" });
  });

  it("does nothing for an id that is not among the roles", () => {
    contentWidth(1100);
    show({ reveal: "nobody" });
    expect(scrollTo).not.toHaveBeenCalled();
  });
});

describe("the cards", () => {
  it("draws the action each card is given, and tells which role Edit permissions is for", async () => {
    contentWidth(1100);
    const edited: string[] = [];
    show({
      action: (role) => <button type="button">{`Sửa ${role.id}`}</button>,
      onEditPermissions: (role) => edited.push(role.id),
    });
    expect(screen.getByRole("button", { name: `Sửa ${TEACHER_ID}` })).toBeVisible();
    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Sửa quyền của Giáo viên" }));
    expect(edited).toEqual([TEACHER_ID]);
  });
});
