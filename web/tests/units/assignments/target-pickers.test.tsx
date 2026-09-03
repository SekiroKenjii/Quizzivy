import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { ClassTargetPicker } from "@/features/assignments/components/TargetPickers";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  installIntersectionObserver,
  scrollAllIntoView,
} from "@tests/support/intersection";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

function klass(n: number) {
  return {
    id: `018f0000-0000-7000-8000-0000000000${String(n).padStart(2, "0")}`,
    name: `Lớp ${n}`,
    description: null,
    studentCount: n,
    selfJoinEnabled: false,
    createdAt: "2026-01-01T00:00:00Z",
  };
}

const requests: URL[] = [];

beforeEach(() => {
  installIntersectionObserver();
  requests.length = 0;
  // Forty-five classes, twenty a page, searchable by name.
  server.use(
    http.get(`${BASE}/admin/classes`, ({ request }) => {
      const url = new URL(request.url);
      requests.push(url);
      const page = Number(url.searchParams.get("page") ?? "1");
      const q = url.searchParams.get("q") ?? "";
      const all = Array.from({ length: 45 }, (_, i) => klass(i + 1)).filter((c) =>
        c.name.includes(q),
      );
      return contractJson("/admin/classes", "get", 200, {
        items: all.slice((page - 1) * 20, page * 20),
        page,
        pageSize: 20,
        total: all.length,
      });
    }),
  );
});
afterEach(() => server.resetHandlers());

function renderPicker() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <ClassTargetPicker selected={[]} onAdd={() => {}} onRemove={() => {}} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the class picker", () => {
  it("pages as the list is scrolled, and searches on the server", async () => {
    const user = renderPicker();
    await user.click(screen.getByRole("combobox"));
    const list = await screen.findByRole("list");
    expect(await within(list).findByText("Lớp 20")).toBeInTheDocument();
    expect(within(list).queryByText("Lớp 21")).toBeNull();

    act(() => scrollAllIntoView());
    expect(await within(list).findByText("Lớp 40")).toBeInTheDocument();
    expect(requests.map((u) => u.searchParams.get("page"))).toEqual([null, "2"]);

    await user.type(screen.getByRole("combobox"), "Lớp 4");
    await waitFor(() => expect(requests.at(-1)?.searchParams.get("q")).toBe("Lớp 4"));
    expect(await within(list).findByText("Lớp 45")).toBeInTheDocument();
    expect(within(list).queryByText("Lớp 20")).toBeNull();
  });
});
