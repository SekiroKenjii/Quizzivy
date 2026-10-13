import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contentWidth } from "@tests/support/contentWidth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import {
  BASE,
  REQUEST_ID,
  THIS_DEVICE,
  listsSessions,
  renderSettings,
  signIn,
} from "./support";
import "@/lib/i18n";

const LOAD = { timeout: 5000 };

type Usage = components["schemas"]["AiMonthUsage"];
type CreditRequest = components["schemas"]["AiCreditRequest"];

const PENDING: CreditRequest = {
  id: "019535d9-3df7-79fb-b466-fa907fa18101",
  userId: teacherUser.id,
  month: "2026-10-01",
  amount: 5000,
  reason: "Đề thi thử cuối kỳ cho ba lớp.",
  status: "pending",
  createdAt: "2026-10-12T03:00:00Z",
};

const USAGE: Usage = {
  month: "2026-10-01",
  used: 12_450,
  allowance: 20_000,
  left: 7_550,
  paused: false,
  requests: true,
  pending: null,
  categories: [
    { category: "generation", credits: 6_200 },
    { category: "grading", credits: 2_100 },
    { category: "explanations", credits: 900 },
    { category: "imports", credits: 2_400 },
    { category: "checks", credits: 850 },
  ],
};

function aiError(status: number, code: string, message: string) {
  return HttpResponse.json(
    { error: { code, message, requestId: REQUEST_ID } },
    { status },
  );
}

function serves({
  ai = true,
  usage = USAGE,
}: { ai?: boolean; usage?: Usage | Response } = {}) {
  const reads = { status: 0, usage: 0 };
  server.use(
    http.get(`${BASE}/public/status`, () => {
      reads.status += 1;
      return contractJson("/public/status", "get", 200, { maintenance: null, ai });
    }),
    http.get(`${BASE}/teacher/ai/usage`, () => {
      reads.usage += 1;
      return usage instanceof Response
        ? usage.clone()
        : contractJson("/teacher/ai/usage", "get", 200, usage);
    }),
  );
  return reads;
}

function nav() {
  return within(screen.getByRole("navigation", { name: "Mục cài đặt" }));
}

