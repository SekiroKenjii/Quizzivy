import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import i18n from "@/lib/i18n";
import { registerContentElement } from "@/layouts/shell/contentWidth";
import {
  ADDED_PROMPT,
  FIRST_QUESTION,
  PUBLISHED_PROMPT,
  SECOND_QUESTION,
  TEST_ID,
  detail,
  renderDetail,
  serveDetail,
  testFixture,
  versionFixture,
} from "./detailHarness";

beforeEach(serveDetail);

afterEach(() => registerContentElement(null));

function history() {
  return screen.getByRole("complementary", { name: "Lịch sử phiên bản" });
}

function card(n: number) {
  const found = history().querySelector<HTMLElement>(`[data-version="${n}"]`);
  if (found === null) throw new Error(`no card for version ${n}`);
  return found;
}

function twoVersions() {
  detail.test = testFixture({ currentVersion: 2, questionCount: 2, totalPoints: 3 });
  detail.versions = [versionFixture(1), versionFixture(2, { assignmentCount: 1 })];
  detail.changes = [
    {
      kind: "added",
      questionNumber: 2,
      questionId: SECOND_QUESTION,
      params: { prompt: ADDED_PROMPT },
    },
    {
      kind: "answer",
      questionNumber: 1,
      questionId: FIRST_QUESTION,
      params: { answerFrom: ["B"], answerTo: ["A"] },
    },
    { kind: "points", params: { pointsFrom: 2, pointsTo: 3 } },
  ];
}

