import { useEffect, useState } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { createMemoryRouter, RouterProvider } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import AssignmentDetailPage from "@/features/assignments/pages/teacher/AssignmentDetailPage";
import { AttemptSheet } from "@/features/attempts/components/AttemptSheet";
import { SheetNoteController } from "@/features/attempts/components/sheetNotes";
import type { AttemptReview } from "@/features/attempts/api";
import type { PermissionKey } from "@/features/auth/permissions";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
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
const PATH = `/teacher/assignments/${ASSIGNMENT_ID}`;
const persisted = new Map<string, string | null>();
const requests = {
  assignments: 0,
  monitor: 0,
  reviews: [] as string[],
  events: [] as string[],
  notes: [] as { id: string; note: string | null }[],
};

function grant(permissions: PermissionKey[], workspace = true) {
  useAuthStore.getState().setSession("token", {
    ...teacherUser,
    permissions,
    workspaces: workspace ? ["teacher"] : [],
  });
}

function paper(id = ATTEMPT_ID): AttemptReview {
  const data = review();
  const initialNote = id === B ? "B saved" : "A saved";
  return {
    ...data,
    student: {
      ...data.student,
      fullName: id === B ? "Nguyễn Đức Minh" : "Phạm Gia Hân",
    },
    attempt: { ...data.attempt, id },
    teacherNote: persisted.has(id) ? persisted.get(id)! : initialNote,
  };
}

function events(id: string) {
  requests.events.push(id);
  return contractJson("/teacher/attempts/{id}/events", "get", 200, {
    startedAt: "2026-09-04T02:10:00Z",
    summary: review().integrity,
    events: [],
  });
}

function noteResponse(id: string, note: string | null) {
  persisted.set(id, note);
  return contractJson("/teacher/attempts/{id}/note", "patch", 200, { note });
}

function serve() {
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}`, () => {
      requests.assignments++;
      return contractJson("/teacher/assignments/{id}", "get", 200, assignment());
    }),
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/attempts`, () => {
      requests.monitor++;
      return contractJson("/teacher/assignments/{id}/attempts", "get", 200, monitor());
    }),
    http.get(`${BASE}/teacher/tests/:id/versions`, () =>
      contractJson("/teacher/tests/{id}/versions", "get", 200, { items: [] }),
    ),
    http.get(`${BASE}/teacher/attempts/:id`, ({ params }) => {
      requests.reviews.push(String(params.id));
      return contractJson(
        "/teacher/attempts/{id}",
        "get",
        200,
        paper(String(params.id)),
      );
    }),
    http.get(`${BASE}/teacher/attempts/:id/events`, ({ params }) =>
      events(String(params.id)),
    ),
    http.patch(`${BASE}/teacher/attempts/:id/note`, async ({ request, params }) => {
      const body = (await request.json()) as { note: string | null };
      requests.notes.push({ id: String(params.id), ...body });
      return noteResponse(ATTEMPT_ID, body.note);
    }),
  );
}

function mount(at = PATH, prepare?: (client: QueryClient) => void, previous?: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  prepare?.(client);
  const router = createMemoryRouter(
    [
      {
        path: "/teacher/assignments/:id",
        element: (
          <main tabIndex={-1}>
            <AssignmentDetailPage />
          </main>
        ),
      },
      { path: "/teacher/attempts/:id", element: <p>toàn bộ bài làm</p> },
      { path: "/before", element: <p>trang trước</p> },
    ],
    {
      initialEntries: previous === undefined ? [at] : [previous, at],
      initialIndex: previous === undefined ? 0 : 1,
    },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, client, user: userEvent.setup() };
}

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

beforeEach(() => {
  persisted.clear();
  requests.assignments = 0;
  requests.monitor = 0;
  requests.reviews.length = 0;
  requests.events.length = 0;
  requests.notes.length = 0;
  grant([
    "teaching.grading",
    "teaching.attempts.intervene",
    "teaching.assignments.write",
  ]);
  serve();
});
afterEach(() => {
  useAuthStore.getState().clearSession();
});

