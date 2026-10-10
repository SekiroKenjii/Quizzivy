import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import { emptyDraft } from "@/features/assignments/draft";
import { AssignmentDefaultsSection } from "@/features/settings/sections/AssignmentDefaults";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { BASE, signIn } from "./support";
import "@/lib/i18n";

let bodies: unknown[] = [];

beforeEach(() => {
  bodies = [];
  signIn({ ...teacherUser, preferences: { theme: "light" } });
  server.use(
    http.patch(`${BASE}/me/preferences`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      bodies.push(body);
      return HttpResponse.json({ theme: "light", ...body });
    }),
  );
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

function renderSection() {
  render(
    <>
      <AssignmentDefaultsSection />
      <Toaster />
    </>,
  );
  return userEvent.setup();
}

describe("the Assignment defaults card", () => {
  it("starts from what a new assignment starts with when nothing is stored", () => {
    renderSection();

    expect(
      screen.getByRole("heading", { name: "Mặc định khi giao bài" }),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "45 phút" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("switch", { name: "Xáo trộn câu hỏi" })).not.toBeChecked();
    expect(screen.getByRole("switch", { name: "Hiện điểm sau khi nộp" })).toBeChecked();
    expect(screen.getByRole("switch", { name: "Chặn sao chép và dán" })).toBeChecked();
    expect(
      screen.getByRole("switch", { name: "Bắt buộc toàn màn hình" }),
    ).not.toBeChecked();
    expect(
      ["30 phút", "45 phút", "60 phút", "90 phút"].map(
        (name) => screen.getByRole("button", { name }).textContent,
      ),
    ).toEqual(["30 phút", "45 phút", "60 phút", "90 phút"]);
  });

  it("saves all five, and a new assignment then starts from them", async () => {
    const user = renderSection();

    await user.click(screen.getByRole("button", { name: "60 phút" }));
    await user.click(screen.getByRole("switch", { name: "Bắt buộc toàn màn hình" }));
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() =>
      expect(bodies).toEqual([
        {
          assignmentDefaults: {
            durationMinutes: 60,
            shuffleQuestions: false,
            showScore: true,
            blockCopyPaste: true,
            requireFullscreen: true,
          },
        },
      ]),
    );
    expect(await screen.findByText("Đã lưu cài đặt.")).toBeInTheDocument();
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();

    const stored = useAuthStore.getState().user?.preferences?.assignmentDefaults;
    const draft = emptyDraft(new Date("2026-10-10T09:00:00Z"), stored ?? {});
    expect(draft.durationMinutes).toBe(60);
    expect(draft.integrity.requireFullscreen).toBe(true);
    expect(draft.integrity.blockCopyPaste).toBe(true);
  });

  it("restores the saved values on Discard", async () => {
    signIn({
      ...teacherUser,
      preferences: {
        assignmentDefaults: { durationMinutes: 30, shuffleQuestions: true },
      },
    });
    const user = renderSection();

    await user.click(screen.getByRole("button", { name: "90 phút" }));
    await user.click(screen.getByRole("switch", { name: "Xáo trộn câu hỏi" }));
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));

    expect(screen.getByRole("button", { name: "30 phút" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("switch", { name: "Xáo trộn câu hỏi" })).toBeChecked();
    expect(bodies).toEqual([]);
  });
});
