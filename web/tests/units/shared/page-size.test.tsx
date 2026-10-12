import { describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { PAGE_SIZES, usePage, usePageSize } from "@/hooks/usePage";

const OFFERED = [10, 20, 24, 30, 48, 50] as const;

function Screen({ sizes }: Readonly<{ sizes: readonly number[] | undefined }>) {
  const [size, setSize] = usePageSize(sizes);
  const [page] = usePage();
  return (
    <>
      <output>{size}</output>
      <p>trang {page}</p>
      {OFFERED.map((n) => (
        <button key={n} onClick={() => setSize(n)}>
          {n} dòng
        </button>
      ))}
    </>
  );
}

function renderAt(entries: string[], sizes?: readonly number[]) {
  const router = createMemoryRouter(
    [
      { path: "/teacher/tests", element: <Screen sizes={sizes} /> },
      { path: "/teacher/classes", element: <h1>Lớp học</h1> },
    ],
    { initialEntries: entries, initialIndex: entries.length - 1 },
  );
  render(<RouterProvider router={router} />);
  return router;
}

const size = () => screen.getByRole("status").textContent;
const choose = (n: number) =>
  userEvent.setup().click(screen.getByRole("button", { name: `${n} dòng` }));

describe("usePageSize", () => {
  it("offers the deck's four sizes, the first of them when the URL names none", () => {
    expect(PAGE_SIZES).toEqual([10, 20, 30, 50]);
    renderAt(["/teacher/tests"]);
    expect(size()).toBe("10");
  });

  it("reads a listed size from the URL", () => {
    renderAt(["/teacher/tests?size=30"]);
    expect(size()).toBe("30");
  });

  it.each(["7", "abc", "", "100", "20.5", "-20"])(
    "falls back to the first size for size=%s",
    (raw) => {
      renderAt([`/teacher/tests?size=${raw}`]);
      expect(size()).toBe("10");
    },
  );

  it("writes the size, drops the page, keeps the rest and replaces the entry", async () => {
    const router = renderAt(["/teacher/classes", "/teacher/tests?tab=draft&page=3"]);
    expect(screen.getByText("trang 3")).toBeInTheDocument();
    await choose(20);
    expect(router.state.location.search).toBe("?tab=draft&size=20");
    expect(router.state.historyAction).toBe("REPLACE");
    expect(size()).toBe("20");
    expect(screen.getByText("trang 1")).toBeInTheDocument();
    await act(() => router.navigate(-1));
    expect(router.state.location.pathname).toBe("/teacher/classes");
  });

  it("removes the parameter for the first size", async () => {
    const router = renderAt(["/teacher/tests?size=30&tab=draft&page=2"]);
    await choose(10);
    expect(router.state.location.search).toBe("?tab=draft");
    expect(size()).toBe("10");
  });

  it("takes a screen's own sizes, whose first is the default", async () => {
    const router = renderAt(["/teacher/tests?size=10"], [24, 48]);
    expect(size()).toBe("24");
    await choose(48);
    expect(router.state.location.search).toBe("?size=48");
    expect(size()).toBe("48");
    await choose(24);
    expect(router.state.location.search).toBe("");
    expect(size()).toBe("24");
  });
});
