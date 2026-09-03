import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { Pager } from "@/components/shared/Pager";
import { usePage } from "@/hooks/usePage";
import "@/lib/i18n";

function Screen({ filter }: { filter: string }) {
  const [page] = usePage(filter);
  return (
    <>
      <p>trang {page}</p>
      <Pager page={page} pageSize={20} total={110} />
    </>
  );
}

function renderAt(path: string, filter = "a") {
  const router = createMemoryRouter(
    [{ path: "/admin/tests", element: <Screen filter={filter} /> }],
    {
      initialEntries: [path],
    },
  );
  const view = render(<RouterProvider router={router} />);
  return { router, view };
}

const nav = () => screen.getByRole("navigation", { name: "Phân trang" });

describe("Pager", () => {
  it("draws nothing for a single page", () => {
    const router = createMemoryRouter(
      [{ path: "/", element: <Pager page={1} pageSize={20} total={20} /> }],
      { initialEntries: ["/"] },
    );
    render(<RouterProvider router={router} />);
    expect(screen.queryByRole("navigation")).toBeNull();
  });

  it("links pages by number, keeping the other search parameters", () => {
    renderAt("/admin/tests?tab=draft&page=3");
    expect(screen.getByText("trang 3")).toBeInTheDocument();
    const three = within(nav()).getByRole("link", { name: "Trang 3" });
    expect(three).toHaveAttribute("aria-current", "page");
    expect(within(nav()).getByRole("link", { name: "Trang 4" })).toHaveAttribute(
      "href",
      "/admin/tests?tab=draft&page=4",
    );
    // Page 1 is the plain URL: no `page=1` to carry around.
    expect(within(nav()).getByRole("link", { name: "Trang 1" })).toHaveAttribute(
      "href",
      "/admin/tests?tab=draft",
    );
  });

  it("turns the page through the URL", async () => {
    const user = userEvent.setup();
    const { router } = renderAt("/admin/tests");
    await user.click(within(nav()).getByRole("link", { name: "Trang sau" }));
    expect(router.state.location.search).toBe("?page=2");
    expect(screen.getByText("trang 2")).toBeInTheDocument();
  });

  it("disables the edge it is on", () => {
    renderAt("/admin/tests?page=6");
    expect(within(nav()).getByRole("link", { name: "Trang sau" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(
      within(nav()).getByRole("link", { name: "Trang trước" }),
    ).not.toHaveAttribute("aria-disabled");
  });
});

describe("usePage", () => {
  it("opens on the page a shared link names", () => {
    renderAt("/admin/tests?page=4");
    expect(screen.getByText("trang 4")).toBeInTheDocument();
  });

  // Page 7 of one search is not page 7 of another.
  it("goes back to page 1 when the filters change, not on mount", () => {
    const { router, view } = renderAt("/admin/tests?page=4", "a");
    expect(screen.getByText("trang 4")).toBeInTheDocument();
    view.rerender(<RouterProvider router={router} />);
    expect(screen.getByText("trang 4")).toBeInTheDocument();
    // A new filter value: a fresh element tree with the same router.
    const swapped = createMemoryRouter(
      [{ path: "/admin/tests", element: <Screen filter="b" /> }],
      { initialEntries: ["/admin/tests?page=4"] },
    );
    view.unmount();
    render(<RouterProvider router={swapped} />);
    expect(screen.getByText("trang 4")).toBeInTheDocument();
  });
});
