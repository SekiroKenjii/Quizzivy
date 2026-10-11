import { afterEach, describe, expect, it, vi } from "vitest";
import { useEffect, useRef } from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryRouter,
  Outlet,
  RouterProvider,
  useLocation,
  useMatches,
} from "react-router";
import { http, HttpResponse } from "msw";
import i18n from "@/lib/i18n";
import { Toaster } from "@/components/ui/sonner";
import { NotificationBell } from "@/features/notifications/components/NotificationBell";
import { audienceOf, describeNotification } from "@/features/notifications/kinds";
import type { Notification, NotificationPage } from "@/features/notifications/api";
import { server } from "@tests/support/server";
import { viewport } from "@tests/support/viewport";
import {
  BASE,
  assignmentId,
  errorResponse,
  id,
  listResponse,
  notices,
  title,
} from "./support";

type Audience = "teacher" | "student";

function Shell({
  audience,
  unread,
  canGrade,
}: Readonly<{ audience: Audience; unread: number; canGrade: boolean }>) {
  const { pathname } = useLocation();
  const route = useMatches().at(-1)?.id;
  const main = useRef<HTMLElement>(null);
  const shown = useRef(route);
  useEffect(() => {
    if (route === shown.current) return;
    shown.current = route;
    main.current?.focus();
  }, [route]);
  return (
    <>
      <NotificationBell audience={audience} unread={unread} canGrade={canGrade} />
      <main ref={main} tabIndex={-1}>
        <p data-testid="where">{pathname}</p>
        <Outlet />
      </main>
    </>
  );
}

function serveList(items: Notification[], nextBefore: string | null = null) {
  const marks: unknown[] = [];
  server.use(
    http.get(`${BASE}/me/notifications`, () => listResponse({ items, nextBefore })),
    http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
      marks.push(await request.json());
      return new HttpResponse(null, { status: 204 });
    }),
  );
  return marks;
}

function renderBell(
  audience: Audience,
  over: Partial<{ unread: number; canGrade: boolean; at: string }> = {},
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const element = (
    <Shell
      audience={audience}
      unread={over.unread ?? 0}
      canGrade={over.canGrade ?? true}
    />
  );
  const router = createMemoryRouter(
    [
      {
        element,
        children: [
          { id: "teacher-assignment", path: "/teacher/assignments/:id" },
          { id: "student-assignment", path: "/app/assignments/:id" },
          { id: "elsewhere", path: "*" },
        ],
      },
    ],
    { initialEntries: [over.at ?? "/start"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), client, router };
}

function textOf(notification: Notification, canGrade = true): string {
  return describeNotification(notification, i18n.getFixedT("vi"), "vi", { canGrade })!
    .text;
}

function of(audience: Audience): Notification[] {
  return notices.filter((notice) => audienceOf(notice.kind) === audience);
}

async function openBell(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: /^Thông báo/ }));
  return screen.findByRole("dialog", { name: "Thông báo" });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("the bell's dot (DG-82)", () => {
  it("shows only while something is unread, and says how many", () => {
    serveList([]);
    renderBell("teacher", { unread: 3 });
    const bell = screen.getByRole("button", { name: "Thông báo, 3 chưa đọc" });
    expect(bell.querySelector("[data-slot='bell-dot']")).not.toBeNull();
  });

  it("is not drawn when nothing is unread", () => {
    serveList([]);
    renderBell("student", { unread: 0 });
    const bell = screen.getByRole("button", { name: "Thông báo" });
    expect(bell.querySelector("[data-slot='bell-dot']")).toBeNull();
  });
});

