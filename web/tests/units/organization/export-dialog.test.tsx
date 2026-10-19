import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import type { components } from "@/lib/api/schema";
import { adminUser } from "@tests/support/fixtures";
import { assertContractRequest, contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { server } from "@tests/support/server";
import { useAuthStore } from "@/stores/auth";
import { BASE, failure, servesSettings, signInAsAdmin } from "./support";
import { renderSettings } from "./render";
import "@/lib/i18n";

type Export = components["schemas"]["DataExport"];

const QUEUED: Export = {
  id: "019535d9-3df7-79fb-b466-fa907fa17f90",
  status: "queued",
  includeAnswers: false,
  bytes: null,
  createdAt: "2026-10-19T03:00:00Z",
  completedAt: null,
  expiresAt: null,
  downloadedAt: null,
  downloads: 0,
};

beforeEach(() => {
  contentWidth(1080);
  signInAsAdmin();
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

function servesExport(answer: () => Response) {
  const sent: { includeAnswers: boolean }[] = [];
  server.use(
    http.post(`${BASE}/admin/data-exports`, async ({ request }) => {
      const body = (await request.json()) as { includeAnswers: boolean };
      assertContractRequest("/admin/data-exports", "post", body);
      sent.push(body);
      return answer();
    }),
  );
  return sent;
}

async function openDialog() {
  servesSettings();
  renderSettings("/admin/settings/data");
  const user = userEvent.setup({ delay: null });
  await user.click(await screen.findByRole("button", { name: "Yêu cầu xuất" }));
  const dialog = await screen.findByRole("dialog", { name: "Xuất toàn bộ dữ liệu?" });
  return { user, dialog };
}

const submit = (dialog: HTMLElement) =>
  within(dialog).getByRole("button", { name: "Yêu cầu xuất" });

describe("the export dialog", () => {
  it("shows the requester's own address, read-only, and answers off by default", async () => {
    const { dialog } = await openDialog();

    const address = within(dialog).getByRole("textbox", { name: "Gửi liên kết tới" });
    expect(address).toHaveValue(adminUser.email);
    expect(address).toHaveAttribute("readonly");
    const toggle = within(dialog).getByRole("switch", { name: /Bao gồm câu trả lời/ });
    expect(toggle).not.toBeChecked();
    expect(within(dialog).getByText("Tệp sẽ lớn hơn nhiều")).toBeVisible();
  });

  it("requests the export, closes and says where the link goes", async () => {
    const sent = servesExport(() =>
      contractJson("/admin/data-exports", "post", 202, QUEUED),
    );
    const { user, dialog } = await openDialog();

    await user.click(
      within(dialog).getByRole("switch", { name: /Bao gồm câu trả lời/ }),
    );
    await user.click(submit(dialog));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent).toEqual([{ includeAnswers: true }]);
    expect(
      await screen.findByText(
        `Đã yêu cầu xuất. Liên kết sẽ gửi tới ${adminUser.email}`,
      ),
    ).toBeVisible();
  });

  it("explains an export that is already under way and stays open", async () => {
    servesExport(() =>
      contractJson(
        "/admin/data-exports",
        "post",
        409,
        failure("EXPORT_IN_PROGRESS", "Đang xuất."),
      ),
    );
    const { user, dialog } = await openDialog();

    await user.click(submit(dialog));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Một yêu cầu xuất đang được xử lý. Hãy đợi email của nó trước khi yêu cầu lại.",
    );
  });

  it("explains the hourly limit with the minutes to wait", async () => {
    servesExport(() =>
      HttpResponse.json(failure("RATE_LIMITED", "Quá nhiều yêu cầu."), {
        status: 429,
        headers: { "Retry-After": "1500" },
      }),
    );
    const { user, dialog } = await openDialog();

    await user.click(submit(dialog));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Mỗi giờ chỉ yêu cầu xuất được một lần. Hãy thử lại sau 25 phút.",
    );
  });

  it("says export is not switched on when the server has no storage for it", async () => {
    servesExport(() =>
      contractJson("/admin/data-exports", "post", 501, failure("EXPORT_OFF", "Tắt.")),
    );
    const { user, dialog } = await openDialog();

    await user.click(submit(dialog));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Máy chủ này chưa bật tính năng xuất dữ liệu.",
    );
  });

  it("sends one request however fast the button is pressed, and leaves no stale error", async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    let posts = 0;
    server.use(
      http.post(`${BASE}/admin/data-exports`, async () => {
        posts += 1;
        await gate;
        return posts === 1
          ? contractJson("/admin/data-exports", "post", 202, QUEUED)
          : contractJson(
              "/admin/data-exports",
              "post",
              409,
              failure("EXPORT_IN_PROGRESS", "Đang xuất."),
            );
      }),
    );
    const { user, dialog } = await openDialog();

    await user.dblClick(submit(dialog));
    release();

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(posts).toBe(1);
    await user.click(screen.getByRole("button", { name: "Yêu cầu xuất" }));
    expect(within(await screen.findByRole("dialog")).queryByRole("alert")).toBeNull();
  });
});
