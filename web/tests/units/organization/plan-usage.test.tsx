import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { contentWidth } from "@tests/support/contentWidth";
import { server } from "@tests/support/server";
import { useAuthStore } from "@/stores/auth";
import {
  BASE,
  SAVED,
  USAGE,
  failure,
  servesSettings,
  servesUsage,
  signInAsAdmin,
} from "./support";
import { contractJson } from "@tests/support/contractResponse";
import { renderSettings } from "./render";
import "@/lib/i18n";

beforeEach(() => {
  contentWidth(1080);
  signInAsAdmin();
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

const GIB = 1024 ** 3;
const MIB = 1024 ** 2;

const field = (name: string) => screen.getByRole("textbox", { name });
const save = () => screen.getByRole("button", { name: "Lưu thay đổi" });

async function openPlan() {
  renderSettings("/admin/settings/plan");
  return screen.findByRole("textbox", { name: "Tài khoản" });
}

describe("the Plan & usage section", () => {
  it("shows each limit empty with its default and where the default comes from", async () => {
    servesSettings();
    servesUsage();
    const seats = await openPlan();

    expect(seats).toHaveValue("");
    expect(seats).toHaveAttribute("placeholder", "Không giới hạn");
    expect(screen.getAllByText("Mặc định: không giới hạn")).toHaveLength(2);
    const storage = field("Tổng dung lượng tệp");
    expect(storage).toHaveAttribute("placeholder", "10");
    expect(storage).toHaveAccessibleDescription(
      expect.stringMatching(/Mặc định: 10\sGB · gói miễn phí Cloudflare R2/),
    );
    expect(field("Tài liệu nhập mỗi giáo viên")).toHaveAttribute("placeholder", "256");
    expect(screen.getByText("Mặc định: 256 MB · mức riêng của Quizzivy")).toBeVisible();
    expect(screen.getByText("Mặc định: 100 · gói miễn phí Resend")).toBeVisible();
    expect(screen.getByText("Mặc định: 3.000 · gói miễn phí Resend")).toBeVisible();
    expect(
      screen.getByText("Mặc định: 10 GB · ngưỡng cảnh báo cho gói Neon Launch"),
    ).toBeVisible();
    expect(screen.getByText("Mặc định: 80% · mức riêng của Quizzivy")).toBeVisible();
    expect(screen.queryByRole("button", { name: /Dùng mặc định/ })).toBeNull();
  });

  it("says what is in use beside each limit, and draws no meters", async () => {
    servesSettings();
    servesUsage();
    await openPlan();

    expect(await screen.findAllByText("Đang dùng 412")).toHaveLength(1);
    expect(screen.getByText("Đang dùng 18")).toBeVisible();
    expect(screen.getByText("Đang dùng 3 GB")).toBeVisible();
    expect(screen.getByText("Thư viện lớn nhất: 1 GB")).toBeVisible();
    expect(screen.getByText("Hôm nay đã gửi 12")).toBeVisible();
    expect(screen.getByText("Tháng này đã gửi 340")).toBeVisible();
    expect(screen.getByText("Hiện khoảng 0.59 GB")).toBeVisible();
    expect(screen.queryByRole("meter")).toBeNull();
  });

  it("saves typed limits in bytes, and Use default sends null", async () => {
    const stored = servesSettings();
    servesUsage();
    await openPlan();
    const user = userEvent.setup({ delay: null });

    await user.type(field("Tài khoản"), "500");
    await user.type(field("Tổng dung lượng tệp"), "2,5");
    await user.type(field("Tài liệu nhập mỗi giáo viên"), "300");
    await user.type(field("Email mỗi ngày"), "80");
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.limits).toEqual({
      ...SAVED.limits,
      seats: 500,
      storageBytes: 2560 * MIB,
      importPerTeacherBytes: 300 * MIB,
      emailsPerDay: 80,
    });

    expect(await screen.findByDisplayValue("2560")).toBe(field("Tổng dung lượng tệp"));
    await user.click(
      screen.getByRole("button", { name: "Dùng mặc định cho Tổng dung lượng tệp" }),
    );
    expect(field("Tổng dung lượng tệp")).toHaveValue("");
    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(2));
    expect(stored.puts[1]?.limits?.storageBytes).toBeNull();
    expect(stored.puts[1]?.limits?.seats).toBe(500);
  });

  it("reads a stored size in the unit it fits, and changes nothing until edited", async () => {
    servesSettings({
      ...SAVED,
      limits: {
        ...SAVED.limits,
        mediaPerTeacherBytes: 3 * GIB,
        importBytes: 768 * MIB,
      },
    });
    servesUsage();
    await openPlan();

    expect(field("Thư viện media mỗi giáo viên")).toHaveValue("3");
    expect(field("Tài liệu nhập, toàn trung tâm")).toHaveValue("768");
    expect(screen.getAllByRole("button", { name: /^Dùng mặc định cho/ })).toHaveLength(
      2,
    );
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
  });

  it("warns, and still saves, a limit below what is in use", async () => {
    const stored = servesSettings();
    servesUsage();
    await openPlan();
    const user = userEvent.setup({ delay: null });

    await user.type(field("Tài khoản"), "400");
    const warning = await screen.findByText(
      /^Thấp hơn mức đang dùng\. Không ai bị vô hiệu hoá/,
    );
    expect(warning).toBeVisible();
    expect(field("Tài khoản")).toHaveAccessibleDescription(
      expect.stringContaining("Thấp hơn mức đang dùng."),
    );
    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.limits?.seats).toBe(400);
  });

  it("marks a limit at what is in use as near, not below, and the warning line at its edge", async () => {
    servesSettings();
    servesUsage();
    await openPlan();
    const user = userEvent.setup({ delay: null });
    const use = await screen.findByText("Đang dùng 412");

    await user.type(field("Tài khoản"), "412");
    expect(screen.queryByText(/^Thấp hơn mức đang dùng/)).toBeNull();
    expect(use).toHaveClass("text-warning-ink");

    await user.clear(field("Tài khoản"));
    await user.type(field("Tài khoản"), "515");
    expect(use).toHaveClass("text-warning-ink");

    await user.clear(field("Tài khoản"));
    await user.type(field("Tài khoản"), "516");
    expect(use).not.toHaveClass("text-warning-ink");
  });

  it("refuses a value out of range on its field, without saving", async () => {
    const stored = servesSettings();
    servesUsage();
    await openPlan();
    const user = userEvent.setup({ delay: null });

    await user.type(field("Cảnh báo khi đạt"), "40");
    await user.type(field("Thư viện media mỗi giáo viên"), "abc");
    await user.click(save());

    expect(
      await screen.findByText("Nhập số nguyên từ 50 đến 100, hoặc để trống."),
    ).toBeVisible();
    expect(
      screen.getByText("Nhập dung lượng từ 1 MB đến 1.048.576 GB, hoặc để trống."),
    ).toBeVisible();
    expect(field("Thư viện media mỗi giáo viên")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(stored.puts).toHaveLength(0);
  });

  it("shows the server's refusal on the limit it names", async () => {
    servesSettings(SAVED, () =>
      contractJson(
        "/admin/settings",
        "put",
        400,
        failure("VALIDATION_FAILED", "Không hợp lệ.", {
          "limits.emailsPerMonth": "Phải từ 1 đến 10.000.000.",
        }),
      ),
    );
    servesUsage();
    await openPlan();
    const user = userEvent.setup({ delay: null });

    await user.type(field("Email mỗi tháng"), "9");
    await user.click(save());

    expect(await screen.findByText("Phải từ 1 đến 10.000.000.")).toBeVisible();
    expect(field("Email mỗi tháng")).toHaveFocus();
  });

  it("names the month's AI credits with a link to where they are edited, only while reported", async () => {
    servesSettings();
    servesUsage();
    await openPlan();

    expect(await screen.findByText("6.200 / 20.000 trong tháng này")).toBeVisible();
    expect(screen.getByRole("link", { name: "Quản lý tín dụng AI" })).toHaveAttribute(
      "href",
      "/admin/ai",
    );
  });

  it("leaves the AI credits out while the credits are null", async () => {
    servesSettings();
    servesUsage({ ...USAGE, aiCredits: null });
    await openPlan();

    await screen.findByText("Đang dùng 412");
    expect(screen.queryByText("Tín dụng AI")).toBeNull();
  });

  it("keeps the limits editable when the usage cannot be read", async () => {
    servesSettings();
    server.use(
      http.get(`${BASE}/admin/usage`, () =>
        contractJson("/admin/usage", "get", 500, failure("INTERNAL", "Lỗi máy chủ")),
      ),
    );
    await openPlan();

    expect(
      await screen.findByText(
        "Không tải được mức đang dùng. Vẫn có thể đổi các giới hạn.",
      ),
    ).toBeVisible();
    expect(field("Tài khoản")).toBeEnabled();
    expect(screen.queryByText(/^Đang dùng/)).toBeNull();
  });
});
