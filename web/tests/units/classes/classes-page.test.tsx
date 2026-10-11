import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { delay, http, HttpResponse } from "msw";
import ClassesPage from "@/features/classes/pages/teacher/ClassesPage";
import type { ClassListItem } from "@/features/classes/api";
import { Toaster } from "@/components/ui/sonner";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { assistantUser, teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const ORIGIN = window.location.origin;

const EVENING: ClassListItem = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "IELTS 6.5 Evening",
  description: null,
  scheduleLabel: "Mon · Wed · Fri, 18:00",
  room: "Room 2",
  studentCount: 24,
  openAssignmentCount: 1,
  selfJoinEnabled: true,
  archivedAt: null,
  joinCode: {
    code: "K7QM-2PXA",
    hint: "2PXA",
    legacy: false,
    expiresAt: "2099-01-01T16:59:59Z",
    maxUses: 40,
    usesCount: 3,
  },
  createdAt: "2026-08-03T03:00:00Z",
  averageScore: { earned: 74, total: 100, pendingManual: 0 },
};
const LEGACY: ClassListItem = {
  ...EVENING,
  id: "018f0000-0000-7000-8000-0000000000c2",
  name: "Kids Starters B",
  scheduleLabel: null,
  room: null,
  studentCount: 12,
  openAssignmentCount: 0,
  joinCode: { ...EVENING.joinCode!, code: null, hint: "7HQM", legacy: true },
  averageScore: null,
};
const ARCHIVED: ClassListItem = {
  ...EVENING,
  id: "018f0000-0000-7000-8000-0000000000c3",
  name: "Summer 2025",
  archivedAt: "2026-09-01T00:00:00Z",
  joinCode: null,
};

let lists: URLSearchParams[] = [];
let posts: unknown[] = [];
let patches: { id: string; body: unknown }[] = [];
let rotations: { id: string; body: unknown }[] = [];

function serveList(
  active: ClassListItem[],
  archived: ClassListItem[] = [ARCHIVED],
  wait = 0,
) {
  server.use(
    http.get(`${BASE}/teacher/classes`, async ({ request }) => {
      if (wait > 0) await delay(wait);
      const query = new URL(request.url).searchParams;
      lists.push(query);
      const items = query.get("status") === "archived" ? archived : active;
      return contractJson("/teacher/classes", "get", 200, {
        items,
        page: 1,
        pageSize: 24,
        total: items.length,
        facets: {
          all: active.length + archived.length,
          joinable: active.length,
          archived: archived.length,
          students: 36,
        },
      });
    }),
  );
}

beforeEach(() => {
  lists = [];
  posts = [];
  patches = [];
  rotations = [];
  useAuthStore.setState({ user: teacherUser });
  serveList([EVENING, LEGACY]);
  server.use(
    http.post(`${BASE}/teacher/classes`, async ({ request }) => {
      posts.push(await request.json());
      return contractJson("/teacher/classes", "post", 201, {
        ...EVENING,
        id: "018f0000-0000-7000-8000-0000000000c9",
        name: "IELTS 7.0 Morning",
        joinCode: null,
        averageScore: undefined,
      });
    }),
    http.patch(`${BASE}/teacher/classes/:id`, async ({ request, params }) => {
      const body = (await request.json()) as Record<string, unknown>;
      patches.push({ id: String(params["id"]), body });
      return contractJson("/teacher/classes/{id}", "patch", 200, {
        ...EVENING,
        joinCode: null,
        averageScore: undefined,
      });
    }),
    http.post(`${BASE}/teacher/classes/:id/join-code`, async ({ request, params }) => {
      rotations.push({ id: String(params["id"]), body: await request.json() });
      return contractJson("/teacher/classes/{id}/join-code", "post", 201, {
        code: "B4XN-9TRE",
        expiresAt: "2099-01-01T16:59:59Z",
        maxUses: 40,
      });
    }),
  );
});

afterEach(() => {
  useAuthStore.setState({ user: null });
});

function renderPage(path = "/teacher/classes") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [
      { path: "/teacher/classes", element: <ClassesPage /> },
      { path: "/teacher/classes/:id", element: <p>class page</p> },
    ],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <main tabIndex={-1}>
        <RouterProvider router={router} />
      </main>
      <Toaster />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router, client };
}

async function card(name: string) {
  return (await screen.findByRole("link", { name })).closest("article")!;
}