describe("the list", () => {
  it("lists a teacher's kinds and never a student's", async () => {
    serveList(notices);
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    for (const notice of of("teacher"))
      expect(await within(panel).findByText(textOf(notice))).toBeInTheDocument();
    for (const notice of of("student"))
      expect(within(panel).queryByText(textOf(notice))).toBeNull();
  });

  it("lists a student's kinds and never a teacher's", async () => {
    serveList(notices);
    const { user } = renderBell("student");
    const panel = await openBell(user);
    for (const notice of of("student"))
      expect(await within(panel).findByText(textOf(notice))).toBeInTheDocument();
    for (const notice of of("teacher"))
      expect(within(panel).queryByText(textOf(notice))).toBeNull();
  });

  it("draws a flagged attempt in the danger tone", async () => {
    const flagged = notices.find((notice) => notice.kind === "attempt.flagged")!;
    serveList([flagged]);
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    const row = (await within(panel).findByText(textOf(flagged))).closest("button")!;
    expect(row).toHaveAttribute("data-tone", "danger");
    expect(row.querySelector(".bg-danger-soft.text-danger-ink")).not.toBeNull();
  });

  it("says when everything has been read", async () => {
    serveList([]);
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    expect(await within(panel).findByText("Bạn đã xem hết thông báo.")).toBeVisible();
  });

  it("says when the list cannot be read, and reads it again on Retry", async () => {
    let asked = 0;
    server.use(
      http.get(`${BASE}/me/notifications`, () => {
        asked += 1;
        return asked === 1
          ? errorResponse()
          : listResponse({ items: of("teacher"), nextBefore: null });
      }),
    );
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    const alert = await within(panel).findByRole("alert");
    expect(alert).toHaveTextContent("Không tải được thông báo.");
    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));
    expect(
      await within(panel).findByText(textOf(of("teacher")[0]!)),
    ).toBeInTheDocument();
  });

  it("loads the next page on Load more", async () => {
    const first = of("teacher").slice(0, 1);
    const second = of("teacher").slice(1, 2);
    const asked: (string | null)[] = [];
    server.use(
      http.get(`${BASE}/me/notifications`, ({ request }) => {
        const before = new URL(request.url).searchParams.get("before");
        asked.push(before);
        const page: NotificationPage =
          before === null
            ? { items: first, nextBefore: first[0]!.id }
            : { items: second, nextBefore: null };
        return listResponse(page);
      }),
    );
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    await user.click(await within(panel).findByRole("button", { name: "Xem thêm" }));
    expect(await within(panel).findByText(textOf(second[0]!))).toBeInTheDocument();
    expect(asked).toEqual([null, first[0]!.id]);
    expect(within(panel).queryByRole("button", { name: "Xem thêm" })).toBeNull();
  });
});

function held() {
  let release: () => void = () => {};
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  return { gate, release };
}

describe("keeping focus while a request is out (QA-43-1, QA-47-1)", () => {
  it("keeps focus on Mark all read while it marks, and after nothing is left unread", async () => {
    const marks: unknown[] = [];
    let read = false;
    const { gate, release } = held();
    server.use(
      http.get(`${BASE}/me/notifications`, () =>
        listResponse({
          items: of("teacher").map((notice) => ({
            ...notice,
            readAt: read ? "2026-10-01T13:00:00Z" : null,
          })),
          nextBefore: null,
        }),
      ),
      http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
        marks.push(await request.json());
        await gate;
        read = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    await within(panel).findByText(textOf(of("teacher")[0]!));
    const all = within(panel).getByRole("button", { name: "Đánh dấu đã đọc hết" });
    expect(all).not.toHaveAttribute("aria-disabled");

    await user.click(all);
    await waitFor(() => expect(all).toHaveAttribute("aria-disabled", "true"));
    expect(all).toBeEnabled();
    expect(all).toHaveFocus();
    await user.click(all);

    release();
    await waitFor(() => expect(within(panel).queryByText("Chưa đọc")).toBeNull());
    expect(all).toHaveAttribute("aria-disabled", "true");
    expect(all).toBeEnabled();
    expect(all).toHaveFocus();
    expect(marks).toEqual([{}]);
  });

  it("keeps focus on Load more while it loads, then moves it to the first row it added", async () => {
    const first = of("teacher").slice(0, 1);
    const second = of("teacher").slice(1, 3);
    const { gate, release } = held();
    server.use(
      http.get(`${BASE}/me/notifications`, async ({ request }) => {
        if (new URL(request.url).searchParams.get("before") === null)
          return listResponse({ items: first, nextBefore: first[0]!.id });
        await gate;
        return listResponse({ items: second, nextBefore: null });
      }),
    );
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    const more = await within(panel).findByRole("button", { name: "Xem thêm" });
    more.focus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(more).toHaveAttribute("aria-disabled", "true"));
    expect(more).toBeEnabled();
    expect(more).toHaveFocus();

    release();
    const added = await within(panel).findByText(textOf(second[0]!));
    await waitFor(() => expect(added.closest("button")).toHaveFocus());
  });
});