function rejectNotes() {
  server.use(
    http.patch(`${BASE}/teacher/attempts/:id/note`, async ({ request, params }) => {
      const body = (await request.json()) as { note: string | null };
      requests.notes.push({ id: String(params.id), ...body });
      return HttpResponse.json(
        { error: { code: "FORBIDDEN", message: "Chưa lưu được", requestId: "note" } },
        { status: 403 },
      );
    }),
  );
}
const RECOVERY = "Chưa lưu được ghi chú. Hãy thử lại hoặc bỏ thay đổi để tiếp tục.";
describe("current note recovery focus and explicit retry policy", () => {
  it.each(["close", "escape", "back", "tab", "spa"])(
    "focuses the actual invalid note after blocked %s without a PATCH",
    async (departure) => {
      const { user, router } = mount(
        `${PATH}?attempt=${ATTEMPT_ID}&other=x&other=y#anchor`,
        undefined,
        "/before",
      );
      const note = await screen.findByLabelText("Ghi chú riêng");
      fireEvent.change(note, { target: { value: "n".repeat(2001) } });
      const close = within(screen.getByRole("dialog")).getByRole("button", {
        name: "Đóng",
      });
      close.focus();
      if (departure === "close") await user.click(close);
      if (departure === "escape") await user.keyboard("{Escape}");
      if (departure === "back") await act(() => router.navigate(-1));
      if (departure === "tab")
        await act(() => router.navigate(`${PATH}?attempt=${ATTEMPT_ID}&tab=settings`));
      if (departure === "spa") await act(() => router.navigate("/before"));
      await screen.findByText(RECOVERY);
      await waitFor(() => expect(note).toHaveFocus());
      expect(note).toHaveValue("n".repeat(2001));
      expect(router.state.location.search).toContain(ATTEMPT_ID);
      expect(router.state.location.hash).toBe("#anchor");
      expect(requests.notes).toEqual([]);
      await user.click(screen.getByRole("button", { name: "Ở lại" }));
      close.focus();
      fireEvent.change(note, { target: { value: "n".repeat(2000) } });
      expect(close).toHaveFocus();
      await user.click(screen.getByRole("button", { name: "Thử lại" }));
      await waitFor(() =>
        expect(requests.notes).toEqual([{ id: ATTEMPT_ID, note: "n".repeat(2000) }]),
      );
    },
  );
  it("does not retry a known failed later edit on ordinary blur but explicitly saves the latest value", async () => {
    rejectNotes();
    const { user } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    const note = await screen.findByLabelText("Ghi chú riêng");
    fireEvent.change(note, { target: { value: "failed" } });
    fireEvent.blur(note);
    await screen.findByText("Chưa lưu được");
    expect(requests.notes).toHaveLength(1);
    fireEvent.change(note, { target: { value: "latest edited" } });
    note.focus();
    await user.tab();
    expect(requests.notes).toHaveLength(1);
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(requests.notes).toHaveLength(2));
    expect(requests.notes[1]).toEqual({ id: ATTEMPT_ID, note: "latest edited" });
  });
  it("actual Escape then focused Discard has blur before click without an implicit second write", async () => {
    rejectNotes();
    const { user, router } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    const note = await screen.findByLabelText("Ghi chú riêng");
    fireEvent.change(note, { target: { value: "failed draft" } });
    note.focus();
    await user.keyboard("{Escape}");
    await screen.findByText(RECOVERY);
    const before = requests.notes.length;
    const order: string[] = [];
    note.addEventListener("blur", () => order.push("blur"));
    const discard = screen.getByRole("button", { name: "Bỏ thay đổi và tiếp tục" });
    discard.addEventListener("click", () => order.push("click"));
    await user.click(discard);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(order).toEqual(["blur", "click"]);
    expect(requests.notes).toHaveLength(before);
    await act(() => router.navigate(`${PATH}?attempt=${ATTEMPT_ID}`));
    expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue("A saved");
  });
  it("reads a failure synchronously when blur precedes the next committed draft render", async () => {
    const writes: string[] = [];
    const notes = new SheetNoteController(async (_id, value) => {
      writes.push(value!);
      throw new Error("failed");
    });
    notes.accept(ATTEMPT_ID, "A saved");
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<MemoryNote client={client} notes={notes} />);
    const note = await screen.findByLabelText("Ghi chú riêng");
    let notified = false;
    const unsubscribe = notes.subscribe(() => {
      if (notes.get(ATTEMPT_ID)?.error != null && !notified) {
        notified = true;
        fireEvent.blur(note);
      }
    });
    try {
      await act(async () => {
        notes.change(ATTEMPT_ID, "fresh failure");
        expect(await notes.flush(ATTEMPT_ID)).toBe(false);
      });
      expect(notified).toBe(true);
      expect(writes).toEqual(["fresh failure"]);
    } finally {
      unsubscribe();
    }
  });
});
function MemoryNote({
  client,
  notes,
}: Readonly<{ client: QueryClient; notes: SheetNoteController }>) {
  const router = createMemoryRouter([
    {
      path: "/",
      element: (
        <main tabIndex={-1}>
          <AttemptSheet
            assignment={assignment()}
            open
            attemptId={ATTEMPT_ID}
            row={monitor().rows[0]}
            questionCount={24}
            serverTime={monitor().serverTime}
            receivedAt={0}
            notes={notes}
            onClose={() => {}}
          />
        </main>
      ),
    },
  ]);
  return (
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}

function mountedSheet(notes: SheetNoteController, initialRequest = 7) {
  let update!: (value: { id: string; open: boolean; request: number }) => void;
  function Host() {
    const [state, setState] = useState({
      id: ATTEMPT_ID,
      open: true,
      request: initialRequest,
    });
    useEffect(() => {
      update = setState;
    }, []);
    return (
      <main tabIndex={-1}>
        <AttemptSheet
          assignment={assignment()}
          open={state.open}
          attemptId={state.id}
          row={monitor().rows.find((row) => row.attemptId === state.id)}
          questionCount={24}
          serverTime={monitor().serverTime}
          receivedAt={0}
          notes={notes}
          noteFocusRequest={state.request}
          onClose={() => {}}
        />
      </main>
    );
  }
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "/", element: <Host /> }]);
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return {
    update: (id = ATTEMPT_ID, open = true, request = initialRequest) =>
      act(() => update({ id, open, request })),
  };
}
it("consumes focus requests once, ignores initial/stale requests and never focuses a valid B draft", async () => {
  const notes = new SheetNoteController(async () => {
    throw new Error("not a focus save");
  });
  notes.accept(ATTEMPT_ID, "A saved");
  notes.accept(B, "B saved");
  notes.change(ATTEMPT_ID, "n".repeat(2001));
  expect(await notes.flush(ATTEMPT_ID)).toBe(false);
  const host = mountedSheet(notes);
  const note = await screen.findByLabelText("Ghi chú riêng");
  const close = within(screen.getByRole("dialog")).getByRole("button", {
    name: "Đóng",
  });
  close.focus();
  expect(note).not.toHaveFocus();
  await host.update(ATTEMPT_ID, true, 8);
  expect(note).toHaveFocus();
  close.focus();
  await act(() => notes.change(ATTEMPT_ID, "m".repeat(2001)));
  expect(close).toHaveFocus();
  await host.update(B, true, 9);
  expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue("B saved");
  const bClose = within(screen.getByRole("dialog")).getByRole("button", {
    name: "Đóng",
  });
  bClose.focus();
  await host.update(B, true, 10);
  expect(bClose).toHaveFocus();
  await host.update(ATTEMPT_ID, true, 10);
  expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue("m".repeat(2001));
  const aClose = within(screen.getByRole("dialog")).getByRole("button", {
    name: "Đóng",
  });
  aClose.focus();
  expect(screen.getByLabelText("Ghi chú riêng")).not.toHaveFocus();
  await act(() => notes.change(ATTEMPT_ID, "corrected"));
  await host.update(ATTEMPT_ID, true, 11);
  expect(aClose).toHaveFocus();
  await act(() => notes.change(ATTEMPT_ID, "n".repeat(2001)));
  await act(() => grant(["teaching.attempts.intervene"]));
  expect(screen.queryByLabelText("Ghi chú riêng")).toBeNull();
  await host.update(ATTEMPT_ID, true, 12);
  await act(() => grant(["teaching.grading", "teaching.attempts.intervene"]));
  expect(await screen.findByLabelText("Ghi chú riêng")).not.toHaveFocus();
  await host.update(ATTEMPT_ID, false, 13);
  await host.update(ATTEMPT_ID, true, 13);
  expect(await screen.findByLabelText("Ghi chú riêng")).not.toHaveFocus();
});
it("waits for the actual active write before Discard and restores that saved value on reopening", async () => {
  const pending = deferred();
  server.use(
    http.patch(`${BASE}/teacher/attempts/${ATTEMPT_ID}/note`, async ({ request }) => {
      const body = (await request.json()) as { note: string | null };
      requests.notes.push({ id: ATTEMPT_ID, ...body });
      await pending.promise;
      return noteResponse(ATTEMPT_ID, body.note);
    }),
  );
  const { router, user } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
  const note = await screen.findByLabelText("Ghi chú riêng");
  fireEvent.change(note, { target: { value: "actually saved" } });
  fireEvent.blur(note);
  await waitFor(() => expect(requests.notes).toHaveLength(1));
  fireEvent.change(note, { target: { value: "n".repeat(2001) } });
  await user.keyboard("{Escape}");
  pending.resolve();
  await screen.findByText(RECOVERY);
  await user.click(screen.getByRole("button", { name: "Bỏ thay đổi và tiếp tục" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(requests.notes).toEqual([{ id: ATTEMPT_ID, note: "actually saved" }]);
  await act(() => router.navigate(`${PATH}?attempt=${ATTEMPT_ID}`));
  expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue("actually saved");
});

it.each([true, false])(
  "Discard waits for an already active recovery Retry and reopens its actual saved outcome=%s",
  async (success) => {
    const pending = deferred();
    server.use(
      http.patch(`${BASE}/teacher/attempts/${ATTEMPT_ID}/note`, async ({ request }) => {
        const body = (await request.json()) as { note: string | null };
        requests.notes.push({ id: ATTEMPT_ID, ...body });
        const attempt = requests.notes.length;
        if (attempt === 3) await pending.promise;
        if (attempt === 3 && success) return noteResponse(ATTEMPT_ID, body.note);
        return HttpResponse.json(
          { error: { code: "FORBIDDEN", message: "Chưa lưu được", requestId: "note" } },
          { status: 403 },
        );
      }),
    );
    const { router, user } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    const note = await screen.findByLabelText("Ghi chú riêng");
    fireEvent.change(note, { target: { value: "held recovery save" } });
    fireEvent.blur(note);
    await screen.findByText("Chưa lưu được");
    note.focus();
    await user.keyboard("{Escape}");
    const panel = within(
      (await screen.findByText(RECOVERY)).closest('[role="alert"]')!,
    );
    expect(requests.notes).toHaveLength(2);
    await user.click(panel.getByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(requests.notes).toHaveLength(3));
    await user.click(panel.getByRole("button", { name: "Bỏ thay đổi và tiếp tục" }));
    expect(router.state.location.search).toContain(ATTEMPT_ID);
    expect(screen.getByLabelText("Ghi chú riêng")).toHaveValue("held recovery save");
    pending.resolve();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(requests.notes).toHaveLength(3);
    await act(() => router.navigate(`${PATH}?attempt=${ATTEMPT_ID}`));
    expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue(
      success ? "held recovery save" : "A saved",
    );
  },
);
