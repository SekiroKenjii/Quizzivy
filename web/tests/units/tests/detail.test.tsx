import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { registerContentElement } from "@/layouts/shell/contentWidth";
import { server } from "@tests/support/server";
import {
  BASE,
  PUBLISHED_PROMPT,
  TEST_ID,
  detail,
  renderDetail,
  serveDetail,
  testFixture,
  versionFixture,
} from "./detailHarness";

beforeEach(serveDetail);

afterEach(() => registerContentElement(null));

function narrow() {
  registerContentElement(document.createElement("div"));
}

describe("the test detail's header", () => {
  it("goes back to Tests and names the test, its status and its facts", async () => {
    detail.test = testFixture({
      questionCount: 12,
      totalPoints: 15,
      assignments: { live: 2, scheduled: 0, closed: 1 },
    });
    renderDetail();

    expect(
      await screen.findByRole("heading", { level: 1, name: "Unit 5" }),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Đề thi" })).toHaveAttribute(
      "href",
      "/teacher/tests",
    );
    expect(screen.getByText("Đã phát hành")).toBeVisible();
    expect(screen.getByText("12 câu hỏi · 15 điểm · 2 đang mở")).toBeVisible();
  });

  it("assigns through the wizard with the test chosen", async () => {
    renderDetail();

    expect(await screen.findByRole("link", { name: "Giao bài" })).toHaveAttribute(
      "href",
      `/teacher/assignments/new?test=${TEST_ID}`,
    );
    expect(screen.getByRole("link", { name: "Mở trình soạn đề" })).toHaveAttribute(
      "href",
      `/teacher/tests/${TEST_ID}/edit`,
    );
  });

  it("cannot assign a test with no version", async () => {
    detail.test = testFixture({
      status: "draft",
      currentVersion: 0,
      unpublishedChanges: null,
    });
    detail.versions = [];
    renderDetail();

    expect(await screen.findByRole("button", { name: "Giao bài" })).toBeDisabled();
    expect(screen.queryByRole("link", { name: "Giao bài" })).toBeNull();
  });

  it("offers the history in the header only below the two columns", async () => {
    renderDetail();
    await screen.findByText(PUBLISHED_PROMPT);
    expect(screen.queryByRole("button", { name: "Lịch sử phiên bản" })).toBeNull();
    expect(
      screen.getByRole("complementary", { name: "Lịch sử phiên bản" }),
    ).toBeVisible();

    cleanup();
    narrow();
    renderDetail();
    await screen.findByText(PUBLISHED_PROMPT);
    expect(screen.getByRole("button", { name: "Lịch sử phiên bản" })).toBeVisible();
    expect(
      screen.queryByRole("complementary", { name: "Lịch sử phiên bản" }),
    ).toBeNull();
  });
});

