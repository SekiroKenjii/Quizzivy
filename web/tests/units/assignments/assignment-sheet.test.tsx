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
import { reviewKey } from "@/features/attempts/keys";
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
} from "../attempts/fixtures";
import "@/lib/i18n";
import { viewport } from "@tests/support/viewport";
import { contentWidth } from "@tests/support/contentWidth";

const B = "018f0000-0000-7000-8000-0000000000a8";
const OTHER = "018f0000-0000-7000-8000-0000000000d9";
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

describe("actual assignment detail permission and identity gates", () => {
  it.each([
    [[], false, false, false],
    [["teaching.grading"], true, true, false],
    [["teaching.attempts.intervene"], true, false, false],
    [["teaching.assignments.write"], false, false, true],
    [
      ["teaching.grading", "teaching.attempts.intervene", "teaching.assignments.write"],
      true,
      true,
      true,
    ],
  ] as const)(
    "honors workspace grants %j for read/edit/action combinations",
    async (permissions, read, edit, write) => {
      grant([...permissions]);
      mount(`${PATH}?attempt=${ATTEMPT_ID}`);
      await screen.findByRole("heading", {
        level: 1,
        name: assignment().testTitle,
        hidden: true,
      });
      const sheet = await screen.findByRole("dialog");
      await waitFor(() => expect(requests.reviews).toHaveLength(read ? 1 : 0));
      if (read) {
        expect(
          await within(sheet).findByText("Học viên không thấy ghi chú này"),
        ).toBeInTheDocument();
        expect(within(sheet).queryByRole("textbox")).toBe(
          edit ? within(sheet).getByRole("textbox") : null,
        );
        expect(
          within(sheet).queryByRole("link", { name: "Chấm câu trả lời" }) !== null,
        ).toBe(edit);
        await waitFor(() => expect(requests.events).toEqual([ATTEMPT_ID]));
      } else {
        expect(
          within(sheet).getByText("Không thể mở bài làm này."),
        ).toBeInTheDocument();
        expect(within(sheet).queryByRole("textbox")).toBeNull();
        expect(requests.events).toEqual([]);
      }
      const header = screen
        .getByRole("heading", { level: 1, hidden: true })
        .closest<HTMLElement>('[data-slot="page-head"]')!;
      expect(
        within(header).queryByRole("button", {
          name: "Thao tác khác",
          hidden: true,
        }) !== null,
      ).toBe(write || permissions.some((key) => key === "teaching.attempts.intervene"));
      expect(requests.notes).toEqual([]);
    },
  );

  it("sends no assignment, monitor, review or event query without Teacher workspace", async () => {
    grant(["teaching.grading", "teaching.attempts.intervene"], false);
    mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    expect(await screen.findByText("Không thể mở bài giao này.")).toBeInTheDocument();
    expect(requests).toMatchObject({
      assignments: 0,
      monitor: 0,
      reviews: [],
      events: [],
      notes: [],
    });
  });

  it("keeps workspace-only and not-started rows plain and unfocusable", async () => {
    grant([]);
    mount();
    const table = await screen.findByRole("table", { name: "Học viên được giao bài" });
    expect(within(table).queryByRole("button")).toBeNull();
    const row = within(table).getByRole("row", { name: /Hoàng Tiến Dũng/ });
    expect(row).not.toHaveAttribute("tabindex");
    expect(within(row).queryByRole("button")).toBeNull();
    fireEvent.click(row);
    expect(requests.reviews).toEqual([]);
  });

  it("validates the assignment response before any secondary query or private sheet", async () => {
    server.use(
      http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}`, () =>
        contractJson(
          "/teacher/assignments/{id}",
          "get",
          200,
          assignment({ id: OTHER }),
        ),
      ),
    );
    mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    expect(await screen.findByText("Không tìm thấy bài giao.")).toBeInTheDocument();
    expect(requests.monitor).toBe(0);
    expect(requests.reviews).toEqual([]);
    expect(requests.events).toEqual([]);
    expect(screen.queryByText(assignment().testTitle)).toBeNull();
  });

  it("refuses a wrong-assignment review without showing its identity or requesting events", async () => {
    server.use(
      http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () =>
        contractJson("/teacher/attempts/{id}", "get", 200, {
          ...paper(),
          student: { ...paper().student, fullName: "secret wrong student" },
          attempt: { ...paper().attempt, assignmentId: OTHER },
        }),
      ),
    );
    mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    const sheet = await screen.findByRole("dialog");
    expect(
      await within(sheet).findByText("Không thể mở bài làm này."),
    ).toBeInTheDocument();
    expect(within(sheet).queryByText("secret wrong student")).toBeNull();
    expect(within(sheet).queryByRole("textbox")).toBeNull();
    expect(requests.events).toEqual([]);
  });

  it("permits a valid historical deep link absent from the current monitor roster", async () => {
    mount(`${PATH}?attempt=${OTHER}`);
    expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue("A saved");
    await waitFor(() => expect(requests.events).toEqual([OTHER]));
    expect(screen.getByRole("link", { name: "Mở toàn bộ bài làm" })).toHaveAttribute(
      "href",
      `/teacher/attempts/${OTHER}`,
    );
  });

  it("hides cached A data under B's query key until B's validated response arrives", async () => {
    const pending = deferred();
    server.use(
      http.get(`${BASE}/teacher/attempts/${B}`, async () => {
        await pending.promise;
        return contractJson("/teacher/attempts/{id}", "get", 200, paper(B));
      }),
    );
    mount(`${PATH}?attempt=${B}`, (client) => {
      client.setQueryData(reviewKey(B), paper());
    });
    const sheet = await screen.findByRole("dialog");
    expect(within(sheet).queryByText("A saved")).toBeNull();
    expect(within(sheet).queryByRole("textbox")).toBeNull();
    expect(requests.events).toEqual([]);
    pending.resolve();
    expect(await within(sheet).findByLabelText("Ghi chú riêng")).toHaveValue("B saved");
    await waitFor(() => expect(requests.events).toEqual([B]));
  });

  it.each([400, 403, 404])(
    "keeps %i sheet failures local and permits a real retry",
    async (status) => {
      let failing = true;
      server.use(
        http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () =>
          failing
            ? HttpResponse.json(
                {
                  error: {
                    code: (
                      { 400: "VALIDATION", 403: "FORBIDDEN", 404: "NOT_FOUND" } as const
                    )[status],
                    message: "Không thể mở bài làm",
                    requestId: "id",
                  },
                },
                { status },
              )
            : contractJson("/teacher/attempts/{id}", "get", 200, paper()),
        ),
      );
      const { user } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
      const sheet = await screen.findByRole("dialog");
      expect(
        await within(sheet).findByText("Không thể mở bài làm này."),
      ).toBeInTheDocument();
      expect(requests.events).toEqual([]);
      failing = false;
      await user.click(within(sheet).getByRole("button", { name: "Thử lại" }));
      expect(await within(sheet).findByRole("textbox")).toHaveValue("A saved");
    },
  );

  it("uses full-monitor pending answers for the header and stable first eligible paper", async () => {
    const { router, user } = mount(
      `${PATH}?q=Ho%C3%A0ng&roster=notStarted&size=20&page=4&other=keep#anchor`,
    );
    await user.click(await screen.findByRole("button", { name: "Chấm 2 câu trả lời" }));
    await waitFor(() =>
      expect(new URLSearchParams(router.state.location.search).get("attempt")).toBe(B),
    );
    expect(new URLSearchParams(router.state.location.search).get("other")).toBe("keep");
    expect(router.state.location.hash).toBe("#anchor");
    expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue("B saved");
  });
});

