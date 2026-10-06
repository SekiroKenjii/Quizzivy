import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { createMemoryRouter, RouterProvider } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import AssignmentDetailPage from "@/features/assignments/pages/teacher/AssignmentDetailPage";

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

describe("actual roster intervention origin focus", () => {
  it.each(["extend", "reset", "void"])(
    "returns %s Cancel and Escape to the actual keyboard menu origin",
    async (kind) => {
      const { user } = mount();
      const row = await screen.findByRole("row", { name: /Phạm Gia Hân/ });
      const trigger = within(row).getByRole("button", { name: "Thao tác" });
      const label = {
        extend: "Gia hạn thời gian",
        reset: "Cho làm lại",
        void: "Huỷ lượt làm này",
      }[kind]!;
      for (const close of ["cancel", "escape"]) {
        trigger.focus();
        await user.keyboard("{Enter}");
        await user.click(await screen.findByRole("menuitem", { name: label }));
        const dialog = await screen.findByRole("dialog");
        if (close === "cancel")
          await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
        else await user.keyboard("{Escape}");
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
        await waitFor(() => expect(trigger).toHaveFocus());
      }
    },
  );
  it("captures main from the real origin if a refetch removes its row before close", async () => {
    const { user, client } = mount();
    const trigger = within(
      await screen.findByRole("row", { name: /Phạm Gia Hân/ }),
    ).getByRole("button", { name: "Thao tác" });
    await user.click(trigger);
    await user.click(
      await screen.findByRole("menuitem", { name: "Gia hạn thời gian" }),
    );
    const dialog = await screen.findByRole("dialog");
    server.use(
      http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/attempts`, () =>
        contractJson(
          "/teacher/assignments/{id}/attempts",
          "get",
          200,
          monitor(monitor().rows.slice(1)),
        ),
      ),
    );
    await act(() =>
      client.invalidateQueries({ queryKey: ["admin-monitor", ASSIGNMENT_ID] }),
    );
    await waitFor(() => expect(trigger.isConnected).toBe(false));
    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(document.querySelector("main")).toHaveFocus());
  });
});

it.each(["extend", "reset", "void"])(
  "returns successful %s to its real origin and preserves the trimmed API payload",
  async (kind) => {
    const sent: unknown[] = [];
    const response = {
      id: ATTEMPT_ID,
      assignmentId: ASSIGNMENT_ID,
      studentId: monitor().rows[0]!.studentId,
      testVersionId: assignment().testVersionId,
      attemptNo: 1,
      status: kind === "extend" ? "in_progress" : "voided",
      startedAt: "2026-09-04T02:10:00Z",
      deadlineAt: "2026-09-04T02:55:00Z",
    };
    server.use(
      http.post(
        `${BASE}/teacher/attempts/${ATTEMPT_ID}/${kind}`,
        async ({ request }) => {
          sent.push(await request.json());
          if (kind === "extend")
            return contractJson("/teacher/attempts/{id}/extend", "post", 200, response);
          if (kind === "reset")
            return contractJson("/teacher/attempts/{id}/reset", "post", 200, response);
          return contractJson("/teacher/attempts/{id}/void", "post", 200, response);
        },
      ),
    );
    const { user } = mount();
    const origin = within(
      await screen.findByRole("row", { name: /Phạm Gia Hân/ }),
    ).getByRole("button", { name: "Thao tác" });
    await user.click(origin);
    const label = {
      extend: "Gia hạn thời gian",
      reset: "Cho làm lại",
      void: "Huỷ lượt làm này",
    }[kind]!;
    await user.click(await screen.findByRole("menuitem", { name: label }));
    const dialog = await screen.findByRole("dialog");
    const confirm = { extend: "Gia hạn", reset: "Cho làm lại", void: "Huỷ lượt làm" }[
      kind
    ]!;
    expect(within(dialog).getByRole("button", { name: confirm })).toBeDisabled();
    await user.type(within(dialog).getByLabelText("Lý do"), "  lý do thật  ");
    await user.click(within(dialog).getByRole("button", { name: confirm }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(origin).toHaveFocus());
    expect(sent).toEqual([
      kind === "extend"
        ? { minutes: 10, reason: "lý do thật" }
        : { reason: "lý do thật" },
    ]);
  },
);
it("nested minutes Escape closes only the Select before the Dialog returns focus, then another row replaces the origin", async () => {
  const { user } = mount();
  const a = within(await screen.findByRole("row", { name: /Phạm Gia Hân/ })).getByRole(
    "button",
    { name: "Thao tác" },
  );
  await user.click(a);
  await user.click(await screen.findByRole("menuitem", { name: "Gia hạn thời gian" }));
  const dialog = await screen.findByRole("dialog");
  const minutes = within(dialog).getByRole("combobox", { name: "Cộng thêm" });
  await user.click(minutes);
  await screen.findByRole("listbox");
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(dialog).toBeInTheDocument();
  await user.keyboard("{Escape}");
  await waitFor(() => expect(a).toHaveFocus());
  const b = within(screen.getByRole("row", { name: /Nguyễn Đức Minh/ })).getByRole(
    "button",
    { name: "Thao tác" },
  );
  await user.click(b);
  await user.click(await screen.findByRole("menuitem", { name: "Cho làm lại" }));
  await user.keyboard("{Escape}");
  await waitFor(() => expect(b).toHaveFocus());
  expect(a).not.toHaveFocus();
});
