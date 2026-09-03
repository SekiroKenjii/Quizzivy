import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { PageAside } from "@/components/shared/PageAside";
import { PageAsideSlot, PageRailSlot } from "@/layouts/slots";

/** A shell the way AdminLayout builds one: slots either side of the scroll. */
function Shell({ children }: { children: React.ReactNode }) {
  const [slot, setSlot] = useState<HTMLDivElement | null>(null);
  const [rail, setRail] = useState<HTMLDivElement | null>(null);
  return (
    <div>
      <div data-testid="rail-slot" ref={setRail} />
      <main data-testid="main">
        <PageAsideSlot.Provider value={slot}>
          <PageRailSlot.Provider value={rail}>{children}</PageRailSlot.Provider>
        </PageAsideSlot.Provider>
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

describe("the filter rail", () => {
  // A-06: the rail filters what main shows, so it takes the other edge -- and
  // its own slot, or it would land beside the detail panel on the right.
  it("lands in the rail slot, not the panel slot", () => {
    render(
      <Shell>
        <PageAside side="left" label="Bộ lọc">
          <p>Loại câu</p>
        </PageAside>
      </Shell>,
    );
    expect(
      within(screen.getByTestId("rail-slot")).getByText("Loại câu"),
    ).toBeInTheDocument();
    expect(within(screen.getByTestId("slot")).queryByText("Loại câu")).toBeNull();
    expect(within(screen.getByTestId("main")).queryByRole("complementary")).toBeNull();
  });

  // F-11: one width per role. A rail of checkboxes at panel width is mostly
  // empty, and a panel at rail width cannot hold G-07's three stat tiles.
  it("is narrower than the panel and borders the other side", () => {
    render(
      <>
        <PageAside side="left" label="Bộ lọc">
          <p>rail</p>
        </PageAside>
        <PageAside label="Tóm tắt">
          <p>panel</p>
        </PageAside>
      </>,
    );
    expect(screen.getByRole("complementary", { name: "Bộ lọc" })).toHaveClass(
      "w-56",
      "border-r",
    );
    expect(screen.getByRole("complementary", { name: "Tóm tắt" })).toHaveClass(
      "w-80",
      "border-l",
    );
  });

  it("both roles scroll themselves rather than with the content", () => {
    render(
      <>
        <PageAside side="left" label="Bộ lọc">
          <p>rail</p>
        </PageAside>
        <PageAside label="Tóm tắt">
          <p>panel</p>
        </PageAside>
      </>,
    );
    for (const name of ["Bộ lọc", "Tóm tắt"]) {
      expect(screen.getByRole("complementary", { name })).toHaveClass(
        "overflow-y-auto",
        "shrink-0",
      );
    }
  });
});