describe("useful pinned assignment panels", () => {
  it.each([3, 4])(
    "requests the immutable version and never substitutes response version %i",
    async (version) => {
      let requestedVersion: string | null = null;
      const prompt = "Published version question";
      server.use(
        http.get(`${BASE}/teacher/tests/:id/preview`, ({ request }) => {
          requestedVersion = new URL(request.url).searchParams.get("version");
          return contractJson("/teacher/tests/{id}/preview", "get", 200, {
            version,
            questions: [
              {
                id: "018f0000-0000-7000-8000-00000000aa01",
                sectionId: "018f0000-0000-7000-8000-00000000aa04",
                type: "single_choice",
                prompt,
                media: null,
                audio: null,
                options: [
                  { id: "018f0000-0000-7000-8000-00000000bb01", text: "First" },
                  { id: "018f0000-0000-7000-8000-00000000bb02", text: "Second" },
                ],
                blanks: [],
                points: 1,
              },
            ],
          });
        }),
      );
      mount(`${PATH}?tab=questions`);
      await waitFor(() => expect(requestedVersion).toBe("3"));
      expect(screen.getByRole("link", { name: "Xem đề" })).toHaveAttribute(
        "href",
        `/teacher/tests/${assignment().testId}`,
      );
      if (version === 3) expect(await screen.findByText(prompt)).toBeInTheDocument();
      else {
        expect(
          await screen.findByText("Không thể tải nội dung phiên bản đề đã giao."),
        ).toBeInTheDocument();
        expect(screen.queryByText(prompt)).toBeNull();
      }
      expect(requests.reviews).toEqual([]);
      expect(requests.events).toEqual([]);
    },
  );

  it("keeps a separately denied test preview local and the roster reachable", async () => {
    server.use(
      http.get(`${BASE}/teacher/tests/:id/preview`, () =>
        HttpResponse.json(
          {
            error: {
              code: "FORBIDDEN",
              message: "Test unavailable",
              requestId: "preview",
            },
          },
          { status: 403 },
        ),
      ),
    );
    const { user } = mount(`${PATH}?tab=questions`);
    expect(
      await screen.findByText("Không thể tải nội dung phiên bản đề đã giao."),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Xem đề" })).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Học viên" }));
    expect(
      await screen.findByRole("table", { name: "Học viên được giao bài" }),
    ).toBeInTheDocument();
    expect(requests.assignments).toBe(1);
    expect(requests.monitor).toBe(1);
  });

  it("shows a real empty roster rather than fabricated students or stats", async () => {
    server.use(
      http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/attempts`, () =>
        contractJson("/teacher/assignments/{id}/attempts", "get", 200, monitor([])),
      ),
    );
    mount();
    expect(await screen.findByText("Chưa có học viên.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
    expect(screen.getByText("/ 0")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Chấm .* câu trả lời/ })).toBeNull();
  });
});

describe("sheet note flush and navigation recovery", () => {
  it("drains a latest edit before X closes and preserves unrelated query and hash", async () => {
    const first = deferred();
    const second = deferred();
    server.use(
      http.patch(`${BASE}/teacher/attempts/${ATTEMPT_ID}/note`, async ({ request }) => {
        const body = (await request.json()) as { note: string | null };
        requests.notes.push({ id: ATTEMPT_ID, ...body });
        await (requests.notes.length === 1 ? first.promise : second.promise);
        return noteResponse(ATTEMPT_ID, body.note);
      }),
    );
    const { user, router } = mount(
      `${PATH}?attempt=${ATTEMPT_ID}&tab=students&q=kept&other=x#anchor`,
    );
    const note = await screen.findByLabelText("Ghi chú riêng");
    fireEvent.change(note, { target: { value: "first" } });
    fireEvent.blur(note);
    await waitFor(() => expect(requests.notes).toHaveLength(1));
    fireEvent.change(note, { target: { value: "latest" } });
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Đóng" }),
    );
    expect(router.state.location.search).toContain(`attempt=${ATTEMPT_ID}`);
    first.resolve();
    await waitFor(() => expect(requests.notes).toHaveLength(2));
    expect(requests.notes[1]).toEqual({ id: ATTEMPT_ID, note: "latest" });
    expect(screen.getByLabelText("Ghi chú riêng")).toHaveValue("latest");
    second.resolve();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    const params = new URLSearchParams(router.state.location.search);
    expect(params.has("attempt")).toBe(false);
    expect(params.get("q")).toBe("kept");
    expect(params.get("other")).toBe("x");
    expect(router.state.location.hash).toBe("#anchor");
  });

  it("blocks an A-to-B sheet switch until A's immutable latest note saves", async () => {
    const pending = deferred();
    server.use(
      http.patch(`${BASE}/teacher/attempts/${ATTEMPT_ID}/note`, async ({ request }) => {
        const body = (await request.json()) as { note: string | null };
        requests.notes.push({ id: ATTEMPT_ID, ...body });
        await pending.promise;
        return noteResponse(ATTEMPT_ID, body.note);
      }),
    );
    const { router } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    fireEvent.change(await screen.findByLabelText("Ghi chú riêng"), {
      target: { value: "A latest" },
    });
    await act(() => router.navigate(`${PATH}?attempt=${B}`));
    await waitFor(() =>
      expect(requests.notes).toEqual([{ id: ATTEMPT_ID, note: "A latest" }]),
    );
    expect(router.state.location.search).toContain(ATTEMPT_ID);
    expect(requests.reviews).not.toContain(B);
    pending.resolve();
    expect(await screen.findByDisplayValue("B saved")).toBeInTheDocument();
    expect(requests.notes.every((item) => item.id === ATTEMPT_ID)).toBe(true);
  });

  it.each(["escape", "backdrop", "back"])(
    "flushes a focused draft before %s departure",
    async (kind) => {
      const pending = deferred();
      server.use(
        http.patch(
          `${BASE}/teacher/attempts/${ATTEMPT_ID}/note`,
          async ({ request }) => {
            const body = (await request.json()) as { note: string | null };
            requests.notes.push({ id: ATTEMPT_ID, ...body });
            await pending.promise;
            return noteResponse(ATTEMPT_ID, body.note);
          },
        ),
      );
      const { user, router } = mount(
        `${PATH}?attempt=${ATTEMPT_ID}`,
        undefined,
        "/before",
      );
      const note = await screen.findByLabelText("Ghi chú riêng");
      fireEvent.change(note, { target: { value: "latest before leaving" } });
      note.focus();
      if (kind === "escape") await user.keyboard("{Escape}");
      else if (kind === "backdrop")
        await user.click(
          document.querySelector<HTMLElement>('[data-slot="sheet-overlay"]')!,
        );
      else await act(() => router.navigate(-1));
      await waitFor(() => expect(requests.notes).toHaveLength(1));
      expect(router.state.location.search).toContain(ATTEMPT_ID);
      pending.resolve();
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      expect(requests.notes[0]).toEqual({
        id: ATTEMPT_ID,
        note: "latest before leaving",
      });
      if (kind === "back")
        expect(await screen.findByText("trang trước")).toBeInTheDocument();
    },
  );

  it("retains a failed draft on departure, supports Stay, retry and explicit discard", async () => {
    let failing = true;
    server.use(
      http.patch(`${BASE}/teacher/attempts/${ATTEMPT_ID}/note`, async ({ request }) => {
        const body = (await request.json()) as { note: string | null };
        requests.notes.push({ id: ATTEMPT_ID, ...body });
        return failing
          ? HttpResponse.json(
              {
                error: {
                  code: "FORBIDDEN",
                  message: "Chưa lưu được",
                  requestId: "note",
                },
              },
              { status: 403 },
            )
          : noteResponse(ATTEMPT_ID, body.note);
      }),
    );
    const { user, router } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    fireEvent.change(await screen.findByLabelText("Ghi chú riêng"), {
      target: { value: "keep this draft" },
    });
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Đóng" }),
    );
    const failure = await screen.findByText(
      "Chưa lưu được ghi chú. Hãy thử lại hoặc bỏ thay đổi để tiếp tục.",
    );
    expect(screen.getByLabelText("Ghi chú riêng")).toHaveValue("keep this draft");
    expect(router.state.location.search).toContain(ATTEMPT_ID);
    await user.click(
      within(failure.closest('[role="alert"]')!).getByRole("button", { name: "Ở lại" }),
    );
    expect(screen.getByLabelText("Ghi chú riêng")).toHaveValue("keep this draft");
    failing = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(screen.getByText("Đã lưu")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("Ghi chú riêng"), {
      target: { value: "discard this" },
    });
    failing = true;
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Đóng" }),
    );
    await user.click(
      await screen.findByRole("button", { name: "Bỏ thay đổi và tiếp tục" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await act(() => router.navigate(`${PATH}?attempt=${ATTEMPT_ID}`));
    expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue(
      "keep this draft",
    );
  });

  it("keeps a dirty note through review refetch and warns before unload without pretending to save asynchronously", async () => {
    const { client } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    const note = await screen.findByLabelText("Ghi chú riêng");
    fireEvent.change(note, { target: { value: "dirty draft" } });
    await act(() => client.invalidateQueries({ queryKey: reviewKey(ATTEMPT_ID) }));
    expect(screen.getByLabelText("Ghi chú riêng")).toHaveValue("dirty draft");
    const event = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
    expect(requests.notes).toEqual([]);
  });

  it("pushes sheet-open history and reconciles native Back/Forward without blind close history", async () => {
    const { user, router } = mount(`${PATH}?other=kept#anchor`);
    await user.click(await screen.findByRole("button", { name: "Phạm Gia Hân" }));
    await screen.findByRole("textbox");
    expect(router.state.historyAction).toBe("PUSH");
    await act(() => router.navigate(-1));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(new URLSearchParams(router.state.location.search).get("other")).toBe("kept");
    await act(() => router.navigate(1));
    expect(await screen.findByRole("textbox")).toHaveValue("A saved");
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Đóng" }),
    );
    await waitFor(() => expect(router.state.historyAction).toBe("REPLACE"));
    expect(router.state.location.pathname).toBe(PATH);
    expect(router.state.location.hash).toBe("#anchor");
  });
  it("retains a focused dirty note through resize and a tab change flushes before changing only tab", async () => {
    const size = viewport(1280);
    const pending = deferred();
    server.use(
      http.patch(`${BASE}/teacher/attempts/${ATTEMPT_ID}/note`, async ({ request }) => {
        const body = (await request.json()) as { note: string | null };
        requests.notes.push({ id: ATTEMPT_ID, ...body });
        await pending.promise;
        return noteResponse(ATTEMPT_ID, body.note);
      }),
    );
    const { router } = mount(`${PATH}?attempt=${ATTEMPT_ID}&other=keep#anchor`);
    const note = await screen.findByLabelText("Ghi chú riêng");
    note.focus();
    fireEvent.change(note, { target: { value: "retained during resize" } });
    await act(() => {
      size.resize(360);
      contentWidth(332);
    });
    expect(screen.getByLabelText("Ghi chú riêng")).toBe(note);
    expect(note).toHaveFocus();
    expect(note).toHaveValue("retained during resize");
    expect(requests.notes).toEqual([]);
    await act(() =>
      router.navigate(`${PATH}?attempt=${ATTEMPT_ID}&tab=settings&other=keep#anchor`, {
        replace: true,
      }),
    );
    await waitFor(() => expect(requests.notes).toHaveLength(1));
    expect(new URLSearchParams(router.state.location.search).get("tab")).toBeNull();
    pending.resolve();
    await waitFor(() =>
      expect(new URLSearchParams(router.state.location.search).get("tab")).toBe(
        "settings",
      ),
    );
    expect(screen.getByLabelText("Ghi chú riêng")).toBe(note);
    expect(note).toHaveValue("retained during resize");
    expect(router.state.location.hash).toBe("#anchor");
  });

  it("refuses a network-failed save and keeps the draft until an explicit retry succeeds", async () => {
    let failing = true;
    server.use(
      http.patch(`${BASE}/teacher/attempts/${ATTEMPT_ID}/note`, async ({ request }) => {
        const body = (await request.json()) as { note: string | null };
        requests.notes.push({ id: ATTEMPT_ID, ...body });
        return failing ? HttpResponse.error() : noteResponse(ATTEMPT_ID, body.note);
      }),
    );
    const { user } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    const note = await screen.findByLabelText("Ghi chú riêng");
    fireEvent.change(note, { target: { value: "network draft" } });
    fireEvent.blur(note);
    expect(
      await screen.findByText("Chưa lưu được ghi chú. Nội dung vẫn được giữ lại."),
    ).toBeInTheDocument();
    expect(note).toHaveValue("network draft");
    failing = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByText("Đã lưu")).toBeInTheDocument();
    expect(requests.notes).toEqual([
      { id: ATTEMPT_ID, note: "network draft" },
      { id: ATTEMPT_ID, note: "network draft" },
    ]);
  });
  it("reports the real 2000-character limit and refuses an oversized mounted draft before any request", async () => {
    const { user } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
    const note = await screen.findByLabelText("Ghi chú riêng");
    fireEvent.change(note, { target: { value: "n".repeat(2001) } });
    fireEvent.blur(note);
    expect(
      await screen.findByText("Ghi chú không được quá 2.000 ký tự."),
    ).toBeInTheDocument();
    expect(note).toHaveValue("n".repeat(2001));
    expect(requests.notes).toEqual([]);
    fireEvent.change(note, { target: { value: "n".repeat(2000) } });
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByText("Đã lưu")).toBeInTheDocument();
    expect(requests.notes).toEqual([{ id: ATTEMPT_ID, note: "n".repeat(2000) }]);
    fireEvent.change(note, { target: { value: " \n " } });
    fireEvent.blur(note);
    await waitFor(() =>
      expect(requests.notes.at(-1)).toEqual({ id: ATTEMPT_ID, note: null }),
    );
    expect(await screen.findByText("Đã lưu")).toBeInTheDocument();
  });
});

