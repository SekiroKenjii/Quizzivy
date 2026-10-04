import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { http } from "msw";
import { Monitor } from "@/features/attempts/components/Monitor";
import { POLL_MS } from "@/features/attempts/keys";
import { IDLE_AFTER_MS } from "@/hooks/useIdlePolling";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { ASSIGNMENT_ID, ATTEMPT_ID, BASE, assignment, monitor } from "./fixtures";

let fetches = 0;
let voided: unknown = null;

function serve() {
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/attempts`, () => {
      fetches += 1;
      return contractJson("/teacher/assignments/{id}/attempts", "get", 200, monitor());
    }),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/void`, async ({ request }) => {
      voided = await request.json();
      return contractJson("/teacher/attempts/{id}/void", "post", 200, {
        id: ATTEMPT_ID,
        assignmentId: ASSIGNMENT_ID,
        studentId: monitor().rows[0]!.studentId,
        testVersionId: "018f0000-0000-7000-8000-0000000000f1",
        attemptNo: 1,
        status: "voided",
        startedAt: "2026-09-04T02:10:00Z",
        deadlineAt: "2026-09-04T02:55:00Z",
      });
    }),
  );
}

function renderMonitor(live: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <Monitor assignment={assignment()} live={live} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return client;
}

function setVisibility(state: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", {
    value: state,
    configurable: true,
  });
  act(() => {
    document.dispatchEvent(new Event("visibilitychange"));
  });
}

function advance(ms: number) {
  return act(() => vi.advanceTimersByTimeAsync(ms));
}

describe("the monitor", () => {
  beforeEach(() => {
    fetches = 0;
    voided = null;
    serve();
  });
  afterEach(() => {
    cleanup();
    setVisibility("visible");
    vi.useRealTimers();
  });

  it("polls every 15s while the assignment is open, and stops when the tab is hidden", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    renderMonitor(true);
    expect(await screen.findByText("Phạm Gia Hân")).toBeInTheDocument();
    expect(fetches).toBe(1);

    await vi.advanceTimersByTimeAsync(15_100);
    await waitFor(() => expect(fetches).toBe(2));

    setVisibility("hidden");
    await vi.advanceTimersByTimeAsync(31_000);
    expect(fetches).toBe(2);
  });

  it("does not poll once the assignment is closed", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    renderMonitor(false);
    expect(await screen.findByText("Phạm Gia Hân")).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(31_000);
    expect(fetches).toBe(1);
  });

  it("stops polling after ten idle minutes, refetches at once on a key press and polls from there", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const client = renderMonitor(true);
    expect(await screen.findByText("Phạm Gia Hân")).toBeInTheDocument();

    await advance(IDLE_AFTER_MS);
    await waitFor(() => expect(client.isFetching()).toBe(0));
    const whenIdle = fetches;
    expect(whenIdle).toBeGreaterThan(1);
    await advance(5 * 60_000);
    expect(client.isFetching()).toBe(0);
    expect(fetches).toBe(whenIdle);

    const pressedAt = Date.now();
    fireEvent.keyDown(window, { key: "Shift" });
    expect(client.isFetching()).toBe(1);
    await waitFor(() => expect(client.isFetching()).toBe(0));
    expect(fetches).toBe(whenIdle + 1);
    expect(Date.now() - pressedAt).toBeLessThan(POLL_MS);

    await advance(12_000);
    expect(fetches).toBe(whenIdle + 1);
    await advance(3_100);
    await waitFor(() => expect(fetches).toBe(whenIdle + 2));
  });

  it("refetches at once when the tab becomes visible again", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const client = renderMonitor(true);
    expect(await screen.findByText("Phạm Gia Hân")).toBeInTheDocument();
    expect(fetches).toBe(1);

    setVisibility("hidden");
    await advance(31_000);
    expect(fetches).toBe(1);

    setVisibility("visible");
    expect(client.isFetching()).toBe(1);
    await waitFor(() => expect(client.isFetching()).toBe(0));
    expect(fetches).toBe(2);
  });

  it("refetches once when the tab comes back after ten hidden minutes, and polls from there without an input", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const client = renderMonitor(true);
    expect(await screen.findByText("Phạm Gia Hân")).toBeInTheDocument();
    expect(fetches).toBe(1);

    setVisibility("hidden");
    await advance(IDLE_AFTER_MS + 60_000);
    expect(fetches).toBe(1);

    setVisibility("visible");
    expect(client.isFetching()).toBe(1);
    await waitFor(() => expect(client.isFetching()).toBe(0));
    expect(fetches).toBe(2);

    fireEvent.keyDown(window, { key: "Shift" });
    expect(client.isFetching()).toBe(0);

    await advance(12_000);
    expect(fetches).toBe(2);
    await advance(3_100);
    await waitFor(() => expect(fetches).toBe(3));
  });

  it("sends nothing on a key press or a return to the tab once the assignment is closed", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const client = renderMonitor(false);
    expect(await screen.findByText("Phạm Gia Hân")).toBeInTheDocument();

    await advance(IDLE_AFTER_MS);
    fireEvent.keyDown(window, { key: "Shift" });
    expect(client.isFetching()).toBe(0);
    setVisibility("hidden");
    setVisibility("visible");
    expect(client.isFetching()).toBe(0);
    await advance(31_000);
    expect(fetches).toBe(1);
  });

  it("draws every targeted student, including the one who has not started", async () => {
    renderMonitor(true);
    expect(await screen.findByText("Hoàng Tiến Dũng")).toBeInTheDocument();
    const row = screen.getByText("Hoàng Tiến Dũng").closest("tr")!;
    expect(within(row).getByText("Chưa bắt đầu")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "chờ chấm 2" })).toHaveAttribute(
      "href",
      "/teacher/attempts/018f0000-0000-7000-8000-0000000000a8",
    );
    expect(screen.getByLabelText("8 trên 24 câu đã trả lời")).toBeInTheDocument();
  });

  it("blocks an intervention until a reason is entered, then sends it trimmed", async () => {
    const user = userEvent.setup();
    renderMonitor(true);
    const row = (await screen.findByText("Phạm Gia Hân")).closest("tr")!;
    await user.click(within(row).getByRole("button", { name: "Thao tác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Huỷ lượt làm này" }));

    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", { name: "Huỷ lượt làm" });
    expect(confirm).toBeDisabled();

    await user.type(within(dialog).getByLabelText("Lý do"), "   ");
    expect(confirm).toBeDisabled();

    await user.type(within(dialog).getByLabelText("Lý do"), "Làm nhầm đề ");
    expect(confirm).toBeEnabled();
    await user.click(confirm);

    await waitFor(() => expect(voided).toEqual({ reason: "Làm nhầm đề" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });
});