describe("reading", () => {
  it("marks every notification of the account read and keeps focus in the panel", async () => {
    const marks = serveList(of("teacher"));
    const { user } = renderBell("teacher", { unread: 4 });
    const panel = await openBell(user);
    const all = await within(panel).findByRole("button", {
      name: "Đánh dấu đã đọc hết",
    });
    await user.click(all);
    await waitFor(() => expect(marks).toEqual([{}]));
    expect(panel.contains(document.activeElement)).toBe(true);
  });

  it("offers Mark all read while the dot is lit by notifications this bell does not list (DG-170)", async () => {
    const marks = serveList(of("student"));
    const { user } = renderBell("teacher", { unread: 2 });
    const panel = await openBell(user);
    expect(await within(panel).findByText("Bạn đã xem hết thông báo.")).toBeVisible();
    const all = within(panel).getByRole("button", { name: "Đánh dấu đã đọc hết" });
    expect(all).not.toHaveAttribute("aria-disabled");
    await user.click(all);
    await waitFor(() => expect(marks).toEqual([{}]));
  });

  it("sends nothing from Mark all read when nothing is unread", async () => {
    const marks = serveList(
      of("teacher").map((notice) => ({ ...notice, readAt: "2026-10-01T13:00:00Z" })),
    );
    const { user } = renderBell("teacher", { unread: 0 });
    const panel = await openBell(user);
    await within(panel).findByText(textOf(of("teacher")[0]!));
    const all = within(panel).getByRole("button", { name: "Đánh dấu đã đọc hết" });
    expect(all).toHaveAttribute("aria-disabled", "true");
    await user.click(all);
    expect(marks).toEqual([]);
  });

  it.each(["teacher", "student"] as const)(
    "sends each %s kind where its notification points and marks it read",
    async (audience) => {
      const rows = of(audience);
      for (const notice of rows) {
        const marks = serveList(rows);
        const { user, router } = renderBell(audience);
        const panel = await openBell(user);
        await user.click(await within(panel).findByText(textOf(notice)));
        const to = describeNotification(notice, i18n.getFixedT("vi"), "vi", {
          canGrade: true,
        })!.to;
        await waitFor(() => expect(router.state.location.pathname).toBe(to));
        await waitFor(() => expect(marks).toEqual([{ ids: [notice.id] }]));
        expect(screen.queryByRole("dialog")).toBeNull();
        document.body.innerHTML = "";
      }
    },
  );

  it("sends a hand-in with papers to grade to Grading, or to its assignment for a reader who cannot grade", async () => {
    const submitted = {
      ...notices[0]!,
      params: { title, count: 8, toGrade: 6 },
    };
    serveList([submitted]);
    const grading = renderBell("teacher", { canGrade: true });
    await grading.user.click(
      await within(await openBell(grading.user)).findByText(textOf(submitted)),
    );
    await waitFor(() =>
      expect(grading.router.state.location.pathname).toBe("/teacher/grading"),
    );
    document.body.innerHTML = "";

    serveList([submitted]);
    const other = renderBell("teacher", { canGrade: false });
    await other.user.click(
      await within(await openBell(other.user)).findByText(textOf(submitted, false)),
    );
    await waitFor(() =>
      expect(other.router.state.location.pathname).toBe(
        `/teacher/assignments/${assignmentId}`,
      ),
    );
  });

  it("still goes where the row points when marking it read fails", async () => {
    const notice = of("teacher")[0]!;
    server.use(
      http.get(`${BASE}/me/notifications`, () =>
        listResponse({ items: [notice], nextBefore: null }),
      ),
      http.post(`${BASE}/me/notifications/read`, () => errorResponse()),
    );
    const { user, router } = renderBell("teacher");
    await user.click(await within(await openBell(user)).findByText(textOf(notice)));
    await waitFor(() => expect(router.state.location.pathname).not.toBe("/start"));
  });

  it("marks a read row nothing more", async () => {
    const read = { ...of("teacher")[0]!, readAt: "2026-10-01T13:00:00Z", id: id(77) };
    const marks = serveList([read]);
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    expect(within(panel).queryByText("Chưa đọc")).toBeNull();
    await user.click(await within(panel).findByText(textOf(read)));
    expect(marks).toEqual([]);
  });
});

