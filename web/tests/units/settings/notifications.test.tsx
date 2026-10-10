import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import { NotificationsSection } from "@/features/settings/sections/Notifications";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { BASE, REQUEST_ID, signIn } from "./support";
import "@/lib/i18n";

const STORED = [
  { event: "attempt.submitted", inApp: true, email: false },
  { event: "attempt.flagged", inApp: true, email: true },
  { event: "assignment.closing", inApp: false, email: false },
  { event: "assignment.due_soon", inApp: false, email: false },
  { event: "result.ready", inApp: true, email: false },
] as const;

let saved: unknown[] = [];

beforeEach(() => {
  saved = [];
  signIn(teacherUser);
  server.use(
    http.get(`${BASE}/me/notification-preferences`, () =>
      contractJson("/me/notification-preferences", "get", 200, [...STORED]),
    ),
    http.put(`${BASE}/me/notification-preferences`, async ({ request }) => {
      const body = (await request.json()) as unknown[];
      saved.push(body);
      return contractJson("/me/notification-preferences", "put", 200, body);
    }),
  );
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

function renderSection() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <NotificationsSection />
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

const toggle = (update: string) =>
  screen.getByRole("switch", { name: `${update} trong ứng dụng` });

describe("the Notifications card", () => {
  it("draws the teacher's three updates with an In app switch each, as stored", async () => {
    renderSection();

    expect(screen.getByRole("heading", { name: "Thông báo" })).toBeVisible();
    expect(screen.getByText("Chọn nơi mỗi cập nhật đến với bạn.")).toBeVisible();
    expect(await screen.findByText("Gộp lại mỗi 15 phút")).toBeVisible();
    expect(toggle("Học viên nộp bài")).toBeChecked();
    expect(toggle("Một lượt làm bài bị gắn cờ")).toBeChecked();
    expect(toggle("Bài giao sắp đóng")).not.toBeChecked();
    expect(screen.getAllByRole("switch")).toHaveLength(3);
    expect(screen.queryByText(/Email/)).toBeNull();
  });

  it("saves a switch turned off with the other four as they were read", async () => {
    const user = renderSection();
    await screen.findByText("Gộp lại mỗi 15 phút");

    await user.click(toggle("Một lượt làm bài bị gắn cờ"));
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() =>
      expect(saved).toEqual([
        [
          { event: "attempt.submitted", inApp: true, email: false },
          { event: "attempt.flagged", inApp: false, email: true },
          { event: "assignment.closing", inApp: false, email: false },
          { event: "assignment.due_soon", inApp: false, email: false },
          { event: "result.ready", inApp: true, email: false },
        ],
      ]),
    );
    expect(await screen.findByText("Đã lưu cài đặt.")).toBeInTheDocument();
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(toggle("Một lượt làm bài bị gắn cờ")).not.toBeChecked();
  });

  it("restores the saved switches on Discard, and a switch put back is not a change", async () => {
    const user = renderSection();
    await screen.findByText("Gộp lại mỗi 15 phút");

    await user.click(toggle("Bài giao sắp đóng"));
    await user.click(toggle("Bài giao sắp đóng"));
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    await user.click(toggle("Học viên nộp bài"));
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));

    expect(toggle("Học viên nộp bài")).toBeChecked();
    expect(saved).toEqual([]);
  });

  it("keeps the change and shows the refusal in the bar", async () => {
    server.use(
      http.put(`${BASE}/me/notification-preferences`, () =>
        HttpResponse.json(
          {
            error: {
              code: "VALIDATION_FAILED",
              message: "Cài đặt không hợp lệ.",
              requestId: REQUEST_ID,
            },
          },
          { status: 400 },
        ),
      ),
    );
    const user = renderSection();
    await screen.findByText("Gộp lại mỗi 15 phút");
    await user.click(toggle("Học viên nộp bài"));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Cài đặt không hợp lệ.");
    expect(toggle("Học viên nộp bài")).not.toBeChecked();
  });

  it("says the settings could not be read, and retries", async () => {
    let fail = true;
    server.use(
      http.get(`${BASE}/me/notification-preferences`, () =>
        fail
          ? HttpResponse.json(
              { error: { code: "INTERNAL", message: "Lỗi.", requestId: REQUEST_ID } },
              { status: 500 },
            )
          : contractJson("/me/notification-preferences", "get", 200, [...STORED]),
      ),
    );
    const user = renderSection();

    const failed = await screen.findByText("Không tải được cài đặt thông báo.");
    fail = false;
    await user.click(
      within(failed.parentElement!).getByRole("button", { name: "Thử lại" }),
    );
    expect(await screen.findByText("Gộp lại mỗi 15 phút")).toBeVisible();
  });
});
