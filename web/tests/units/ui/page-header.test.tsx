import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { PageHeader } from "@/components/shared/PageHeader";
import { PageBarSlot } from "@/layouts/pageBar";
import "@/lib/i18n";

/** A shell the way AdminLayout builds one: a slot above the scrolling main. */
function Shell({ children }: { children: React.ReactNode }) {
  const [slot, setSlot] = useState<HTMLDivElement | null>(null);
  return (
    <>
      <div data-testid="slot" ref={setSlot} />
      <main data-testid="main">
        <PageBarSlot.Provider value={slot}>{children}</PageBarSlot.Provider>
      </main>
    </>
  );
}

function renderAt(element: React.ReactNode, path = "/admin/tests/1") {
  const router = createMemoryRouter(
    [
      { path, element },
      { path: "/admin/tests", element: <p>list</p> },
    ],
    { initialEntries: [path] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

describe("the contextual bar", () => {
  // The deck keeps the topbar and sets this bar under it. Rendering it into
  // the shell's slot puts it outside the scroll container, where it stays put
  // the way the topbar does -- no sticky, no negative margins.
  it("lands in the shell's slot, above main, not inside the scrolling content", () => {
    renderAt(
      <Shell>
        <PageHeader title="Unit 5" backTo="/admin/tests" meta={<span>v3</span>} />
        <p>content</p>
      </Shell>,
    );
    const slot = screen.getByTestId("slot");
    expect(within(slot).getByRole("heading", { name: "Unit 5" })).toBeInTheDocument();
    expect(within(slot).getByText("v3")).toBeInTheDocument();
    expect(within(screen.getByTestId("main")).queryByRole("heading")).toBeNull();
  });

  it("renders in place when there is no shell", () => {
    renderAt(<PageHeader title="Unit 5" />);
    expect(screen.getByRole("heading", { name: "Unit 5" })).toBeInTheDocument();
  });

  it("goes back where it was told", async () => {
    const user = userEvent.setup();
    const router = renderAt(<PageHeader title="Unit 5" backTo="/admin/tests" />);
    await user.click(screen.getByRole("button", { name: "Quay lại" }));
    expect(router.state.location.pathname).toBe("/admin/tests");
  });

  it("has no back button unless there is somewhere to go", () => {
    renderAt(<PageHeader title="Unit 5" />);
    expect(screen.queryByRole("button", { name: "Quay lại" })).toBeNull();
  });
});

describe("the title block", () => {
  it("stays in the content, with the summary and the actions beside it", () => {
    renderAt(
      <Shell>
        <PageHeader
          variant="title"
          title="Đề thi"
          subtitle="12 đề · 3 nháp"
          actions={<button type="button">Đề mới</button>}
        />
      </Shell>,
    );
    const main = screen.getByTestId("main");
    expect(
      within(main).getByRole("heading", { level: 1, name: "Đề thi" }),
    ).toBeInTheDocument();
    expect(within(main).getByText("12 đề · 3 nháp")).toBeInTheDocument();
    expect(within(main).getByRole("button", { name: "Đề mới" })).toBeInTheDocument();
    expect(within(screen.getByTestId("slot")).queryByRole("heading")).toBeNull();
  });
});