function serveManyStudents() {
  const source = monitor().rows[0]!;
  const rows = Array.from({ length: 35 }, (_, i) => ({
    ...source,
    studentId: `018f0000-0000-7000-8000-${String(i + 1000).padStart(12, "0")}`,
    attemptId: `018f0000-0000-7000-8000-${String(i + 2000).padStart(12, "0")}`,
    fullName: `Học viên ${String(i).padStart(2, "0")}`,
  }));
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/attempts`, () =>
      contractJson("/teacher/assignments/{id}/attempts", "get", 200, monitor(rows)),
    ),
  );
}

describe("assignment outer-state recovery and roster URL regressions", () => {
  beforeEach(() => viewport("desktop"));
  it.each(["error", "missing", "loading", "noWorkspace"])(
    "keeps generic recovery reachable after a dirty note enters %s without exposing private content",
    async (state) => {
      const { user, router, client } = mount(`${PATH}?attempt=${ATTEMPT_ID}`);
      fireEvent.change(await screen.findByLabelText("Ghi chú riêng"), {
        target: { value: "hidden dirty draft" },
      });
      const held = deferred();
      server.use(
        http.patch(
          `${BASE}/teacher/attempts/${ATTEMPT_ID}/note`,
          async ({ request }) => {
            requests.notes.push({
              id: ATTEMPT_ID,
              ...((await request.json()) as { note: string | null }),
            });
            return HttpResponse.json(
              {
                error: { code: "FORBIDDEN", message: "Unavailable", requestId: "note" },
              },
              { status: 403 },
            );
          },
        ),
        http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}`, async () => {
          if (state === "loading") await held.promise;
          if (state === "missing")
            return contractJson(
              "/teacher/assignments/{id}",
              "get",
              200,
              assignment({ id: OTHER }),
            );
          return HttpResponse.json(
            {
              error: {
                code: "NOT_FOUND",
                message: "Unavailable",
                requestId: "assignment",
              },
            },
            { status: 404 },
          );
        }),
      );
      if (state === "noWorkspace") await act(() => grant([], false));
      else if (state === "loading")
        await act(() => {
          void client.resetQueries({ queryKey: ["admin-assignment", ASSIGNMENT_ID] });
        });
      else
        await act(() =>
          client.invalidateQueries({ queryKey: ["admin-assignment", ASSIGNMENT_ID] }),
        );
      await waitFor(() => expect(screen.queryByLabelText("Ghi chú riêng")).toBeNull());
      expect(screen.queryByText("hidden dirty draft")).toBeNull();
      let departures = 0;
      const unsubscribe = router.subscribe(({ location }) => {
        if (location.pathname === "/before") departures++;
      });
      await act(() => router.navigate("/before"));
      const recovery = await screen.findByText(
        "Chưa lưu được ghi chú. Hãy thử lại hoặc bỏ thay đổi để tiếp tục.",
      );
      const panel = within(recovery.closest('[role="alert"]')!);
      expect(router.state.location.pathname).toBe(PATH);
      await user.click(panel.getByRole("button", { name: "Thử lại" }));
      await waitFor(() =>
        expect(panel.getByRole("button", { name: "Ở lại" })).toBeInTheDocument(),
      );
      await user.click(panel.getByRole("button", { name: "Ở lại" }));
      expect(
        screen.queryByText(
          "Chưa lưu được ghi chú. Hãy thử lại hoặc bỏ thay đổi để tiếp tục.",
        ),
      ).toBeNull();
      expect(router.state.location.pathname).toBe(PATH);
      if (state === "noWorkspace") {
        expect(requests.notes).toEqual([]);
        server.use(
          http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}`, () =>
            contractJson("/teacher/assignments/{id}", "get", 200, assignment()),
          ),
        );
        await act(() => grant(["teaching.grading"]));
        expect(await screen.findByLabelText("Ghi chú riêng")).toHaveValue(
          "hidden dirty draft",
        );
        await act(() => grant([], false));
        await waitFor(() =>
          expect(screen.queryByLabelText("Ghi chú riêng")).toBeNull(),
        );
      }
      await act(() => router.navigate("/before"));
      await user.click(
        await screen.findByRole("button", { name: "Bỏ thay đổi và tiếp tục" }),
      );
      expect(await screen.findByText("trang trước")).toBeInTheDocument();
      expect(departures).toBe(1);
      if (state === "noWorkspace") expect(requests.notes).toEqual([]);
      unsubscribe();
      held.resolve();
    },
  );

  it("keeps actual next/previous/last/first, size and implicit filter/clamp navigation hash and duplicate parameters", async () => {
    serveManyStudents();
    const { user, router } = mount(`${PATH}?other=x&other=y#anchor`);
    await screen.findByRole("table");
    const check = (page: string | null) => {
      const params = new URLSearchParams(router.state.location.search);
      expect(params.get("page")).toBe(page);
      expect(params.getAll("other")).toEqual(["x", "y"]);
      expect(router.state.location.hash).toBe("#anchor");
    };
    for (const [name, page] of [
      ["Trang sau", "2"],
      ["Trang trước", null],
      ["Trang cuối", "4"],
      ["Trang đầu", null],
    ] as const) {
      await user.click(screen.getByRole("link", { name }));
      check(page);
    }
    await user.click(screen.getByRole("combobox", { name: "Số dòng mỗi trang" }));
    await user.click(screen.getByRole("option", { name: "20" }));
    check(null);
    expect(new URLSearchParams(router.state.location.search).get("size")).toBe("20");
    await user.click(screen.getByRole("link", { name: "Trang cuối" }));
    check("2");
    await user.click(screen.getByRole("combobox", { name: "Số dòng mỗi trang" }));
    await user.click(screen.getByRole("option", { name: "10" }));
    check(null);
    expect(new URLSearchParams(router.state.location.search).has("size")).toBe(false);
    await act(() => router.navigate(`${PATH}?page=3&other=x&other=y&q=Học#anchor`));
    await waitFor(() => check(null));
    await act(() => router.navigate(`${PATH}?page=999&other=x&other=y&q=Học#anchor`));
    await user.click(screen.getByRole("link", { name: "Trang trước" }));
    check("3");
  });

  it("does not select null or omitted attempts and restores flagged tone after real selection closes", async () => {
    const source = monitor().rows;
    const empty = source[2]!;
    server.use(
      http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/attempts`, () =>
        contractJson(
          "/teacher/assignments/{id}/attempts",
          "get",
          200,
          monitor([
            source[0]!,
            {
              ...empty,
              attemptId: null,
              studentId: "018f0000-0000-7000-8000-000000000099",
              fullName: "Chưa bắt đầu null",
              flagged: true,
            },
            { ...empty, fullName: "Chưa bắt đầu omitted" },
          ]),
        ),
      ),
    );
    const { user } = mount();
    const table = await screen.findByRole("table");
    const nullRow = within(table).getByRole("row", { name: /Chưa bắt đầu null/ });
    const omittedRow = within(table).getByRole("row", { name: /Chưa bắt đầu omitted/ });
    expect(nullRow).not.toHaveClass("bg-muted");
    expect(nullRow).toHaveClass("bg-danger-soft");
    expect(omittedRow).not.toHaveClass("bg-muted");
    expect(within(nullRow).queryByRole("button")).toBeNull();
    const realRow = within(table).getByRole("row", { name: /Phạm Gia Hân/ });
    await user.click(within(realRow).getByRole("button", { name: "Phạm Gia Hân" }));
    await screen.findByLabelText("Ghi chú riêng");
    expect(realRow).toHaveClass("bg-muted");
    expect(nullRow).toHaveClass("bg-danger-soft");
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(realRow).not.toHaveClass("bg-muted");
    expect(realRow).toHaveClass("bg-danger-soft");
  });
});

describe("actual pager red navigation cases", () => {
  beforeEach(() => viewport("desktop"));
  it.each([
    ["Trang sau", "3"],
    ["Trang trước", null],
    ["Trang cuối", "4"],
    ["Trang đầu", null],
  ] as const)("%s preserves route hash and duplicates", async (name, page) => {
    serveManyStudents();
    const { user, router } = mount(`${PATH}?page=2&other=x&other=y#anchor`);
    await screen.findByRole("table");
    await user.click(screen.getByRole("link", { name }));
    expect(new URLSearchParams(router.state.location.search).get("page")).toBe(page);
    expect(new URLSearchParams(router.state.location.search).getAll("other")).toEqual([
      "x",
      "y",
    ]);
    expect(router.state.location.hash).toBe("#anchor");
  });
  it.each([
    [undefined, "20", "20"],
    ["20", "10", null],
  ] as const)(
    "actual page-size %s to %s preserves the hash while resetting page",
    async (initial, size, expected) => {
      serveManyStudents();
      const sizeParam = initial === undefined ? "" : `&size=${initial}`;
      const { user, router } = mount(
        `${PATH}?page=2&other=x&other=y${sizeParam}#anchor`,
      );
      await screen.findByRole("table");
      await user.click(screen.getByRole("combobox", { name: "Số dòng mỗi trang" }));
      await user.click(screen.getByRole("option", { name: size }));
      const params = new URLSearchParams(router.state.location.search);
      expect(params.get("size")).toBe(expected);
      expect(params.has("page")).toBe(false);
      expect(params.getAll("other")).toEqual(["x", "y"]);
      expect(router.state.location.hash).toBe("#anchor");
    },
  );
  it("implicit external filter reset preserves hash and duplicates", async () => {
    serveManyStudents();
    const { router } = mount(`${PATH}?page=3&other=x&other=y#anchor`);
    await screen.findByRole("table");
    await act(() => router.navigate(`${PATH}?page=3&other=x&other=y&q=Học#anchor`));
    await waitFor(() =>
      expect(new URLSearchParams(router.state.location.search).has("page")).toBe(false),
    );
    expect(new URLSearchParams(router.state.location.search).getAll("other")).toEqual([
      "x",
      "y",
    ]);
    expect(router.state.location.hash).toBe("#anchor");
  });
  it("arrows from a visibly clamped page preserve hash and duplicates", async () => {
    serveManyStudents();
    const { router, user } = mount(`${PATH}?page=999&other=x&other=y#anchor`);
    await screen.findByRole("table");
    await user.click(screen.getByRole("link", { name: "Trang trước" }));
    expect(new URLSearchParams(router.state.location.search).get("page")).toBe("3");
    expect(new URLSearchParams(router.state.location.search).getAll("other")).toEqual([
      "x",
      "y",
    ]);
    expect(router.state.location.hash).toBe("#anchor");
  });
});
