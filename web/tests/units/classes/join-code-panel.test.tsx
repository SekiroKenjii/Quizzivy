import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { http, HttpResponse } from "msw";
import { JoinCodePanel } from "@/features/classes/components/JoinCodePanel";
import type { Class, JoinCode } from "@/features/classes/api";
import { Toaster } from "@/components/ui/sonner";
import { formatDate } from "@/lib/i18n/datetime";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CLASS_ID = "018f0000-0000-7000-8000-0000000000c1";
const ORIGIN = window.location.origin;
const TEN_DAYS_ON = new Date(Date.now() + 10 * 86_400_000);
TEN_DAYS_ON.setUTCHours(16, 59, 59, 0);
const IN_TEN_DAYS = TEN_DAYS_ON.toISOString();
const DAYS_LEFT = Math.floor((TEN_DAYS_ON.getTime() - Date.now()) / 86_400_000);

const CODE: JoinCode = {
  code: "K7QM-2PXA",
  hint: "2PXA",
  legacy: false,
  expiresAt: IN_TEN_DAYS,
  maxUses: 40,
  usesCount: 3,
};
const KLASS: Class = {
  id: CLASS_ID,
  name: "IELTS 6.5 Evening",
  description: null,
  scheduleLabel: null,
  room: null,
  studentCount: 24,
  openAssignmentCount: 1,
  selfJoinEnabled: true,
  archivedAt: null,
  joinCode: {
    hint: CODE.hint,
    expiresAt: CODE.expiresAt,
    maxUses: CODE.maxUses,
    usesCount: CODE.usesCount,
  },
  createdAt: "2026-08-03T03:00:00Z",
};

let reads = 0;
let readable: JoinCode | null = CODE;
let rotations: unknown[] = [];
let revokes = 0;
let patches: unknown[] = [];

beforeEach(() => {
  reads = 0;
  readable = CODE;
  rotations = [];
  revokes = 0;
  patches = [];
  server.use(
    http.get(`${BASE}/teacher/classes/${CLASS_ID}/join-code`, () => {
      reads += 1;
      if (readable === null)
        return HttpResponse.json(
          {
            error: {
              code: "NOT_FOUND",
              message: "Không có mã.",
              requestId: "018f0000-0000-7000-8000-0000000000f1",
            },
          },
          { status: 404 },
        );
      return contractJson("/teacher/classes/{id}/join-code", "get", 200, readable);
    }),
    http.post(`${BASE}/teacher/classes/${CLASS_ID}/join-code`, async ({ request }) => {
      rotations.push(await request.json());
      return contractJson("/teacher/classes/{id}/join-code", "post", 201, {
        code: "B4XN-9TRE",
        expiresAt: IN_TEN_DAYS,
        maxUses: 40,
      });
    }),
    http.delete(`${BASE}/teacher/classes/${CLASS_ID}/join-code`, () => {
      revokes += 1;
      return new HttpResponse(null, { status: 204 });
    }),
    http.patch(`${BASE}/teacher/classes/${CLASS_ID}`, async ({ request }) => {
      const body = await request.json();
      patches.push(body);
      return contractJson("/teacher/classes/{id}", "patch", 200, KLASS);
    }),
  );
});

function renderPanel(klass: Class = KLASS, canWrite = true) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <JoinCodePanel klass={klass} canWrite={canWrite} />
      </MemoryRouter>
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the join code card (DG-01)", () => {
  it("shows the code in full, when it expires and how much it was used, and never says it is shown once", async () => {
    renderPanel();
    expect(await screen.findByText("K7QM-2PXA", { selector: "p" })).toBeVisible();
    expect(
      screen.getByText(`${formatDate(IN_TEN_DAYS, "vi")}, 23:59`, { exact: false }),
    ).toBeVisible();
    expect(screen.getByText(`· còn ${DAYS_LEFT} ngày`)).toBeVisible();
    expect(screen.getByText("3 / 40")).toBeVisible();
    expect(screen.queryByText(/một lần/)).toBeNull();
    expect(reads).toBe(1);
  });

  it("copies the link in the one format and the bare code", async () => {
    const user = renderPanel();
    await screen.findByText("K7QM-2PXA", { selector: "p" });
    await user.click(screen.getByRole("button", { name: "Chép liên kết" }));
    expect(await navigator.clipboard.readText()).toBe(`${ORIGIN}/join/K7QM-2PXA`);
    await user.click(screen.getByRole("button", { name: "Chép mã" }));
    expect(await navigator.clipboard.readText()).toBe("K7QM-2PXA");
    expect(await screen.findByText("Đã sao chép mã")).toBeInTheDocument();
  });

  it("opens the QR for the code", async () => {
    const user = renderPanel();
    await screen.findByText("K7QM-2PXA", { selector: "p" });
    await user.click(screen.getByRole("button", { name: "Mã QR" }));
    const dialog = await screen.findByRole("dialog", { name: "IELTS 6.5 Evening" });
    expect(within(dialog).getByText(`${ORIGIN}/join/K7QM-2PXA`)).toBeVisible();
  });
});