describe("the version history", () => {
  it("draws a card per version with its use, its stats and who published it", async () => {
    twoVersions();
    renderDetail();
    await screen.findByText(PUBLISHED_PROMPT, {}, { timeout: 5000 });

    const latest = within(await waitFor(() => card(2)));
    expect(latest.getByText("Phiên bản 2")).toBeVisible();
    expect(latest.getByText("Mặc định")).toBeVisible();
    expect(latest.getByText("2 câu · 3 điểm")).toBeVisible();
    expect(latest.getByText(/Cô Thương/)).toBeVisible();
    expect(latest.getByText("Đang dùng · 1 bài giao")).toBeVisible();
    expect(latest.queryByRole("button", { name: "Đặt làm mặc định" })).toBeNull();

    const first = within(card(1));
    expect(first.getByText("Chưa dùng")).toBeVisible();
    expect(first.getByText("Phiên bản đầu tiên")).toBeVisible();
  });

  it("summarises a version without a note from its diff against the one before", async () => {
    twoVersions();
    renderDetail();

    expect(
      await within(await waitFor(() => card(2))).findByText(
        "Thêm 1 câu · Đổi 1 đáp án",
      ),
    ).toBeVisible();
    expect(detail.diffs).toEqual(["2:1"]);
  });

  it("shows the change note the teacher left instead of a summary", async () => {
    detail.test = testFixture({ currentVersion: 2 });
    detail.versions = [
      versionFixture(1),
      versionFixture(2, { changeNote: "Sửa câu 4" }),
    ];
    renderDetail();

    expect(
      await within(await waitFor(() => card(2))).findByText("Sửa câu 4"),
    ).toBeVisible();
    expect(detail.diffs).toEqual([]);
  });

  it("keeps Delete off for a version in use, saying why", async () => {
    detail.test = testFixture({ currentVersion: 1 });
    detail.versions = [versionFixture(1), versionFixture(2, { assignmentCount: 3 })];
    const { user } = renderDetail();

    const remove = await within(await waitFor(() => card(2))).findByRole("button", {
      name: "Xoá",
    });
    expect(remove).toHaveAttribute("aria-disabled", "true");
    expect(remove).toHaveAccessibleDescription(
      "Đang được bài giao dùng nên không xoá được",
    );
    await user.click(remove);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("deletes an unused version after confirming", async () => {
    detail.test = testFixture({ currentVersion: 2 });
    detail.versions = [versionFixture(1), versionFixture(2)];
    const { user } = renderDetail();

    await user.click(
      await within(await waitFor(() => card(1))).findByRole("button", { name: "Xoá" }),
    );
    const dialog = await screen.findByRole("dialog", { name: "Xoá phiên bản 1?" });
    expect(
      within(dialog).getByText(
        "Phiên bản sẽ bị xoá vĩnh viễn. Không bài giao nào dùng phiên bản này, và các phiên bản khác không bị ảnh hưởng.",
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Xoá phiên bản" }));

    await waitFor(() => expect(detail.deleted).toEqual([1]));
    await waitFor(() =>
      expect(history().querySelector('[data-version="1"]')).toBeNull(),
    );
  });

  it("requests no preview of the version it just deleted", async () => {
    detail.test = testFixture({ currentVersion: 2 });
    detail.versions = [versionFixture(1), versionFixture(2), versionFixture(3)];
    const { user, router } = renderDetail(`/teacher/tests/${TEST_ID}?version=3`);
    await screen.findByText(ADDED_PROMPT);
    expect(detail.previewVersions).toEqual([3]);

    await user.click(within(card(3)).getByRole("button", { name: "Xoá" }));
    const dialog = await screen.findByRole("dialog", { name: "Xoá phiên bản 3?" });
    await user.click(within(dialog).getByRole("button", { name: "Xoá phiên bản" }));

    await waitFor(() =>
      expect(history().querySelector('[data-version="3"]')).toBeNull(),
    );
    await waitFor(() => expect(detail.previewVersions).toContain(2));
    expect(router.state.location.search).toBe("");
    expect(detail.previewVersions.filter((version) => version === 3)).toHaveLength(1);
  });

  it("makes an older version the default, guarded by the test's updatedAt", async () => {
    detail.test = testFixture({ currentVersion: 2 });
    detail.versions = [versionFixture(1), versionFixture(2)];
    const { user } = renderDetail();

    await user.click(
      await within(await waitFor(() => card(1))).findByRole("button", {
        name: "Đặt làm mặc định",
      }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Đặt phiên bản 1 làm mặc định?",
    });
    await user.click(within(dialog).getByRole("button", { name: "Đặt làm mặc định" }));

    await waitFor(() =>
      expect(detail.current).toEqual([
        { version: 1, body: { expectedUpdatedAt: "2026-01-02T00:00:00Z" } },
      ]),
    );
    await waitFor(() => expect(within(card(1)).getByText("Mặc định")).toBeVisible());
  });

  it("restores a version as the draft, opens the builder and says so there", async () => {
    detail.test = testFixture({ unpublishedChanges: 2 });
    const { user, router } = renderDetail();

    await user.click(
      await within(await waitFor(() => card(1))).findByRole("button", {
        name: "Khôi phục thành bản nháp",
      }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Khôi phục phiên bản 1 thành bản nháp?",
    });
    expect(within(dialog).getByText(/thay bằng bản sao của phiên bản 1/)).toBeVisible();
    await user.click(
      within(dialog).getByRole("button", { name: "Khôi phục thành bản nháp" }),
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/teacher/tests/${TEST_ID}/edit`),
    );
    expect(detail.drafted).toEqual([1]);
    expect(screen.getByText("builder")).toBeVisible();
    expect(
      await screen.findByText("Đã khôi phục bản nháp từ phiên bản 1"),
    ).toBeVisible();
  });

  it("locks an archived test's versions and says why", async () => {
    detail.test = testFixture({
      status: "archived",
      currentVersion: 2,
      unpublishedChanges: 1,
    });
    detail.versions = [versionFixture(1), versionFixture(2)];
    const { user } = renderDetail();

    expect(
      await screen.findByText(
        "Đề này đã lưu trữ. Khôi phục đề trước khi đổi phiên bản mặc định hoặc tạo bản nháp từ một phiên bản.",
      ),
    ).toBeVisible();
    const older = within(await waitFor(() => card(1)));
    const restore = older.getByRole("button", { name: "Khôi phục thành bản nháp" });
    expect(restore).toHaveAttribute("aria-disabled", "true");
    expect(restore).toHaveAccessibleDescription("Hãy khôi phục đề trước");
    expect(older.getByRole("button", { name: "Đặt làm mặc định" })).toBeDisabled();
    await user.click(restore);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByText("Bản nháp có thay đổi chưa phát hành.")).toBeNull();
  });

  it("selects a version for the preview from its card", async () => {
    detail.test = testFixture({ currentVersion: 2 });
    detail.versions = [versionFixture(1), versionFixture(2)];
    const { user, router } = renderDetail();
    await screen.findByText(ADDED_PROMPT);

    await user.click(within(card(1)).getByRole("button", { name: /Phiên bản 1/ }));

    await waitFor(() => expect(screen.queryByText(ADDED_PROMPT)).toBeNull());
    expect(router.state.location.search).toBe("?version=1");
    expect(
      within(card(1)).getByRole("button", { name: /Phiên bản 1/ }),
    ).toHaveAttribute("aria-pressed", "true");
  });

  it("selects a version from the sheet below the two columns and closes it", async () => {
    registerContentElement(document.createElement("div"));
    detail.test = testFixture({ currentVersion: 2 });
    detail.versions = [versionFixture(1), versionFixture(2)];
    const { user, router } = renderDetail();
    await screen.findByText(ADDED_PROMPT);

    await user.click(screen.getByRole("button", { name: "Lịch sử phiên bản" }));
    const sheet = await screen.findByRole("dialog", { name: "Lịch sử phiên bản" });
    expect(router.state.location.search).toBe("?history=1");
    await user.click(await within(sheet).findByRole("button", { name: /Phiên bản 1/ }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(router.state.location.search).toBe("?version=1");
  });

  it("shows the draft row while the draft has unpublished changes", async () => {
    detail.test = testFixture({ unpublishedChanges: 3 });
    renderDetail();

    expect(await screen.findByText(/^Có thay đổi chưa phát hành · sửa /)).toBeVisible();
    expect(within(history()).getByRole("link", { name: "Sửa" })).toHaveAttribute(
      "href",
      `/teacher/tests/${TEST_ID}/edit`,
    );
  });

  it("explains publishing when there is no version", async () => {
    detail.test = testFixture({
      status: "draft",
      currentVersion: 0,
      unpublishedChanges: null,
    });
    detail.versions = [];
    renderDetail();

    expect(
      await screen.findByText(
        "Chưa phát hành. Phát hành sẽ chốt một phiên bản để học viên nhận. Những lần sửa sau sẽ thành phiên bản mới.",
      ),
    ).toBeVisible();
    expect(screen.getByText(/^Chưa phát hành · sửa /)).toBeVisible();
  });
});

describe("changes from the previous version", () => {
  it("lists the changes and marks the questions they touch", async () => {
    twoVersions();
    const { user, router } = renderDetail();
    await screen.findByText(ADDED_PROMPT);

    const toggle = screen.getByRole("button", { name: "Thay đổi so với phiên bản 1" });
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    await user.click(toggle);

    expect(router.state.location.search).toBe("?compare=1");
    const changes = await screen.findByRole("region", {
      name: "Thay đổi so với phiên bản 1",
    });
    expect(await within(changes).findByText(`Câu 2 · “${ADDED_PROMPT}”`)).toBeVisible();
    expect(
      within(changes).getByText("Câu 1 · đáp án đúng đổi từ B sang A"),
    ).toBeVisible();
    expect(within(changes).getByText("Tổng điểm đổi từ 2 thành 3")).toBeVisible();
    expect(
      within(changes)
        .getAllByRole("listitem")
        .map((row) => row.firstElementChild?.textContent),
    ).toEqual(["Thêm", "Đáp án", "Điểm"]);

    expect(screen.getByText("Mới")).toBeVisible();
    expect(screen.getByText("Đã sửa")).toBeVisible();
    expect(document.querySelectorAll("[data-preview-mark]")).toHaveLength(2);
    expect(detail.diffs).toEqual(["2:1"]);
  });

  it("is not offered for the first version", async () => {
    renderDetail(`/teacher/tests/${TEST_ID}?compare=1`);
    await screen.findByText(PUBLISHED_PROMPT);

    expect(screen.queryByRole("button", { name: /Thay đổi so với/ })).toBeNull();
    expect(document.querySelectorAll("[data-preview-mark]")).toHaveLength(0);
  });

  it("says when nothing changed", async () => {
    twoVersions();
    detail.changes = [];
    renderDetail(`/teacher/tests/${TEST_ID}?compare=1`);

    expect(
      await screen.findByText("Không có thay đổi nào so với phiên bản 1."),
    ).toBeVisible();
  });
});

describe("publishing the draft from the detail", () => {
  it("names the versions in the banner and publishes with a change note", async () => {
    twoVersions();
    detail.test = testFixture({ currentVersion: 2, unpublishedChanges: 4 });
    const { user } = renderDetail();

    expect(
      await screen.findByText(
        "Bài giao mới vẫn nhận phiên bản 2 cho tới khi bạn phát hành bản nháp thành phiên bản 3.",
        { exact: false },
      ),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Xem bản nháp" })).toHaveAttribute(
      "href",
      `/teacher/tests/${TEST_ID}/edit`,
    );
    await user.click(screen.getByRole("button", { name: "Phát hành phiên bản 3" }));

    const dialog = await screen.findByRole("dialog", {
      name: "Phát hành bản nháp thành phiên bản 3?",
    });
    expect(
      within(dialog).getByText(
        "Phiên bản mới sẽ là mặc định cho bài giao mới. 1 bài giao đang dùng phiên bản 2 vẫn giữ phiên bản đó.",
      ),
    ).toBeVisible();
    await user.type(
      within(dialog).getByRole("textbox", { name: /Ghi chú thay đổi/ }),
      "  Sửa đáp án câu 1  ",
    );
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(detail.publishBodies).toEqual([{ changeNote: "Sửa đáp án câu 1" }]);
  });

  it("names the number the server will assign, not the highest listed version + 1", async () => {
    twoVersions();
    detail.test = testFixture({
      currentVersion: 2,
      nextVersion: 4,
      unpublishedChanges: 4,
    });
    const { user } = renderDetail();

    expect(
      await screen.findByText(
        "Bài giao mới vẫn nhận phiên bản 2 cho tới khi bạn phát hành bản nháp thành phiên bản 4.",
        { exact: false },
      ),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Phát hành phiên bản 4" }));
    expect(
      await screen.findByRole("dialog", {
        name: "Phát hành bản nháp thành phiên bản 4?",
      }),
    ).toBeVisible();
  });

  it("says the same in English", async () => {
    await i18n.changeLanguage("en");
    try {
      twoVersions();
      detail.test = testFixture({
        currentVersion: 2,
        nextVersion: 4,
        unpublishedChanges: 4,
      });
      const { user } = renderDetail();

      expect(
        await screen.findByText(
          "New assignments get version 2 until you publish the draft as version 4.",
          { exact: false },
        ),
      ).toBeVisible();
      await user.click(screen.getByRole("button", { name: "Publish version 4" }));
      expect(
        await screen.findByRole("dialog", { name: "Publish the draft as version 4?" }),
      ).toBeVisible();
    } finally {
      await i18n.changeLanguage("vi");
    }
  });

  it("sends a null change note when the note is blank", async () => {
    detail.test = testFixture({ unpublishedChanges: 1 });
    const { user } = renderDetail();

    await user.click(
      await screen.findByRole("button", { name: "Phát hành phiên bản 2" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(
        "Phiên bản mới sẽ là mặc định cho bài giao mới. Các bài giao hiện có vẫn giữ phiên bản của mình.",
      ),
    ).toBeVisible();
    await user.type(
      within(dialog).getByRole("textbox", { name: /Ghi chú thay đổi/ }),
      "   ",
    );
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));

    await waitFor(() => expect(detail.publishBodies).toEqual([{ changeNote: null }]));
  });

  it("says how many problems block publishing and keeps the dialog open", async () => {
    detail.test = testFixture({ unpublishedChanges: 1 });
    detail.publishFailure = Response.json(
      {
        error: { code: "PUBLISH_VALIDATION_FAILED", message: "Cannot publish" },
        violations: [
          { rule: "section_not_empty", message: "Empty section", sectionId: "s1" },
          {
            rule: "choice_has_correct_option",
            message: "No correct option",
            questionId: "q1",
          },
        ],
      },
      { status: 409 },
    );
    const { user } = renderDetail();

    await user.click(
      await screen.findByRole("button", { name: "Phát hành phiên bản 2" }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));

    expect(
      await within(dialog).findByText(
        "Bản nháp còn 2 lỗi cần sửa trước khi phát hành. Mở trình soạn đề để xem.",
      ),
    ).toBeVisible();
  });

  it("shows no banner when the draft matches the latest version", async () => {
    renderDetail();
    await screen.findByText(PUBLISHED_PROMPT);

    expect(screen.queryByText("Bản nháp có thay đổi chưa phát hành.")).toBeNull();
  });
});
