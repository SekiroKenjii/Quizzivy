import { useState } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { createMemoryRouter, RouterProvider } from "react-router";
import { Pager } from "@/components/shared/data/Pager";
import { usePage, usePageSize } from "@/hooks/usePage";
import "@/lib/i18n";

function Screen({ preserveHash }: Readonly<{ preserveHash?: boolean | undefined }>) {
  const [filter, setFilter] = useState("first");
  const [page, setPage] = usePage(filter, preserveHash);
  const [size, setSize] = usePageSize([10, 20], preserveHash);
  return (
    <>
      <button onClick={() => setPage(2)}>Set page</button>
      <button onClick={() => setSize(20)}>Set size</button>
      <button onClick={() => setFilter("second")}>Change filter</button>
      <Pager
        page={page}
        pageSize={size}
        total={45}
        sizes={[10, 20]}
        noun={() => "bài giao"}
        preserveHash={preserveHash}
      />
    </>
  );
}
function mount(preserveHash?: boolean) {
  const router = createMemoryRouter(
    [
      { path: "/list", element: <Screen preserveHash={preserveHash} /> },
      { path: "/before", element: <p>Previous entry</p> },
    ],
    {
      initialEntries: ["/before", "/list?page=3&other=x&other=y#anchor"],
      initialIndex: 1,
    },
  );
  render(<RouterProvider router={router} />);
  return { router, user: userEvent.setup() };
}

describe("pagination opt-in hash contract", () => {
  it.each([false, true])(
    "preserveHash=%s keeps duplicate unrelated values and existing arrow history",
    async (preserveHash) => {
      const { router, user } = mount(preserveHash);
      await user.click(screen.getByRole("link", { name: "Trang sau" }));
      expect(new URLSearchParams(router.state.location.search).get("page")).toBe("4");
      expect(new URLSearchParams(router.state.location.search).getAll("other")).toEqual(
        ["x", "y"],
      );
      expect(router.state.location.hash).toBe(preserveHash ? "#anchor" : "");
      expect(router.state.historyAction).toBe("PUSH");
      await act(() => router.navigate(-1));
      expect(router.state.location.hash).toBe("#anchor");
    },
  );
  it.each([false, true])(
    "preserveHash=%s page/size setters preserve default replace and size reset policy",
    async (preserveHash) => {
      const { router, user } = mount(preserveHash);
      await user.click(screen.getByRole("button", { name: "Set page" }));
      expect(new URLSearchParams(router.state.location.search).get("page")).toBe("2");
      expect(router.state.location.hash).toBe(preserveHash ? "#anchor" : "");
      await act(() =>
        router.navigate("/list?page=3&other=x&other=y#anchor", { replace: true }),
      );
      await user.click(screen.getByRole("button", { name: "Set size" }));
      const params = new URLSearchParams(router.state.location.search);
      expect(params.get("size")).toBe("20");
      expect(params.has("page")).toBe(false);
      expect(params.getAll("other")).toEqual(["x", "y"]);
      expect(router.state.location.hash).toBe(preserveHash ? "#anchor" : "");
      expect(router.state.historyAction).toBe("REPLACE");
      await act(() => router.navigate(-1));
      expect(screen.getByText("Previous entry")).toBeInTheDocument();
    },
  );
  it.each([false, true])(
    "preserveHash=%s implicit filter reset uses the same opt-in policy",
    async (preserveHash) => {
      const { router, user } = mount(preserveHash);
      await user.click(screen.getByRole("button", { name: "Change filter" }));
      await waitFor(() =>
        expect(new URLSearchParams(router.state.location.search).has("page")).toBe(
          false,
        ),
      );
      expect(new URLSearchParams(router.state.location.search).getAll("other")).toEqual(
        ["x", "y"],
      );
      expect(router.state.location.hash).toBe(preserveHash ? "#anchor" : "");
      expect(router.state.historyAction).toBe("REPLACE");
    },
  );
});

it("omitted flags retain the existing query-only navigation defaults", async () => {
  const { router, user } = mount();
  await user.click(screen.getByRole("link", { name: "Trang sau" }));
  expect(router.state.location.hash).toBe("");
  await act(() =>
    router.navigate("/list?page=3&other=x&other=y#anchor", { replace: true }),
  );
  await user.click(screen.getByRole("button", { name: "Set page" }));
  expect(router.state.location.hash).toBe("");
  await act(() =>
    router.navigate("/list?page=3&other=x&other=y#anchor", { replace: true }),
  );
  await user.click(screen.getByRole("button", { name: "Set size" }));
  expect(router.state.location.hash).toBe("");
  expect(new URLSearchParams(router.state.location.search).getAll("other")).toEqual([
    "x",
    "y",
  ]);
});
