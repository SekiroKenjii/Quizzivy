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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentDetailPage from "@/features/assignments/pages/teacher/AssignmentDetailPage";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  ASSIGNMENT_ID,
  ATTEMPT_ID,
  BASE,
  assignment,
  monitor,
  review,
} from "../../units/attempts/fixtures";
import "@/lib/i18n";

const B = "018f0000-0000-7000-8000-0000000000a8";
const path = `/teacher/assignments/${ASSIGNMENT_ID}`;
const reviews: string[] = [];
const writes: unknown[] = [];
const eventReads: string[] = [];
let style: HTMLStyleElement;

beforeEach(() => {
  reviews.length = 0;
  writes.length = 0;
  eventReads.length = 0;
  useAuthStore.getState().setSession("token", teacherUser);
  style = document.createElement("style");
  style.textContent =
    '[data-slot="sheet"],[data-slot="sheet-overlay"] { animation-duration:200ms; } [data-state="open"][data-slot="sheet"],[data-state="open"][data-slot="sheet-overlay"] { animation-name:test-enter; } [data-state="closed"][data-slot="sheet"],[data-state="closed"][data-slot="sheet-overlay"] { animation-name:test-exit; }';
  document.head.append(style);
  const readStyle = globalThis.getComputedStyle;
  vi.spyOn(globalThis, "getComputedStyle").mockImplementation((node, pseudo) => {
    const original = readStyle(node, pseudo);
    if (!node.matches('[data-slot="sheet"],[data-slot="sheet-overlay"]'))
      return original;
    return new Proxy(original, {
      get(_target, key) {
        const current = readStyle(node, pseudo);
        const value: unknown = Reflect.get(current, key);
        return typeof value === "function" ? value.bind(current) : value;
      },
    });
  });
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}`, () =>
      contractJson("/teacher/assignments/{id}", "get", 200, assignment()),
    ),
    http.get(`${BASE}/teacher/tests/:id/versions`, () =>
      contractJson("/teacher/tests/{id}/versions", "get", 200, { items: [] }),
    ),
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/attempts`, () =>
      contractJson("/teacher/assignments/{id}/attempts", "get", 200, monitor()),
    ),
    http.get(`${BASE}/teacher/attempts/:id`, ({ params }) => {
      const id = String(params.id);
      reviews.push(id);
      const data = review();
      return contractJson("/teacher/attempts/{id}", "get", 200, {
        ...data,
        attempt: { ...data.attempt, id },
        student: { ...data.student, fullName: id === B ? "Paper B" : "Paper A" },
        teacherNote: id === B ? "B saved" : "A saved",
      });
    }),
    http.get(`${BASE}/teacher/attempts/:id/events`, ({ params }) => {
      eventReads.push(String(params.id));
      return contractJson("/teacher/attempts/{id}/events", "get", 200, {
        startedAt: review().attempt.startedAt,
        summary: review().integrity,
        events: [],
      });
    }),
    http.patch(`${BASE}/teacher/attempts/:id/note`, async ({ request }) => {
      const body: unknown = await request.json();
      writes.push(body);
      return contractJson("/teacher/attempts/{id}/note", "patch", 200, { note: null });
    }),
  );
});

