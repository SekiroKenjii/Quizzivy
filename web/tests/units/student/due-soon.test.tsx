import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { focusManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "@/app/queryClient";
import { http, HttpResponse } from "msw";
import type { ReactNode } from "react";
import { dueSoonCount, useDueSoonCount } from "@/features/assignments/dueSoon";
import type { StudentAssignmentCard } from "@/features/assignments/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { BASE, card } from "./support";

const NOW = Date.parse("2026-08-29T08:00:00Z");
const HOUR = 60 * 60 * 1000;
const at = (offset: number) => new Date(NOW + offset).toISOString();
const paper = (over: Record<string, unknown>) => card(over) as StudentAssignmentCard;
const count = (
  dueNow: StudentAssignmentCard[],
  upcoming: StudentAssignmentCard[] = [],
) => dueSoonCount({ dueNow, upcoming }, NOW);

describe("the Home badge's count", () => {
  it("counts a paper to do that closes within seven days", () => {
    expect(count([paper({ closesAt: at(2 * 24 * HOUR) })])).toBe(1);
    expect(count([paper({ closesAt: at(7 * 24 * HOUR) })])).toBe(1);
  });

  it("leaves out a paper that closes later than that, or has closed", () => {
    expect(count([paper({ closesAt: at(7 * 24 * HOUR + 1) })])).toBe(0);
    expect(count([paper({ closesAt: at(-1) })])).toBe(0);
    expect(count([paper({ closesAt: at(0) })])).toBe(0);
  });

  it("counts a paper that has not opened yet when it closes within seven days", () => {
    const scheduled = paper({
      status: "scheduled",
      opensAt: at(24 * HOUR),
      closesAt: at(3 * 24 * HOUR),
    });
    expect(count([], [scheduled])).toBe(1);
  });

  it("counts a paper taken once that still has an attempt left", () => {
    expect(
      count([paper({ attemptsUsed: 1, maxAttempts: 2, closesAt: at(24 * HOUR) })]),
    ).toBe(1);
  });

  it("measures a paper being taken by its attempt's deadline, not its window", () => {
    const closedEarly = paper({
      status: "closed",
      closesAt: at(-HOUR),
      hasLiveAttempt: true,
      liveDeadlineAt: at(HOUR / 2),
    });
    expect(count([closedEarly])).toBe(1);

    const spent = paper({
      closesAt: at(24 * HOUR),
      hasLiveAttempt: true,
      liveDeadlineAt: at(-1),
    });
    expect(count([spent])).toBe(0);
  });

  it("adds both lists and nothing else", () => {
    const soon = paper({ closesAt: at(24 * HOUR) });
    const later = paper({ closesAt: at(30 * 24 * HOUR) });
    expect(count([soon, later], [soon, soon])).toBe(3);
  });
});

function wrapper({ children }: Readonly<{ children: ReactNode }>) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe("the shell's reading of that count", () => {
  it("is 0 until the lists arrive, then counts from when they were fetched", async () => {
    const soon = card({ closesAt: new Date(Date.now() + 24 * HOUR).toISOString() });
    server.use(
      http.get(`${BASE}/app/assignments`, () =>
        contractJson("/app/assignments", "get", 200, {
          dueNow: [soon],
          upcoming: [],
          completed: [soon],
        }),
      ),
    );
    const { result } = renderHook(() => useDueSoonCount(), { wrapper });
    expect(result.current).toBe(0);
    await waitFor(() => expect(result.current).toBe(1));
  });

  it("stays 0 when the lists cannot be read", async () => {
    let asked = 0;
    server.use(
      http.get(`${BASE}/app/assignments`, () => {
        asked += 1;
        return HttpResponse.json({ error: { code: "NOT_FOUND" } }, { status: 404 });
      }),
    );
    const { result } = renderHook(() => useDueSoonCount(), { wrapper });
    await waitFor(() => expect(asked).toBe(1));
    expect(result.current).toBe(0);
  });
});

const EMPTY = { dueNow: [], upcoming: [], completed: [] };

describe("when the shell reads the lists again, under the app's defaults", () => {
  afterEach(() => {
    vi.useRealTimers();
    focusManager.setFocused();
  });

  it("asks on mount over fresh lists, never on a timer, and when the tab comes back stale", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let asked = 0;
    server.use(
      http.get(`${BASE}/app/assignments`, () => {
        asked += 1;
        return contractJson("/app/assignments", "get", 200, EMPTY);
      }),
    );
    const client = new QueryClient({
      defaultOptions: {
        queries: { ...queryClient.getDefaultOptions().queries, retry: false },
      },
    });
    client.setQueryData(["my-assignments"], EMPTY);
    const fresh = ({ children }: Readonly<{ children: ReactNode }>) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    renderHook(() => useDueSoonCount(), { wrapper: fresh });
    await waitFor(() => expect(asked).toBe(1));

    await act(() => vi.advanceTimersByTimeAsync(5 * 60 * 1000));
    expect(asked).toBe(1);

    act(() => focusManager.setFocused(false));
    act(() => focusManager.setFocused(true));
    await waitFor(() => expect(asked).toBe(2));
  });
});
