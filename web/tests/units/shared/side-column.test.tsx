import { beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SideColumn } from "@/components/shared/SideColumn";
import "@/lib/i18n";

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div data-columns style={{ display: "flex" }}>
      {children}
      <main data-resize-middle>middle</main>
    </div>
  );
}

beforeEach(() => {
  localStorage.clear();
});

describe("a side column (F-13)", () => {
  it("starts at the deck's width and remembers what the teacher drags it to", async () => {
    const user = userEvent.setup();
    render(
      <Layout>
        <SideColumn column="panel" side="right" aria-label="Tóm tắt">
          <p>content</p>
        </SideColumn>
      </Layout>,
    );
    const column = screen.getByRole("complementary", { name: "Tóm tắt" });
    expect(column).toHaveStyle({ width: "320px" });

    const handle = screen.getByRole("separator", { name: "Độ rộng bảng bên" });
    expect(handle).toHaveAttribute("aria-valuenow", "320");
    expect(handle).toHaveAttribute("aria-valuemin", "256");
    expect(handle).toHaveAttribute("aria-valuemax", "512");

    // A right column grows when dragged left.
    handle.setPointerCapture = () => {};
    handle.releasePointerCapture = () => {};
    fireEvent.pointerDown(handle, { button: 0, clientX: 800, pointerId: 1 });
    fireEvent.pointerMove(handle, { clientX: 740, pointerId: 1 });
    fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(column).toHaveStyle({ width: "380px" });
    expect(localStorage.getItem("quizzivy.column.panel")).toBe("380");

    // Double-click puts the deck's width back.
    await user.dblClick(handle);
    expect(column).toHaveStyle({ width: "320px" });
    expect(localStorage.getItem("quizzivy.column.panel")).toBeNull();
  });

  it("moves by keyboard and stays inside its limits", async () => {
    localStorage.setItem("quizzivy.column.rail", "200");
    const user = userEvent.setup();
    render(
      <Layout>
        <SideColumn column="rail" side="left" aria-label="Bộ lọc">
          <p>filters</p>
        </SideColumn>
      </Layout>,
    );
    const column = screen.getByRole("complementary", { name: "Bộ lọc" });
    expect(column).toHaveStyle({ width: "200px" });

    const handle = screen.getByRole("separator", { name: "Độ rộng cột lọc" });
    handle.focus();
    await user.keyboard("{ArrowRight}");
    expect(column).toHaveStyle({ width: "216px" });
    await user.keyboard("{ArrowLeft}{ArrowLeft}{ArrowLeft}");
    expect(column).toHaveStyle({ width: "192px" });
    await user.keyboard("{End}");
    expect(column).toHaveStyle({ width: "320px" });
    await user.keyboard("{Home}");
    expect(column).toHaveStyle({ width: "192px" });
  });

  it("ignores a remembered width outside the role's limits", () => {
    localStorage.setItem("quizzivy.column.outline", "9000");
    render(
      <Layout>
        <SideColumn as="div" column="outline" side="left" aria-label="Dàn ý">
          <p>outline</p>
        </SideColumn>
      </Layout>,
    );
    expect(screen.getByLabelText("Dàn ý")).toHaveStyle({ width: "384px" });
  });
});