afterEach(() => {
  cleanup();
  style.remove();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

function board(attempt: string | null = ATTEMPT_ID) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  });
  const router = createMemoryRouter(
    [
      {
        path: "/teacher/assignments/:id",
        element: (
          <main tabIndex={-1} data-scale="deck">
            <AssignmentDetailPage />
          </main>
        ),
      },
    ],
    { initialEntries: [path + (attempt ? `?attempt=${attempt}` : "")] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { client, router };
}

function end(node: Element) {
  const event = new Event("animationend", { bubbles: true });
  Object.defineProperty(event, "animationName", { value: "test-exit" });
  fireEvent(node, event);
}

describe("the actual route's controlled outgoing sheet", () => {
  it("retains the same connected identity as closed until panel and overlay Presence finish", async () => {
    const { router, client } = board();
    const sheet = await screen.findByRole("dialog", { name: "Paper A" });
    const overlay = document.querySelector('[data-slot="sheet-overlay"]')!;
    const note = screen.getByLabelText("Ghi chú riêng");
    fireEvent.click(screen.getByRole("button", { name: "Đóng" }));
    await waitFor(() => expect(router.state.location.search).toBe(""));
    expect(sheet.isConnected).toBe(true);
    expect(sheet).toHaveAttribute("data-state", "closed");
    expect(overlay.isConnected).toBe(true);
    expect(overlay).toHaveAttribute("data-state", "closed");
    expect(note).toHaveValue("A saved");
    expect(note).toBeDisabled();
    fireEvent.change(note, { target: { value: "must not become a closing draft" } });
    fireEvent.blur(note);
    expect(writes).toEqual([]);
    expect(note).toHaveValue("A saved");
    const link = sheet.querySelector<HTMLAnchorElement>(
      `a[href="/teacher/attempts/${ATTEMPT_ID}"]`,
    )!;
    fireEvent.click(link);
    expect(router.state.location.pathname).toBe(path);
    await act(() =>
      client.invalidateQueries({ queryKey: ["admin-attempt", ATTEMPT_ID] }),
    );
    expect(reviews).toEqual([ATTEMPT_ID]);
    end(sheet);
    end(overlay);
    await waitFor(() => expect(sheet.isConnected).toBe(false));
  });
  it("uses current B immediately, retains B on close and follows reopen during exit", async () => {
    const { router } = board();
    await screen.findByRole("dialog", { name: "Paper A" });
    await act(() => router.navigate(`${path}?attempt=${B}`));
    const sheet = await screen.findByRole("dialog", { name: "Paper B" });
    expect(screen.getByLabelText("Ghi chú riêng")).toHaveValue("B saved");
    await act(() => router.navigate(path));
    expect(sheet).toHaveAttribute("data-state", "closed");
    expect(sheet).toHaveAccessibleName("Paper B");
    await act(() => router.navigate(`${path}?attempt=${ATTEMPT_ID}`));
    expect(await screen.findByRole("dialog", { name: "Paper A" })).toBe(sheet);
    expect(sheet).toHaveAttribute("data-state", "open");
    expect(screen.getByLabelText("Ghi chú riêng")).toHaveValue("A saved");
    expect(reviews).not.toContain("");
    expect(writes).toEqual([]);
  });
  it("freezes the last painted live countdown through the controlled exit", async () => {
    vi.useFakeTimers({ toFake: ["Date", "setInterval", "clearInterval"] });
    server.use(
      http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () => {
        const data = review();
        return contractJson("/teacher/attempts/{id}", "get", 200, {
          ...data,
          student: { ...data.student, fullName: "Live paper" },
          attempt: {
            ...data.attempt,
            status: "in_progress",
            submittedAt: null,
            score: null,
          },
        });
      }),
    );
    const { router } = board();
    const sheet = await screen.findByRole("dialog", { name: "Live paper" });
    await act(() => vi.advanceTimersByTime(1000));
    const clock = within(sheet).getByText(/^Còn lại:/);
    const painted = clock.textContent;
    await act(() => router.navigate(path));
    expect(sheet).toHaveAttribute("data-state", "closed");
    expect(clock).toHaveTextContent(painted!);
    await act(() => vi.advanceTimersByTime(4000));
    expect(clock).toHaveTextContent(painted!);
  });
  it("does not reveal or mount late review data after an accepted close", async () => {
    let release = () => {};
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, async () => {
        await pending;
        const data = review();
        return contractJson("/teacher/attempts/{id}", "get", 200, {
          ...data,
          student: { ...data.student, fullName: "Late private name" },
          teacherNote: "Late private note",
        });
      }),
    );
    const { router, client } = board();
    const sheet = await screen.findByRole("dialog");
    await act(() => router.navigate(path));
    expect(sheet).toHaveAttribute("data-state", "closed");
    release();
    await waitFor(() =>
      expect(client.getQueryData(["admin-attempt", ATTEMPT_ID])).toBeDefined(),
    );
    expect(sheet).not.toHaveAccessibleName("Late private name");
    expect(screen.queryByLabelText("Ghi chú riêng")).toBeNull();
    expect(screen.queryByText("Late private note")).toBeNull();
    expect(sheet.querySelector("a")).toBeNull();
    expect(eventReads).toEqual([]);
  });
  it("does not resurrect a closed historical host through cached assignment navigation", async () => {
    const other = "018f0000-0000-7000-8000-0000000000d9";
    server.use(
      http.get(`${BASE}/teacher/assignments/${other}`, () =>
        contractJson(
          "/teacher/assignments/{id}",
          "get",
          200,
          assignment({ id: other }),
        ),
      ),
      http.get(`${BASE}/teacher/assignments/${other}/attempts`, () =>
        contractJson("/teacher/assignments/{id}/attempts", "get", 200, monitor()),
      ),
    );
    const { router, client } = board();
    const sheet = await screen.findByRole("dialog", { name: "Paper A" });
    await act(() => router.navigate(path));
    end(sheet);
    end(document.querySelector('[data-slot="sheet-overlay"]')!);
    await waitFor(() => expect(sheet.isConnected).toBe(false));
    client.setQueryData(["admin-assignment", other], assignment({ id: other }));
    await act(() => router.navigate(`/teacher/assignments/${other}`));
    await screen.findByRole("heading", { level: 1 });
    await act(() => router.navigate(path));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(reviews).toEqual([ATTEMPT_ID]);
  });
  it("creates no host or review without an initial URL selection", async () => {
    board(null);
    await screen.findByRole("heading", { level: 1 });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(reviews).toEqual([]);
  });
  it("sanitizes outgoing private data immediately when permission is lost", async () => {
    const { router } = board();
    await screen.findByRole("dialog", { name: "Paper A" });
    await act(() => router.navigate(path));
    await act(() =>
      useAuthStore.getState().setSession("token", { ...teacherUser, permissions: [] }),
    );
    expect(screen.queryByText("A saved")).toBeNull();
    expect(screen.queryByLabelText("Ghi chú riêng")).toBeNull();
    const outgoing = document.querySelector('[data-slot="sheet"]');
    if (outgoing) expect(outgoing).not.toHaveAccessibleName("Paper A");
  });
});