describe("the classes list", () => {
  it("draws the deck's cards: schedule and room, the three figures and the code in full", async () => {
    renderPage();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Lớp học" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Học viên tham gia bằng mã hoặc đường liên kết."),
    ).toBeInTheDocument();
    const evening = await card("IELTS 6.5 Evening");
    expect(within(evening).getByText("Mon · Wed · Fri, 18:00 · Room 2")).toBeVisible();
    expect(within(evening).getByText("24")).toBeVisible();
    expect(within(evening).getByText("74%")).toBeVisible();
    expect(within(evening).getByText("K7QM-2PXA")).toBeVisible();

    const legacy = await card("Kids Starters B");
    expect(within(legacy).getByText("Chưa có lịch học")).toBeVisible();
    expect(within(legacy).getByText("—")).toBeVisible();
    expect(within(legacy).getByText("••••-7HQM")).toBeVisible();
    expect(
      within(legacy).getByRole("button", {
        name: "Tạo mã tham gia mới cho Kids Starters B",
      }),
    ).toBeVisible();

    expect(lists[0]?.get("withCodes")).toBe("true");
    expect(lists[0]?.get("status")).toBe("active");
    expect(lists[0]?.get("limit")).toBe("24");
  });

  it("keeps an expired code's footer on one row, its note truncating with a title (QA-41-4)", async () => {
    serveList([
      {
        ...EVENING,
        joinCode: { ...EVENING.joinCode!, expiresAt: "2020-01-01T00:00:00Z" },
      },
    ]);
    renderPage();
    const evening = await card("IELTS 6.5 Evening");
    const note = within(evening).getByText("Hết hạn");
    expect(note).toHaveAttribute("title", "Hết hạn");
    expect(note).toHaveClass("min-w-0", "truncate");
    expect(note.parentElement).not.toHaveClass("flex-wrap");
  });

  it("tells an expired code from a used-up one in words that fit a 360 card without hover (QA-505-1)", async () => {
    serveList([
      {
        ...EVENING,
        joinCode: { ...EVENING.joinCode!, expiresAt: "2020-01-01T00:00:00Z" },
      },
      {
        ...EVENING,
        id: "018f0000-0000-7000-8000-0000000000c4",
        name: "IELTS 7.0 Weekend",
        joinCode: { ...EVENING.joinCode!, usesCount: 40 },
      },
    ]);
    renderPage();
    const expired = within(await card("IELTS 6.5 Evening")).getByText("Hết hạn");
    const usedUp = within(await card("IELTS 7.0 Weekend")).getByText("Hết lượt");
    for (const note of [expired, usedUp]) {
      expect(note.textContent!.length).toBeLessThanOrEqual(8);
      expect(note).toHaveAttribute("title", note.textContent);
    }
  });

  it("asks for codes only for a reader who may change classes, and offers that reader nothing to change", async () => {
    useAuthStore.setState({ user: assistantUser });
    renderPage();
    const evening = await card("IELTS 6.5 Evening");
    expect(lists[0]?.has("withCodes")).toBe(false);
    expect(screen.queryByRole("button", { name: "Lớp mới" })).toBeNull();
    expect(
      within(evening).queryByRole("button", {
        name: "Thao tác với lớp IELTS 6.5 Evening",
      }),
    ).toBeNull();
  });

  it("copies the one link format, dash kept, and says which link it copied", async () => {
    const { user } = renderPage();
    const evening = await card("IELTS 6.5 Evening");
    await user.click(
      within(evening).getByRole("button", {
        name: "Sao chép liên kết tham gia của IELTS 6.5 Evening",
      }),
    );
    expect(await navigator.clipboard.readText()).toBe(`${ORIGIN}/join/K7QM-2PXA`);
    expect(
      await screen.findByText(
        `Đã sao chép liên kết tham gia: ${ORIGIN}/join/K7QM-2PXA`,
      ),
    ).toBeInTheDocument();
  });

  it("shows the QR with the code and the link, and gives focus back to the button that opened it", async () => {
    const { user } = renderPage();
    const evening = await card("IELTS 6.5 Evening");
    const opener = within(evening).getByRole("button", {
      name: "Hiện mã QR của IELTS 6.5 Evening",
    });
    await user.click(opener);
    const dialog = await screen.findByRole("dialog", { name: "IELTS 6.5 Evening" });
    expect(dialog.style.getPropertyValue("--dialog-width")).toBe("480px");
    expect(
      within(dialog).getByRole("img", {
        name: "Mã QR để tham gia lớp bằng mã K7QM-2PXA",
      }),
    ).toBeInTheDocument();
    expect(within(dialog).getByText(`${ORIGIN}/join/K7QM-2PXA`)).toBeVisible();
    expect(within(dialog).getByText("Ảnh QR cho slide hoặc Zalo")).toBeVisible();
    await user.click(within(dialog).getAllByRole("button", { name: "Đóng" }).at(-1)!);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(opener).toHaveFocus();
  });

  it("makes a new code only after a confirm, then gives focus back to the card's menu", async () => {
    const { user } = renderPage();
    const evening = await card("IELTS 6.5 Evening");
    const trigger = within(evening).getByRole("button", {
      name: "Thao tác với lớp IELTS 6.5 Evening",
    });
    await user.click(trigger);
    expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual([
      "Sửa lớp",
      "Hiện mã QR",
      "Sao chép liên kết tham gia",
      "Mã tham gia mới",
      "Lưu trữ lớp",
    ]);
    await user.click(screen.getByRole("menuitem", { name: "Mã tham gia mới" }));
    const dialog = await screen.findByRole("dialog", { name: "Tạo mã tham gia mới?" });
    expect(rotations).toEqual([]);
    await user.click(within(dialog).getByRole("button", { name: "Mã mới" }));
    await waitFor(() =>
      expect(rotations).toEqual([
        { id: EVENING.id, body: { expiresInDays: 30, maxUses: 40 } },
      ]),
    );
    expect(await screen.findByText("Mã tham gia mới: B4XN-9TRE")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(trigger).toHaveFocus();
  });

  it("gives focus to the card's menu once its footer New code is gone (QA-41-1)", async () => {
    const { user } = renderPage();
    const legacy = await card("Kids Starters B");
    await user.click(
      within(legacy).getByRole("button", {
        name: "Tạo mã tham gia mới cho Kids Starters B",
      }),
    );
    const dialog = await screen.findByRole("dialog", { name: "Tạo mã tham gia mới?" });
    serveList([
      EVENING,
      {
        ...LEGACY,
        joinCode: {
          ...LEGACY.joinCode!,
          code: "B4XN-9TRE",
          hint: "9TRE",
          legacy: false,
        },
      },
    ]);
    await user.click(within(dialog).getByRole("button", { name: "Mã mới" }));
    expect(
      await screen.findByRole("button", {
        name: "Sao chép liên kết tham gia của Kids Starters B",
      }),
    ).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(
      screen.queryByRole("button", { name: "Tạo mã tham gia mới cho Kids Starters B" }),
    ).toBeNull();
    expect(
      within(legacy).getByRole("button", { name: "Thao tác với lớp Kids Starters B" }),
    ).toHaveFocus();
  });

  it("gives focus to the page once a restored card leaves the Archived list (QA-41-1)", async () => {
    const { user } = renderPage("/teacher/classes?status=archived");
    const archived = await card("Summer 2025");
    serveList([EVENING, LEGACY], []);
    await user.click(
      within(archived).getByRole("button", { name: "Khôi phục Summer 2025" }),
    );
    await waitFor(() => expect(screen.queryByText("Summer 2025")).toBeNull());
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });

  it("gives focus to the page once a deleted card is gone (QA-41-1)", async () => {
    server.use(
      http.delete(
        `${BASE}/teacher/classes/:id`,
        () => new HttpResponse(null, { status: 204 }),
      ),
    );
    const { user } = renderPage("/teacher/classes?status=archived");
    const archived = await card("Summer 2025");
    await user.click(
      within(archived).getByRole("button", { name: "Thao tác với lớp Summer 2025" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Xoá lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Xoá vĩnh viễn" });
    serveList([EVENING, LEGACY], [], 200);
    await user.click(within(dialog).getByRole("button", { name: "Xoá vĩnh viễn" }));
    await waitFor(() => expect(screen.queryByText("Summer 2025")).toBeNull());
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });

  it("edits the name, schedule and room, a cleared field sent as none", async () => {
    const { user } = renderPage();
    const evening = await card("IELTS 6.5 Evening");
    await user.click(
      within(evening).getByRole("button", {
        name: "Thao tác với lớp IELTS 6.5 Evening",
      }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Sửa lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Sửa lớp" });
    expect(within(dialog).getByLabelText("Tên lớp")).toHaveValue("IELTS 6.5 Evening");
    await user.clear(within(dialog).getByLabelText(/Phòng/));
    await user.click(within(dialog).getByRole("button", { name: "Lưu thay đổi" }));
    await waitFor(() =>
      expect(patches).toEqual([
        {
          id: EVENING.id,
          body: {
            name: "IELTS 6.5 Evening",
            scheduleLabel: "Mon · Wed · Fri, 18:00",
            room: null,
          },
        },
      ]),
    );
    expect(await screen.findByText("Đã cập nhật lớp")).toBeInTheDocument();
  });

  it("creates a class with its schedule and room, then issues its first code and names it", async () => {
    const { user } = renderPage();
    await user.click(await screen.findByRole("button", { name: "Lớp mới" }));
    const dialog = await screen.findByRole("dialog", { name: "Lớp mới" });
    await user.type(within(dialog).getByLabelText("Tên lớp"), "  IELTS 7.0 Morning ");
    await user.type(within(dialog).getByLabelText(/Lịch học/), "Mon · Wed, 09:00");
    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));
    await waitFor(() =>
      expect(posts).toEqual([
        {
          name: "IELTS 7.0 Morning",
          scheduleLabel: "Mon · Wed, 09:00",
          room: null,
          selfJoinEnabled: true,
        },
      ]),
    );
    await waitFor(() =>
      expect(rotations).toEqual([
        {
          id: "018f0000-0000-7000-8000-0000000000c9",
          body: { expiresInDays: 30, maxUses: 40 },
        },
      ]),
    );
    expect(
      await screen.findByText("Đã tạo lớp · mã tham gia B4XN-9TRE"),
    ).toBeInTheDocument();
  });

  it("splits Active and Archived, and restores an archived class from its card", async () => {
    const { user } = renderPage();
    const filter = await screen.findByRole("group", { name: "Lọc theo trạng thái" });
    await user.click(within(filter).getByRole("button", { name: /Đã lưu trữ/ }));
    const archived = await card("Summer 2025");
    expect(lists.at(-1)?.get("status")).toBe("archived");
    await user.click(
      within(archived).getByRole("button", { name: "Khôi phục Summer 2025" }),
    );
    await waitFor(() =>
      expect(patches).toEqual([{ id: ARCHIVED.id, body: { archived: false } }]),
    );
  });

  it("says when the codes were replaced while that notice is unread, and marks it read when dismissed", async () => {
    const marks: unknown[] = [];
    const audiences: (string | null)[] = [];
    server.use(
      http.get(`${BASE}/me/notifications`, ({ request }) => {
        audiences.push(new URL(request.url).searchParams.get("audience"));
        return contractJson("/me/notifications", "get", 200, {
          items: [
            {
              id: "018f0000-0000-7000-8000-0000000000e1",
              kind: "join_codes.rotated",
              params: {
                count: 2,
                classNames: ["IELTS 6.5 Evening", "Kids Starters B"],
              },
              target: { route: "classes" },
              createdAt: "2026-10-05T03:00:00Z",
              readAt: null,
            },
          ],
          nextBefore: null,
        });
      }),
      http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
        marks.push(await request.json());
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user } = renderPage();
    const notice = await screen.findByText(
      "Mã tham gia của các lớp đã được thay ngày 05/10/2026. Hãy gửi mã mới cho học viên.",
    );
    await user.click(
      within(notice.closest("div")!.parentElement!).getByRole("button", {
        name: /Đóng|Ẩn/,
      }),
    );
    await waitFor(() =>
      expect(marks).toEqual([
        { audience: "teacher", ids: ["018f0000-0000-7000-8000-0000000000e1"] },
      ]),
    );
    expect(audiences).toContain("teacher");
    expect(screen.queryByText(/đã được thay ngày/)).toBeNull();
  });

  it("offers New class when there is nothing yet", async () => {
    serveList([], []);
    renderPage();
    expect(await screen.findByText("Chưa có lớp học nào.")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Lớp mới" })).toHaveLength(2);
    expect(screen.queryByRole("group", { name: "Lọc theo trạng thái" })).toBeNull();
  });

  it("says the list could not be read", async () => {
    server.use(
      http.get(`${BASE}/teacher/classes`, () =>
        HttpResponse.json(
          {
            error: {
              code: "INTERNAL",
              message: "Máy chủ gặp lỗi.",
              requestId: "018f0000-0000-7000-8000-0000000000f1",
            },
          },
          { status: 500 },
        ),
      ),
    );
    renderPage();
    expect(
      await screen.findByText("Không tải được danh sách lớp."),
    ).toBeInTheDocument();
  });
});