beforeEach(() => {
  contentWidth(1080);
  listsSessions([THIS_DEVICE]);
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("Settings › AI assistant", () => {
  it("is the sixth section, after Appearance, for a holder of ai.use", () => {
    signIn(teacherUser);
    const reads = serves();
    renderSettings();

    expect(
      nav()
        .getAllByRole("link")
        .map((link) => link.textContent),
    ).toEqual([
      "Hồ sơ",
      "Đăng nhập & bảo mật",
      "Thông báo",
      "Mặc định khi giao bài",
      "Giao diện",
      "Trợ lý AI",
    ]);
    expect(nav().getByRole("link", { name: "Trợ lý AI" })).toHaveAttribute(
      "href",
      "/teacher/settings/ai",
    );
    expect(reads).toEqual({ status: 0, usage: 0 });
  });

  it("draws the month's usage, its reset date and the five rows", async () => {
    signIn(teacherUser);
    serves();
    renderSettings("/teacher/settings/ai");

    const card = await screen.findByRole(
      "region",
      { name: "Mức dùng AI tháng này" },
      LOAD,
    );
    expect(
      await within(card).findByText(
        "Trung tâm cấp cho mỗi giáo viên một hạn mức hằng tháng. Hạn mức được làm mới vào 1 tháng 11.",
      ),
    ).toBeVisible();
    expect(await within(card).findByText("12.450")).toBeVisible();
    expect(within(card).getByText("trên 20.000 tín dụng")).toBeVisible();
    expect(within(card).getByText("Còn 7.550")).toBeVisible();
    expect(
      within(card).getByRole("progressbar", {
        name: "Đã dùng 12.450 trên 20.000 tín dụng",
      }),
    ).toHaveAttribute("aria-valuenow", "62");
    const rows = within(
      within(card).getByRole("list", { name: "Mức dùng theo tính năng" }),
    ).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      "Tạo câu hỏi và bản biến thể6.200",
      "Gợi ý chấm điểm2.100",
      "Giải thích và phương án sai900",
      "Đọc tệp nhập2.400",
      "Kiểm tra, danh sách từ và nhận xét850",
    ]);
  });

  it("starts the preferences from the defaults: the interface's language, tags and answers on, imports off", async () => {
    signIn(teacherUser);
    serves();
    renderSettings("/teacher/settings/ai");

    const prefs = await screen.findByRole("region", { name: "Tùy chọn" }, LOAD);
    expect(within(prefs).getByRole("button", { name: "Tiếng Việt" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      within(prefs).getByRole("switch", { name: "Gắn thẻ bản nháp AI" }),
    ).toBeChecked();
    expect(
      within(prefs).getByRole("switch", { name: "Cho AI đọc bài làm của học viên" }),
    ).toBeChecked();
    expect(
      within(prefs).getByRole("switch", { name: "Đọc tệp nhập bằng AI" }),
    ).not.toBeChecked();
  });

  it("names no credit figure for reading imports, which the product does not meter", async () => {
    signIn(teacherUser);
    serves();
    renderSettings("/teacher/settings/ai");

    const prefs = await screen.findByRole("region", { name: "Tùy chọn" }, LOAD);
    expect(
      within(prefs).getByRole("switch", { name: "Đọc tệp nhập bằng AI" }),
    ).toHaveAccessibleDescription("Xử lý PDF quét, ảnh chụp đề in và bố cục lạ.");
    expect(within(prefs).queryByText(/tín dụng/)).toBeNull();
  });

  it("saves a switch as the whole ai object and shows it at once", async () => {
    signIn({ ...teacherUser, preferences: { ai: { language: "en" } } });
    serves();
    const bodies: unknown[] = [];
    server.use(
      http.patch(`${BASE}/me/preferences`, async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        bodies.push(body);
        return HttpResponse.json(body);
      }),
    );
    renderSettings("/teacher/settings/ai");

    const prefs = await screen.findByRole("region", { name: "Tùy chọn" }, LOAD);
    expect(within(prefs).getByRole("button", { name: "English" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await userEvent.click(
      within(prefs).getByRole("switch", { name: "Đọc tệp nhập bằng AI" }),
    );

    await waitFor(() =>
      expect(bodies).toEqual([
        {
          ai: {
            language: "en",
            tagDrafts: true,
            readStudentAnswers: true,
            readImports: true,
          },
        },
      ]),
    );
    expect(await screen.findByText("Đã lưu tùy chọn.", {}, LOAD)).toBeVisible();
    expect(useAuthStore.getState().user?.preferences?.ai?.readImports).toBe(true);
    expect(
      within(prefs).getByRole("switch", { name: "Đọc tệp nhập bằng AI" }),
    ).toBeChecked();
  });

  it("restores a switch whose save failed", async () => {
    signIn(teacherUser);
    serves();
    server.use(
      http.patch(`${BASE}/me/preferences`, () =>
        aiError(500, "INTERNAL", "Máy chủ gặp lỗi."),
      ),
    );
    renderSettings("/teacher/settings/ai");

    const prefs = await screen.findByRole("region", { name: "Tùy chọn" }, LOAD);
    const tags = within(prefs).getByRole("switch", { name: "Gắn thẻ bản nháp AI" });
    await userEvent.click(tags);

    expect(await screen.findByText("Máy chủ gặp lỗi.", {}, LOAD)).toBeVisible();
    await waitFor(() => expect(tags).toBeChecked());
  });

  it("says one sentence and reads no usage while the server has no assistant", async () => {
    signIn(teacherUser);
    const reads = serves({ ai: false });
    renderSettings("/teacher/settings/ai");

    expect(
      await screen.findByText("AI chưa được bật cho trung tâm này.", {}, LOAD),
    ).toBeVisible();
    expect(nav().getByRole("link", { name: "Trợ lý AI" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.queryByRole("region", { name: "Mức dùng AI tháng này" })).toBeNull();
    expect(screen.queryByRole("region", { name: "Tùy chọn" })).toBeNull();
    expect(reads.usage).toBe(0);
  });

  it("says the same sentence when the usage read answers 501 AI_OFF", async () => {
    signIn(teacherUser);
    serves({ usage: aiError(501, "AI_OFF", "Trợ lý AI chưa được cấu hình.") });
    renderSettings("/teacher/settings/ai");

    expect(
      await screen.findByText("AI chưa được bật cho trung tâm này.", {}, LOAD),
    ).toBeVisible();
    expect(screen.queryByRole("region", { name: "Tùy chọn" })).toBeNull();
  });

  it("is not offered without ai.use, and its address falls back to Profile", async () => {
    signIn({
      ...teacherUser,
      permissions: teacherUser.permissions.filter((key) => key !== "ai.use"),
    });
    const reads = serves();
    const { router } = renderSettings("/teacher/settings/ai");

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/settings"),
    );
    expect(nav().queryByRole("link", { name: "Trợ lý AI" })).toBeNull();
    expect(reads).toEqual({ status: 0, usage: 0 });
  });

  it("offers a retry when the status cannot be read", async () => {
    signIn(teacherUser);
    serves();
    let fail = true;
    server.use(
      http.get(`${BASE}/public/status`, () =>
        fail
          ? aiError(500, "INTERNAL", "Máy chủ gặp lỗi.")
          : contractJson("/public/status", "get", 200, { maintenance: null, ai: true }),
      ),
    );
    renderSettings("/teacher/settings/ai");

    const alert = await screen.findByRole("alert", {}, LOAD);
    expect(alert).toHaveTextContent("Không tải được trạng thái của trợ lý AI.");
    fail = false;
    await userEvent.click(within(alert).getByRole("button", { name: "Thử lại" }));
    expect(
      await screen.findByRole("region", { name: "Mức dùng AI tháng này" }, LOAD),
    ).toBeVisible();
  });

  it("keeps the preferences when the usage cannot be read", async () => {
    signIn(teacherUser);
    serves({ usage: aiError(500, "INTERNAL", "Máy chủ gặp lỗi.") });
    renderSettings("/teacher/settings/ai");

    const card = await screen.findByRole(
      "region",
      { name: "Mức dùng AI tháng này" },
      LOAD,
    );
    expect(await within(card).findByRole("alert")).toHaveTextContent(
      "Không tải được mức dùng AI.",
    );
    expect(screen.getByRole("region", { name: "Tùy chọn" })).toBeVisible();
    expect(
      within(card).queryByRole("button", { name: "Xin thêm tín dụng" }),
    ).toBeNull();
  });

  it("says when the teacher is paused", async () => {
    signIn(teacherUser);
    serves({ usage: { ...USAGE, paused: true } });
    renderSettings("/teacher/settings/ai");

    expect(
      await screen.findByText("AI đang tạm dừng với tài khoản của bạn.", {}, LOAD),
    ).toBeVisible();
  });

  it("says whose credits ran out", async () => {
    signIn(teacherUser);
    serves({ usage: { ...USAGE, used: 20_000, left: 0 } });
    renderSettings("/teacher/settings/ai");

    expect(
      await screen.findByText("Bạn đã dùng hết hạn mức tháng này.", {}, LOAD),
    ).toBeVisible();
  });
});

describe("Ask for more credits", () => {
  function sendsRequests() {
    const bodies: unknown[] = [];
    server.use(
      http.post(`${BASE}/teacher/ai/credit-requests`, async ({ request }) => {
        const body = (await request.json()) as { amount: number; reason: string };
        bodies.push(body);
        await new Promise((resolve) => setTimeout(resolve, 30));
        return contractJson("/teacher/ai/credit-requests", "post", 201, {
          ...PENDING,
          amount: body.amount,
          reason: body.reason,
        });
      }),
    );
    return bodies;
  }

  it("is hidden while the centre's rule is to pause", async () => {
    signIn(teacherUser);
    serves({ usage: { ...USAGE, requests: false } });
    renderSettings("/teacher/settings/ai");

    expect(await screen.findByText("12.450", {}, LOAD)).toBeVisible();
    expect(screen.queryByRole("button", { name: "Xin thêm tín dụng" })).toBeNull();
  });

  it("shows the waiting request instead of the form", async () => {
    signIn(teacherUser);
    serves({ usage: { ...USAGE, pending: PENDING } });
    renderSettings("/teacher/settings/ai");

    expect(
      await screen.findByText("Đang chờ duyệt: thêm 5.000 tín dụng", {}, LOAD),
    ).toBeVisible();
    expect(
      screen.getByText("Gửi ngày 12/10/2026. Quản trị viên sẽ quyết định."),
    ).toBeVisible();
    expect(screen.getByText("Đề thi thử cuối kỳ cho ba lớp.")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Xin thêm tín dụng" })).toBeNull();
  });

  it("checks the amount and the reason before sending", async () => {
    signIn(teacherUser);
    serves();
    const bodies = sendsRequests();
    renderSettings("/teacher/settings/ai");

    await userEvent.click(
      await screen.findByRole("button", { name: "Xin thêm tín dụng" }, LOAD),
    );
    const amount = screen.getByRole("textbox", { name: "Số tín dụng" });
    expect(amount).toHaveFocus();
    await userEvent.type(amount, "500");
    await userEvent.click(screen.getByRole("button", { name: "Gửi yêu cầu" }));

    expect(
      await screen.findByText("Nhập một số nguyên từ 1.000 đến 100.000.", {}, LOAD),
    ).toBeVisible();
    expect(screen.getByText("Hãy cho biết lý do.")).toBeVisible();
    expect(amount).toHaveAttribute("aria-invalid", "true");
    expect(bodies).toEqual([]);
  });

  it("sends once on a double press, then shows the request waiting", async () => {
    signIn(teacherUser);
    const reads = serves();
    const bodies = sendsRequests();
    renderSettings("/teacher/settings/ai");

    await userEvent.click(
      await screen.findByRole("button", { name: "Xin thêm tín dụng" }, LOAD),
    );
    await userEvent.type(screen.getByRole("textbox", { name: "Số tín dụng" }), "5000");
    await userEvent.type(
      screen.getByRole("textbox", { name: "Lý do" }),
      "Đề thi thử cuối kỳ cho ba lớp.",
    );
    await userEvent.dblClick(screen.getByRole("button", { name: "Gửi yêu cầu" }));

    expect(
      await screen.findByText("Đã gửi yêu cầu. Quản trị viên sẽ được báo.", {}, LOAD),
    ).toBeVisible();
    expect(bodies).toEqual([
      { amount: 5000, reason: "Đề thi thử cuối kỳ cho ba lớp." },
    ]);
    const pending = screen.getByTestId("ai-request-pending");
    expect(pending).toHaveTextContent("Đang chờ duyệt: thêm 5.000 tín dụng");
    expect(pending).toHaveFocus();
    expect(screen.queryByRole("button", { name: "Xin thêm tín dụng" })).toBeNull();
    expect(reads.usage).toBe(1);
  });

  it("shows the refusal and rereads the usage on a 409", async () => {
    signIn(teacherUser);
    const reads = serves();
    server.use(
      http.post(`${BASE}/teacher/ai/credit-requests`, () =>
        aiError(409, "AI_REQUEST_PENDING", "Bạn đã có một yêu cầu đang chờ duyệt."),
      ),
    );
    renderSettings("/teacher/settings/ai");

    await userEvent.click(
      await screen.findByRole("button", { name: "Xin thêm tín dụng" }, LOAD),
    );
    await userEvent.type(screen.getByRole("textbox", { name: "Số tín dụng" }), "2000");
    await userEvent.type(
      screen.getByRole("textbox", { name: "Lý do" }),
      "Thêm bài luyện.",
    );
    await userEvent.click(screen.getByRole("button", { name: "Gửi yêu cầu" }));

    expect(await screen.findByRole("alert", {}, LOAD)).toHaveTextContent(
      "Bạn đã có một yêu cầu đang chờ duyệt.",
    );
    await waitFor(() => expect(reads.usage).toBe(2));
  });

  it("returns focus to its button when cancelled", async () => {
    signIn(teacherUser);
    serves();
    renderSettings("/teacher/settings/ai");

    await userEvent.click(
      await screen.findByRole("button", { name: "Xin thêm tín dụng" }, LOAD),
    );
    await userEvent.click(screen.getByRole("button", { name: "Huỷ" }));

    expect(screen.getByRole("button", { name: "Xin thêm tín dụng" })).toHaveFocus();
  });
});