describe("the test detail's preview", () => {
  it("renders the default version, not the draft", async () => {
    renderDetail();

    expect(await screen.findByText(PUBLISHED_PROMPT)).toBeInTheDocument();
    expect(screen.getByText("Phiên bản 1 · mặc định cho bài giao mới")).toBeVisible();
    expect(screen.getByText("1 câu hỏi. Học viên không thấy đáp án.")).toBeVisible();
    expect(detail.previewVersions).toEqual([1]);
  });

  it("does not change after the draft is edited", async () => {
    renderDetail();
    await screen.findByText(PUBLISHED_PROMPT);

    detail.test = testFixture({
      title: "Unit 5 (đang sửa)",
      questionCount: 9,
      totalPoints: 99,
    });
    cleanup();
    renderDetail();

    expect(
      await screen.findByRole("heading", { level: 1, name: "Unit 5 (đang sửa)" }),
    ).toBeVisible();
    expect(await screen.findByText(PUBLISHED_PROMPT)).toBeInTheDocument();
    expect(detail.previewVersions).toEqual([1, 1]);
  });

  it("carries no grading key into the preview", async () => {
    renderDetail();
    await screen.findByText(PUBLISHED_PROMPT);

    expect(document.body.textContent ?? "").not.toContain("isCorrect");
    expect(screen.getByText("went")).toBeInTheDocument();
    expect(screen.queryByText(/đáp án đúng/i)).toBeNull();
  });

  it("shows an older version when the address names it", async () => {
    detail.test = testFixture({ currentVersion: 2 });
    detail.versions = [versionFixture(1), versionFixture(2)];
    renderDetail(`/teacher/tests/${TEST_ID}?version=1`);

    expect(
      await screen.findByText(
        "Phiên bản 1 · phiên bản cũ. Bài giao đang dùng vẫn giữ phiên bản này.",
      ),
    ).toBeVisible();
    expect(detail.previewVersions).toEqual([1]);
  });

  it("draws the phone frame from the address and returns to the computer", async () => {
    const { user, router } = renderDetail(`/teacher/tests/${TEST_ID}?device=phone`);
    await screen.findByText(PUBLISHED_PROMPT);

    const frame = document.querySelector("[data-preview-viewport]");
    expect(frame).toHaveAttribute("data-preview-viewport", "phone");
    expect(screen.getByRole("button", { name: "Điện thoại" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    await user.click(screen.getByRole("button", { name: "Máy tính" }));
    expect(frame).toHaveAttribute("data-preview-viewport", "desktop");
    expect(router.state.location.search).toBe("");
  });

  it("says a test with no version has nothing to preview and offers the builder", async () => {
    detail.test = testFixture({
      status: "draft",
      currentVersion: 0,
      unpublishedChanges: null,
    });
    detail.versions = [];
    renderDetail();

    expect(
      await screen.findByText(
        "Đề này chưa phát hành nên học viên chưa có gì để nhận. Mở trình soạn đề để hoàn thiện và phát hành.",
      ),
    ).toBeVisible();
    expect(screen.getByText("Chưa phát hành phiên bản nào")).toBeVisible();
    expect(screen.getAllByRole("link", { name: "Mở trình soạn đề" })).toHaveLength(2);
    expect(detail.previewVersions).toEqual([]);
  });

  it("reports a preview that fails to load, with a retry", async () => {
    server.use(
      http.get(`${BASE}/teacher/tests/:id/preview`, () =>
        Response.json(
          { error: { code: "INTERNAL", message: "Lỗi máy chủ." } },
          { status: 500 },
        ),
      ),
    );
    renderDetail();

    expect(await screen.findByText("Không tải được bản xem trước.")).toBeVisible();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeVisible();
  });
});

describe("the test detail's states", () => {
  it("shows a skeleton while the test loads", () => {
    renderDetail();
    expect(document.querySelector("[aria-busy='true']")).not.toBeNull();
  });

  it("reports a test that fails to load", async () => {
    server.use(
      http.get(`${BASE}/teacher/tests/:id`, () =>
        Response.json(
          { error: { code: "INTERNAL", message: "Lỗi máy chủ." } },
          { status: 500 },
        ),
      ),
    );
    renderDetail();

    expect(await screen.findByText("Không tải được đề thi.")).toBeVisible();
  });

  it("opens the history sheet from the builder's #versions link below the two columns", async () => {
    narrow();
    renderDetail(`/teacher/tests/${TEST_ID}#versions`);

    const sheet = await screen.findByRole("dialog", { name: "Lịch sử phiên bản" });
    expect(await within(sheet).findByText("Phiên bản 1")).toBeVisible();
  });

  it("reaches the header's controls by keyboard, back link first", async () => {
    const { user } = renderDetail();
    await screen.findByText(PUBLISHED_PROMPT);

    await user.tab();
    expect(document.activeElement).toHaveTextContent("Đề thi");
    await user.tab();
    expect(document.activeElement).toHaveTextContent("Mở trình soạn đề");
    await user.tab();
    expect(document.activeElement).toHaveTextContent("Giao bài");
    await waitFor(() =>
      expect(document.activeElement).toBeInstanceOf(HTMLAnchorElement),
    );
  });
});
