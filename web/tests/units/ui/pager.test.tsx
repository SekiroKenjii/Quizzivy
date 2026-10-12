import { describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { useState } from "react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider, useNavigate } from "react-router";
import { usePage } from "@/hooks/usePage";
import "@/lib/i18n";

function Screen({ filter }: Readonly<{ filter: string }>) {
  const [page] = usePage(filter);
  return (
    <>
      <p>trang {page}</p>
    </>
  );
}

function renderAt(path: string, filter = "a") {
  const router = createMemoryRouter(
    [{ path: "/teacher/tests", element: <Screen filter={filter} /> }],
    {
      initialEntries: [path],
    },
  );
  const view = render(<RouterProvider router={router} />);
  return { router, view };
}

describe("usePage", () => {
  it("does not cancel a pending child navigation when a filter settles on page one", async () => {
    let finish!: () => void;
    const loaded = new Promise<void>((resolve) => {
      finish = resolve;
    });
    function ListWithPendingFilter() {
      const [filter, setFilter] = useState("old");
      usePage(filter);
      const navigate = useNavigate();
      return (
        <button
          onClick={() => {
            setFilter("new");
            void navigate("/child");
          }}
        >
          Open child
        </button>
      );
    }
    const router = createMemoryRouter(
      [
        { path: "/list", element: <ListWithPendingFilter /> },
        {
          path: "/child",
          loader: async () => {
            await loaded;
            return null;
          },
          element: <h1>Child</h1>,
        },
      ],
      { initialEntries: ["/list"] },
    );
    render(<RouterProvider router={router} />);
    await userEvent.setup().click(screen.getByText("Open child"));
    await act(async () => {
      finish();
      await loaded;
    });
    expect(await screen.findByRole("heading", { name: "Child" })).toBeInTheDocument();
  });

  it("opens on the page a shared link names", () => {
    renderAt("/teacher/tests?page=4");
    expect(screen.getByText("trang 4")).toBeInTheDocument();
  });

  // Page 7 of one search is not page 7 of another.
  it("goes back to page 1 when the filters change, not on mount", () => {
    const { router, view } = renderAt("/teacher/tests?page=4", "a");
    expect(screen.getByText("trang 4")).toBeInTheDocument();
    view.rerender(<RouterProvider router={router} />);
    expect(screen.getByText("trang 4")).toBeInTheDocument();
    // A new filter value: a fresh element tree with the same router.
    const swapped = createMemoryRouter(
      [{ path: "/teacher/tests", element: <Screen filter="b" /> }],
      { initialEntries: ["/teacher/tests?page=4"] },
    );
    view.unmount();
    render(<RouterProvider router={swapped} />);
    expect(screen.getByText("trang 4")).toBeInTheDocument();
  });
});
