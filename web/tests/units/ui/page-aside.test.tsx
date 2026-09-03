import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { PageAside } from "@/components/shared/PageAside";
import { PageAsideSlot } from "@/layouts/slots";

/** A shell the way AdminLayout builds one: a slot beside the scrolling main. */
function Shell({ children }: { children: React.ReactNode }) {
  const [slot, setSlot] = useState<HTMLDivElement | null>(null);
  return (
    <div>
      <main data-testid="main">
        <PageAsideSlot.Provider value={slot}>{children}</PageAsideSlot.Provider>
      </main>
      <div data-testid="slot" ref={setSlot} />
    </div>
  );
}

describe("the side panel", () => {
  // The deck sets it beside the content, not in it. Rendering into the shell's
  // slot is what keeps it still while main scrolls: it is a sibling of the
  // scroll container, so there is nothing to scroll it with.
  it("lands in the shell's slot, beside main, not inside the scrolling content", () => {
    render(
      <Shell>
        <p>content</p>
        <PageAside label="Tóm tắt">
          <p>19 học viên</p>
        </PageAside>
      </Shell>,
    );
    const panel = screen.getByRole("complementary", { name: "Tóm tắt" });
    expect(within(screen.getByTestId("slot")).getByText("19 học viên")).toBe(
      within(panel).getByText("19 học viên"),
    );
    expect(within(screen.getByTestId("main")).queryByRole("complementary")).toBeNull();
  });

  it("renders in place when there is no shell", () => {
    render(
      <main data-testid="main">
        <PageAside label="Tóm tắt">
          <p>19 học viên</p>
        </PageAside>
      </main>,
    );
    expect(
      within(screen.getByTestId("main")).getByRole("complementary", {
        name: "Tóm tắt",
      }),
    ).toBeInTheDocument();
  });

  // A-04: the builder lays out its own row under its own bar, and its panel
  // belongs in that row -- the nearest slot wins over the shell's.
  it("prefers the nearest slot", () => {
    function Builder({ children }: { children: React.ReactNode }) {
      const [slot, setSlot] = useState<HTMLDivElement | null>(null);
      return (
        <div>
          <PageAsideSlot.Provider value={slot}>{children}</PageAsideSlot.Provider>
          <div data-testid="builder-slot" ref={setSlot} />
        </div>
      );
    }
    render(
      <Shell>
        <Builder>
          <PageAside label="Cài đặt câu hỏi">
            <p>Điểm</p>
          </PageAside>
        </Builder>
      </Shell>,
    );
    expect(
      within(screen.getByTestId("builder-slot")).getByText("Điểm"),
    ).toBeInTheDocument();
    expect(within(screen.getByTestId("slot")).queryByText("Điểm")).toBeNull();
  });

  // S-08: under 1024px the navigator is a sheet, so the rail is not drawn.
  // jsdom has no viewport, so this checks the contract rather than the layout.
  it("can be hidden below the large breakpoint", () => {
    render(
      <PageAside label="Danh sách câu" hideBelow="lg">
        <p>dots</p>
      </PageAside>,
    );
    expect(screen.getByRole("complementary", { name: "Danh sách câu" })).toHaveClass(
      "hidden",
      "lg:block",
    );
  });
});