describe("closing", () => {
  it("returns focus to the bell when the popover closes with Escape", async () => {
    serveList([]);
    const { user } = renderBell("teacher");
    await openBell(user);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: /^Thông báo/ })).toHaveFocus();
  });

  it("returns focus to the bell when the popover closes from the bell", async () => {
    serveList([]);
    const { user } = renderBell("teacher");
    await openBell(user);
    await user.click(screen.getByRole("button", { name: /^Thông báo/ }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: /^Thông báo/ })).toHaveFocus();
  });

  it.each(["Escape", "the close button"])(
    "returns focus to the bell when the phone's sheet closes with %s",
    async (how) => {
      viewport("phone");
      serveList([]);
      const { user } = renderBell("student");
      const sheet = await openBell(user);
      expect(within(sheet).getAllByRole("heading", { name: "Thông báo" })).toHaveLength(
        1,
      );
      if (how === "Escape") await user.keyboard("{Escape}");
      else await user.click(within(sheet).getByRole("button", { name: "Đóng" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      expect(screen.getByRole("button", { name: /^Thông báo/ })).toHaveFocus();
    },
  );

  it.each(["desktop", "phone"] as const)(
    "leaves focus to the new page after a row sends the reader there, on a %s",
    async (width) => {
      viewport(width);
      const notice = of("student")[0]!;
      serveList([notice]);
      const { user, router } = renderBell("student");
      await user.click(await within(await openBell(user)).findByText(textOf(notice)));
      await waitFor(() => expect(router.state.location.pathname).not.toBe("/start"));
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      expect(screen.getByRole("main")).toHaveFocus();
    },
  );

  it("returns focus to the bell from the popover after a row on a phone sent the reader on", async () => {
    const width = viewport("phone");
    const notice = of("student")[0]!;
    serveList([notice]);
    const { user, router } = renderBell("student");
    await user.click(await within(await openBell(user)).findByText(textOf(notice)));
    await waitFor(() => expect(router.state.location.pathname).not.toBe("/start"));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    act(() => width.resize("desktop"));
    await openBell(user);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: /^Thông báo/ })).toHaveFocus();
  });

  it.each([
    ["teacher", "assignment.closing", "/teacher/assignments"],
    ["student", "assignment.due_soon", "/app/assignments"],
  ] as const)(
    "puts focus on the page when a %s row only changes the route's parameter (QA-45b-1)",
    async (audience, kind, base) => {
      const notice = notices.find((candidate) => candidate.kind === kind)!;
      serveList([notice]);
      const from = `${base}/${id(4242)}`;
      const { user, router } = renderBell(audience, { at: from });
      await user.click(await within(await openBell(user)).findByText(textOf(notice)));
      await waitFor(() =>
        expect(router.state.location.pathname).toBe(`${base}/${assignmentId}`),
      );
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
    },
  );
});

describe("what the panel draws and says", () => {
  it("draws a neutral kind's icon in the muted text colour, as both decks do (QA-45b-2)", async () => {
    const joined = notices.find((notice) => notice.kind === "class.joined")!;
    serveList([joined]);
    const { user } = renderBell("teacher");
    const panel = await openBell(user);
    const row = (await within(panel).findByText(textOf(joined))).closest("button")!;
    expect(row).toHaveAttribute("data-tone", "neutral");
    const tile = row.querySelector("span[aria-hidden='true']")!;
    expect(tile.className.split(" ")).toEqual(
      expect.arrayContaining(["bg-muted", "text-muted-fg"]),
    );
    expect(tile.className.split(" ")).not.toContain("text-fg");
  });

  it("says when Mark all read fails, and keeps focus on it", async () => {
    server.use(
      http.get(`${BASE}/me/notifications`, () =>
        listResponse({ items: of("teacher"), nextBefore: null }),
      ),
      http.post(`${BASE}/me/notifications/read`, () => errorResponse()),
    );
    const { user } = renderBell("teacher", { unread: 2 });
    const panel = await openBell(user);
    const all = await within(panel).findByRole("button", {
      name: "Đánh dấu đã đọc hết",
    });
    await user.click(all);
    expect(
      await screen.findByText("Chưa đánh dấu được thông báo là đã đọc. Hãy thử lại."),
    ).toBeInTheDocument();
    expect(all).toHaveFocus();
    await waitFor(() => expect(all).not.toHaveAttribute("aria-disabled"));
  });
});
