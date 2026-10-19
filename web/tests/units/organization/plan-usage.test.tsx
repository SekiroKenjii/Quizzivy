import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, within } from "@testing-library/react";
import { http } from "msw";
import { contentWidth } from "@tests/support/contentWidth";
import { server } from "@tests/support/server";
import { useAuthStore } from "@/stores/auth";
import {
  BASE,
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

async function openPlan() {
  servesSettings();
  renderSettings("/admin/settings/plan");
  return screen.findByRole("meter", { name: "Chỗ ngồi" });
}

describe("the Plan & usage section", () => {
  it("names the plan and when it renews, and draws a meter per resource", async () => {
    servesUsage();
    const seats = await openPlan();

    expect(screen.getByText("Gói Trung tâm · gia hạn 01/12/2026")).toBeVisible();
    expect(seats).toHaveAttribute("aria-valuetext", "412 / 500");
    expect(seats).toHaveAttribute("aria-valuenow", "412");
    expect(screen.getByRole("meter", { name: "Dung lượng tệp media" })).toHaveAttribute(
      "aria-valuetext",
      "2\u00a0GB / 5\u00a0GB",
    );
    expect(
      screen.getByRole("meter", { name: "Dung lượng tài liệu nhập" }),
    ).toHaveAttribute("aria-valuetext", "0.29\u00a0GB / 1\u00a0GB");
    expect(screen.getByText("14 lần nhập", { exact: false })).toBeVisible();
    expect(screen.getByRole("meter", { name: "Giáo viên" })).toHaveAttribute(
      "aria-valuetext",
      "18 / 25",
    );
  });

  it("shows only the figure of a resource that has no limit", async () => {
    servesUsage({
      ...USAGE,
      seats: { used: 412, limit: null },
      teachers: { used: 18, limit: null },
      media: { bytes: 2 * 1024 ** 3, limit: null },
      imports: { bytes: 0, count: 0, limit: null },
    });
    servesSettings();
    renderSettings("/admin/settings/plan");

    expect(await screen.findByText("Chỗ ngồi")).toBeVisible();
    expect(screen.getByText("412")).toBeVisible();
    expect(screen.getByText("2 GB")).toBeVisible();
    expect(screen.queryByRole("meter", { name: "Chỗ ngồi" })).toBeNull();
    expect(screen.queryByRole("meter", { name: "Giáo viên" })).toBeNull();
    expect(screen.queryByRole("meter", { name: "Dung lượng tệp media" })).toBeNull();
    expect(
      screen.queryByRole("meter", { name: "Dung lượng tài liệu nhập" }),
    ).toBeNull();
  });

  it("shows the AI credits row when the server reports the month's credits", async () => {
    servesUsage();
    await openPlan();

    const row = screen.getByRole("meter", { name: "Điểm AI" });
    expect(row).toHaveAttribute("aria-valuetext", "6.200 / 20.000 trong tháng này");
  });

  it("leaves the AI credits row out while the credits are null", async () => {
    servesUsage({ ...USAGE, aiCredits: null });
    await openPlan();

    expect(screen.queryByText("Điểm AI")).toBeNull();
  });

  it("says no plan is set when there is none", async () => {
    servesUsage({ ...USAGE, plan: null });
    await openPlan();

    expect(screen.getByText("Chưa cấu hình gói cho trung tâm này.")).toBeVisible();
  });

  it("says the usage could not be read, with a retry", async () => {
    servesSettings();
    server.use(
      http.get(`${BASE}/admin/usage`, () =>
        contractJson("/admin/usage", "get", 500, failure("INTERNAL", "Lỗi máy chủ")),
      ),
    );
    renderSettings("/admin/settings/plan");

    const alert = await screen.findByRole("alert");
    expect(
      within(alert.parentElement as HTMLElement).getByRole("button", {
        name: "Thử lại",
      }),
    ).toBeVisible();
    expect(alert).toHaveTextContent("Không tải được mức dùng.");
  });
});