describe("a new code", () => {
  it("asks for its life and its uses, 30 days and 40 by default, refuses an impossible limit, and gives focus back", async () => {
    const user = renderPanel();
    await screen.findByText("K7QM-2PXA", { selector: "p" });
    const opener = screen.getByRole("button", { name: "Mã mới" });
    await user.click(opener);
    const dialog = await screen.findByRole("dialog", { name: "Tạo mã mới?" });
    const uses = within(dialog).getByLabelText("Số lượt tối đa");
    expect(uses).toHaveValue(40);
    expect(
      within(dialog).getByRole("combobox", { name: "Hết hạn sau" }),
    ).toHaveTextContent("30 ngày");
    await user.clear(uses);
    await user.type(uses, "0");
    await user.click(within(dialog).getByRole("button", { name: "Tạo mã mới" }));
    expect(within(dialog).getByText("Hãy nhập số nguyên từ 1 đến 1.000")).toBeVisible();
    expect(rotations).toEqual([]);

    await user.click(within(dialog).getByRole("combobox", { name: "Hết hạn sau" }));
    await user.click(await screen.findByRole("option", { name: "7 ngày" }));
    await user.clear(uses);
    await user.type(uses, "100");
    await user.click(within(dialog).getByRole("button", { name: "Tạo mã mới" }));
    await waitFor(() =>
      expect(rotations).toEqual([{ expiresInDays: 7, maxUses: 100 }]),
    );
    expect(await screen.findByText("Mã tham gia mới: B4XN-9TRE")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(opener).toHaveFocus();
  });

  it("offers Create code for a class with none, and reads no code", async () => {
    const user = renderPanel({ ...KLASS, joinCode: null });
    expect(
      screen.getByText("Chưa có mã đang dùng. Chỉ bạn thêm được học viên vào lớp."),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Tạo mã" }));
    expect(
      await screen.findByRole("dialog", { name: "Tạo mã tham gia" }),
    ).toBeInTheDocument();
    expect(reads).toBe(0);
  });
});

describe("stopping a code", () => {
  it("asks first, then stops it", async () => {
    const user = renderPanel();
    await screen.findByText("K7QM-2PXA", { selector: "p" });
    await user.click(screen.getByRole("button", { name: "Ngừng" }));
    const dialog = await screen.findByRole("dialog", { name: "Ngừng mã này?" });
    expect(revokes).toBe(0);
    await user.click(within(dialog).getByRole("button", { name: "Ngừng mã" }));
    await waitFor(() => expect(revokes).toBe(1));
    expect(await screen.findByText("Đã ngừng mã tham gia")).toBeInTheDocument();
  });
});

describe("the Students can join switch", () => {
  it("pauses joining without touching the code, keeping focus while it saves", async () => {
    const user = renderPanel();
    const toggle = screen.getByRole("switch", { name: "Cho học viên tham gia" });
    await user.click(toggle);
    await waitFor(() => expect(patches).toEqual([{ selfJoinEnabled: false }]));
    expect(toggle).toHaveFocus();
    expect(await screen.findByText("Đã tạm ngừng tham gia")).toBeInTheDocument();
  });

  it("says what a paused class means", () => {
    renderPanel({ ...KLASS, selfJoinEnabled: false });
    expect(
      screen.getByText(
        "Đang tạm ngừng tham gia. Mã vẫn còn nhưng không ai dùng được cho đến khi bạn bật lại.",
      ),
    ).toBeVisible();
  });
});

describe("a code that cannot be read", () => {
  it("shows its last four characters and says to make a new one", async () => {
    readable = { ...CODE, code: null, legacy: true };
    renderPanel();
    expect(
      await screen.findByText(
        "Không đọc lại được mã này. Hãy tạo mã mới để gửi cho học viên.",
      ),
    ).toBeVisible();
    expect(screen.getByText("••••-2PXA")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Chép liên kết" })).toBeNull();
  });

  it("is masked, with nothing to change, for a reader who may not change classes", () => {
    renderPanel(KLASS, false);
    expect(screen.getByText("••••-2PXA")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Mã mới" })).toBeNull();
    expect(
      screen.getByRole("switch", { name: "Cho học viên tham gia" }),
    ).toBeDisabled();
    expect(reads).toBe(0);
  });
});
