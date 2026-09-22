import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, Link, RouterProvider } from "react-router";
import { expect, it } from "vitest";
import { useListFilters } from "@/hooks/useListFilters";

function List() {
  const { params, setFilter } = useListFilters();
  return <>
    <input aria-label="Search" value={params.get("q") ?? ""} onChange={(event) => setFilter("q", event.target.value)} />
    <button onClick={() => setFilter("status", "archived")}>Archived</button>
    <button onClick={() => setFilter("q", null)}>Clear search</button>
    <output>{params.get("status")}</output>
    <Link to="/list/child">Open</Link>
  </>;
}

function mount(initial = "/list") {
  const client = new QueryClient();
  const router = createMemoryRouter([
    { path: "/list", element: <List /> },
    { path: "/list/child", element: <Link to="/list">Back</Link> },
  ], { initialEntries: [initial] });
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);
  return { user: userEvent.setup(), router };
}

it("keeps URL filters after returning through a plain list link", async () => {
  const { user, router } = mount();
  await user.type(screen.getByLabelText("Search"), "listening");
  await user.click(screen.getByText("Archived"));
  await user.click(screen.getByText("Open"));
  await user.click(await screen.findByText("Back"));
  expect(await screen.findByLabelText("Search")).toHaveValue("listening");
  await waitFor(() => expect(new URLSearchParams(router.state.location.search).get("status")).toBe("archived"));
});

it("respects an explicit URL and does not restore a deliberately cleared filter", async () => {
  const { user, router } = mount("/list?q=old&status=archived&page=4");
  await user.click(screen.getByText("Clear search"));
  expect(router.state.location.search).not.toContain("q=");
  expect(router.state.location.search).not.toContain("page=");
  await user.click(screen.getByText("Open"));
  await user.click(await screen.findByText("Back"));
  expect(await screen.findByLabelText("Search")).toHaveValue("");
  await act(async () => { await router.navigate("/list?q=shared"); });
  expect(screen.getByLabelText("Search")).toHaveValue("shared");
  expect(screen.getByRole("status")).toHaveTextContent("");
});
