import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { useLazyList, type PagedResult } from "@/hooks/useLazyList";
import {
  installIntersectionObserver,
  observedCount,
  scrollAllIntoView,
} from "@tests/support/intersection";
import "@/lib/i18n";

/** Forty rows, twenty a page. */
function fetchPage(page: number): Promise<PagedResult<{ id: string }>> {
  const items = Array.from({ length: 20 }, (_, i) => ({
    id: `r${(page - 1) * 20 + i + 1}`,
  }));
  return Promise.resolve({ items, page, pageSize: 20, total: 40 });
}

function List({ fetch = fetchPage }: Readonly<{ fetch?: typeof fetchPage }>) {
  const list = useLazyList({ queryKey: ["lazy"], fetchPage: (page) => fetch(page) });
  return (
    <ul>
      {list.items.map((r) => (
        <li key={r.id}>{r.id}</li>
      ))}
      <LoadMoreSentinel
        as="li"
        active={list.hasMore}
        loading={list.loadingMore}
        onVisible={list.loadMore}
      />
    </ul>
  );
}

function renderList(fetch?: typeof fetchPage) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <List {...(fetch ? { fetch } : {})} />
    </QueryClientProvider>,
  );
}

beforeEach(() => installIntersectionObserver());
afterEach(() => vi.restoreAllMocks());

describe("a lazy list", () => {
  it("shows the first page and asks for the next when its end comes into view", async () => {
    const calls: number[] = [];
    renderList((page) => {
      calls.push(page);
      return fetchPage(page);
    });
    expect(await screen.findByText("r20")).toBeInTheDocument();
    expect(screen.queryByText("r21")).toBeNull();
    expect(observedCount()).toBe(1);

    act(() => scrollAllIntoView());
    expect(await screen.findByText("r40")).toBeInTheDocument();
    expect(calls).toEqual([1, 2]);
  });

  // `total` says when the list is complete; a page coming back short does not.
  it("stops watching once total is reached", async () => {
    renderList();
    await screen.findByText("r20");
    act(() => scrollAllIntoView());
    await screen.findByText("r40");
    await waitFor(() => expect(observedCount()).toBe(0));
    expect(screen.queryByText("Đang tải thêm…")).toBeNull();
  });

  it("does not ask twice while a page is still on its way", async () => {
    const calls: number[] = [];
    let release: (() => void) | undefined;
    renderList((page) => {
      calls.push(page);
      if (page === 1) return fetchPage(1);
      return new Promise((resolve) => {
        release = () => resolve(fetchPage(page));
      });
    });
    await screen.findByText("r20");
    act(() => scrollAllIntoView());
    act(() => scrollAllIntoView());
    expect(await screen.findByText("Đang tải thêm…")).toBeInTheDocument();
    expect(calls).toEqual([1, 2]);
    act(() => release?.());
    await screen.findByText("r40");
  });
});
