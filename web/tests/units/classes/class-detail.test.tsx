import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ClassDetailPage from "@/features/classes/pages/ClassDetailPage";
import { AddMemberDialog } from "@/features/classes/components/AddMemberDialog";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CLASS_ID = "018f0000-0000-7000-8000-0000000000c1";
const MINH = "018f0000-0000-7000-8000-0000000000e1";
const HAN = "018f0000-0000-7000-8000-0000000000e2";

const klass = {
  id: CLASS_ID,
  name: "IELTS Foundation — Lớp tối T3/T5",
  description: null,
  studentCount: 2,
  openAssignmentCount: 1,
  archivedAt: null,
  selfJoinEnabled: true,
  joinCode: null,
  createdAt: "2026-06-01T00:00:00Z",
};

const stats = (over: Record<string, unknown>) => ({
  submittedCount: 0,
  score: null,
  flaggedCount: 0,
  activity: { live: false, lastAttemptAt: null },
  ...over,
});

beforeEach(() => {
  server.use(
    http.get(`${BASE}/admin/classes/${CLASS_ID}`, () =>
      contractJson("/admin/classes/{id}", "get", 200, klass),
    ),
    http.get(`${BASE}/admin/classes/${CLASS_ID}/members`, () =>
      contractJson("/admin/classes/{id}/members", "get", 200, {
        items: [
          {
            userId: MINH,
            fullName: "Nguyễn Đức Minh",
            email: "minh.nguyen@gmail.com",
            joinedVia: "admin",
            joinedAt: "2026-07-12T01:00:00Z",
            joinCodeHint: null,
            stats: stats({
              submittedCount: 14,
              score: { earned: 86, total: 100, pendingManual: 0 },
            }),
          },
          {
            userId: HAN,
            fullName: "Phạm Gia Hân",
            email: "han.pham@gmail.com",
            joinedVia: "join_code",
            joinedAt: "2026-08-29T01:12:00Z",
            joinCodeHint: "P9QR",
            stats: stats({}),
          },
        ],
        page: 1,
        pageSize: 20,
        total: 2,
      }),
    ),
    http.get(`${BASE}/admin/students`, () =>
      contractJson("/admin/students", "get", 200, {
        items: [
          {
            id: MINH,
            email: "minh.nguyen@gmail.com",
            fullName: "Nguyễn Đức Minh",
            hasPassword: true,
            linkedProviders: [],
            mustChangePassword: false,
            createdAt: "2026-06-01T00:00:00Z",
            disabledAt: null,
            classes: [
              {
                id: CLASS_ID,
                name: klass.name,
                joinedVia: "admin",
                joinedAt: "2026-07-12T01:00:00Z",
              },
            ],
            stats: stats({}),
          },
          {
            id: HAN,
            email: "han.pham@gmail.com",
            fullName: "Phạm Gia Hân",
            hasPassword: false,
            linkedProviders: ["google"],
            mustChangePassword: false,
            createdAt: "2026-06-01T00:00:00Z",
            disabledAt: null,
            classes: [],
            stats: stats({}),
          },
        ],
        facets: { total: 2, activeLast7Days: 1 },
        page: 1,
        pageSize: 20,
        total: 2,
      }),
    ),
  );
});

function renderDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/classes/:id", element: <ClassDetailPage /> }],
    { initialEntries: [`/admin/classes/${CLASS_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the class roster (G-06)", () => {
  it("carries the same figures as the students table, and one search box", async () => {
    renderDetail();

    const row = (await screen.findByText("Nguyễn Đức Minh")).closest("tr")!;
    expect(within(row).getByText("14")).toBeInTheDocument();
    expect(within(row).getByText("86%")).toBeInTheDocument();
    const han = screen.getByText("Phạm Gia Hân").closest("tr")!;
    expect(within(han).getByText("—")).toBeInTheDocument();
    expect(screen.getByRole("searchbox", { name: "Tìm học viên" })).toBeInTheDocument();
  });

  it("removes through the row menu, after a confirm", async () => {
    const user = renderDetail();
    const row = (await screen.findByText("Nguyễn Đức Minh")).closest("tr")!;

    await user.click(within(row).getByRole("button", { name: "Thao tác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Xoá khỏi lớp" }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/Nguyễn Đức Minh/)).toBeInTheDocument();
  });
});

describe("adding a student who already belongs", () => {
  it("reads membership off the student row, not the roster page on screen", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <AddMemberDialog classId={CLASS_ID} open onOpenChange={() => {}} />
      </QueryClientProvider>,
    );

    const minh = (await screen.findByText("Nguyễn Đức Minh")).closest("li")!;
    expect(within(minh).getByText("đã trong lớp")).toBeInTheDocument();
    expect(within(minh).queryByRole("button", { name: "Thêm" })).toBeNull();
    const han = screen.getByText("Phạm Gia Hân").closest("li")!;
    expect(within(han).getByRole("button", { name: "Thêm" })).toBeInTheDocument();
  });
});
