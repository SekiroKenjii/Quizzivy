import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { emptyDraft, draftOf } from "@/features/assignments/draft";
import { leavingMode, leavingValue } from "@/features/assignments/wizardValues";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BASE,
  CLASS_A,
  CREATED,
  TEST_ID,
  VERSIONS,
  installWizardServer,
  renderWizard,
  requests,
  stored,
} from "@tests/support/assignmentWizard";
import i18n from "@/lib/i18n";

beforeEach(installWizardServer);
afterEach(() => useAuthStore.setState({ user: null }));

function stepper(name: string) {
  return screen.getByRole("spinbutton", { name });
}

async function type(
  user: ReturnType<typeof userEvent.setup>,
  name: string,
  value: string,
) {
  const field = stepper(name);
  await user.click(field);
  await user.keyboard(`{Control>}a{/Control}${value}{Enter}`);
}

function reads() {
  return within(screen.getByRole("complementary", { name: "Tóm tắt bài giao" }));
}

describe("the leaving limit", () => {
  it("maps the deck's three choices onto maxFocusLoss and back", () => {
    expect(leavingMode(0)).toBe("unlimited");
    expect(leavingMode(-1)).toBe("none");
    expect(leavingMode(3)).toBe("limit");
    expect(leavingValue("unlimited", 3)).toBe(0);
    expect(leavingValue("none", 3)).toBe(-1);
    expect(leavingValue("limit", 3)).toBe(3);
    expect(leavingValue("limit", 0), "a new limit starts at 2").toBe(2);
    expect(leavingValue("limit", -1)).toBe(2);
  });
});

describe("the wizard's defaults", () => {
  it("start from the teacher's stored defaults and fall back to the deck's", () => {
    const now = new Date("2026-09-01T03:20:00Z");
    expect(emptyDraft(now)).toMatchObject({
      durationMinutes: 45,
      shuffleQuestions: false,
      review: { showScore: true, release: "on_submit", showClassAverage: false },
      integrity: {
        requireFullscreen: false,
        blockCopyPaste: true,
        maxFocusLoss: 0,
        onLimitExceeded: "flag",
        minAwayMs: 3000,
      },
      studentNote: "",
    });
    expect(
      emptyDraft(now, {
        durationMinutes: 30,
        shuffleQuestions: true,
        showScore: false,
        blockCopyPaste: false,
        requireFullscreen: true,
      }),
    ).toMatchObject({
      durationMinutes: 30,
      shuffleQuestions: true,
      review: { showScore: false },
      integrity: { requireFullscreen: true, blockCopyPaste: false, maxFocusLoss: 0 },
    });
    expect(emptyDraft(now, { durationMinutes: 90 })).toMatchObject({
      durationMinutes: 90,
      shuffleQuestions: false,
      review: { showScore: true },
      integrity: { requireFullscreen: false, blockCopyPaste: true },
    });
  });

  it("are what a new assignment opens with, while a stored one keeps its policy", async () => {
    useAuthStore.setState({
      user: {
        ...teacherUser,
        preferences: {
          assignmentDefaults: { durationMinutes: 30, requireFullscreen: true },
        },
      },
    });
    const user = userEvent.setup();
    renderWizard("/teacher/assignments/new?step=4");
    expect(
      screen.getByRole("switch", { name: "Bắt buộc toàn màn hình" }),
    ).toBeChecked();
    expect(screen.getByRole("switch", { name: "Chặn sao chép và dán" })).toBeChecked();
    await user.click(screen.getByRole("button", { name: /^Lịch/ }));
    expect(screen.getByRole("button", { name: "30 phút" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    const kept = stored({
      testVersionId: VERSIONS[TEST_ID]!.id,
      targets: { classIds: [CLASS_A], studentIds: [] },
      window: { opensAt: "2026-09-01T01:00:00Z", closesAt: "2026-09-03T14:00:00Z" },
      durationMinutes: 75,
      maxAttempts: 1,
      shuffleQuestions: false,
      shuffleOptions: false,
      review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
      integrity: {
        requireFullscreen: false,
        blockCopyPaste: true,
        maxFocusLoss: 0,
        onLimitExceeded: "flag",
        minAwayMs: 3000,
      },
      draft: true,
    });
    expect(draftOf(kept, [VERSIONS[TEST_ID]!])).toMatchObject({
      durationMinutes: 75,
      integrity: { requireFullscreen: false },
    });
  });
});

describe("the Rules step", () => {
  it("waits for correct answers before explanations, and turns them off with them", async () => {
    const user = userEvent.setup();
    renderWizard("/teacher/assignments/new?step=4");
    const explanations = screen.getByRole("switch", { name: "Giải thích" });
    expect(explanations).toBeDisabled();
    expect(explanations).toHaveAccessibleDescription("Bật đáp án đúng trước");
    await user.click(screen.getByRole("switch", { name: "Đáp án đúng" }));
    expect(explanations).toBeEnabled();
    expect(explanations).toHaveAccessibleDescription(
      "Lời giải thích viết cho từng câu",
    );
    await user.click(explanations);
    expect(explanations).toBeChecked();
    await user.click(screen.getByRole("switch", { name: "Đáp án đúng" }));
    expect(explanations).not.toBeChecked();
    expect(explanations).toBeDisabled();
  });

  it("dims the action under no limit and maps each leaving choice", async () => {
    const user = userEvent.setup();
    renderWizard("/teacher/assignments/new?step=4");
    const action = screen.getByRole("group", { name: "Khi vượt giới hạn" });
    for (const option of within(action).getAllByRole("button"))
      expect(option).toBeDisabled();
    expect(
      screen.getByText("Hãy chọn giới hạn trước. Mỗi lần rời bài vẫn được ghi lại."),
    ).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Đặt giới hạn" }));
    expect(stepper("lần được phép")).toHaveAttribute("aria-valuenow", "2");
    for (const option of within(action).getAllByRole("button"))
      expect(option).toBeEnabled();
    expect(screen.getByText("Đánh dấu bài làm để bạn xem lại.")).toBeVisible();
    expect(screen.getByRole("button", { name: /^Quy định/ })).toHaveTextContent(
      "Được rời bài 2 lần",
    );

    await user.click(screen.getByRole("button", { name: "Không được phép" }));
    expect(screen.queryByRole("spinbutton", { name: "lần được phép" })).toBeNull();
    await user.click(within(action).getByRole("button", { name: "Tự nộp bài" }));
    expect(
      reads().getByText(
        "Không được rời trang làm bài. Nếu bạn rời trang, bài sẽ được nộp ngay.",
      ),
    ).toBeVisible();
  });

  it("tells the teacher what students will read for each choice", async () => {
    const user = userEvent.setup();
    renderWizard("/teacher/assignments/new?step=4");
    expect(reads().getByText("Bạn sẽ xem được điểm sau khi nộp bài.")).toBeVisible();
    expect(reads().queryByText("Bài chạy ở chế độ toàn màn hình.")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Sau khi bài đóng" }));
    expect(reads().getByText("Bạn sẽ xem được điểm sau khi bài đóng.")).toBeVisible();
    await user.click(screen.getByRole("switch", { name: "Bắt buộc toàn màn hình" }));
    expect(reads().getByText("Bài chạy ở chế độ toàn màn hình.")).toBeVisible();
    await user.click(screen.getByRole("switch", { name: "Điểm của mình" }));
    expect(reads().queryByText(/Bạn sẽ xem được điểm/)).toBeNull();
  });

  it("keeps the honest limits beside the integrity rules", () => {
    renderWizard("/teacher/assignments/new?step=4");
    expect(
      screen.getByText(
        "Trình duyệt không thể ngăn gian lận. Các quy định này ghi lại điều đã xảy ra để bạn tự quyết định.",
      ),
    ).toBeVisible();
  });
});

describe("assigning", () => {
  it("sends the schedule, the rules and the note, and opens the right tab", async () => {
    const user = userEvent.setup();
    const router = renderWizard(
      `/teacher/assignments/new?test=${TEST_ID}&class=${CLASS_A}&step=3`,
    );
    await waitFor(
      () =>
        expect(screen.getByRole("button", { name: /^Học viên/ })).toHaveTextContent(
          "IELTS Foundation",
        ),
      { timeout: 5000 },
    );
    await user.click(screen.getByRole("button", { name: "Tuỳ chỉnh" }));
    await type(user, "Thời gian làm bài", "65");
    await type(user, "Số lượt mỗi học viên", "2");
    expect(
      screen.getByText("Lấy điểm cao nhất. Mỗi lượt có thời gian làm bài riêng."),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: /^Quy định/ }));
    await user.click(screen.getByRole("button", { name: "Không được phép" }));
    await user.click(screen.getByRole("button", { name: "Tự nộp bài" }));
    await type(user, "Bỏ qua lần rời ngắn", "5");
    await user.type(
      screen.getByRole("textbox", { name: "Lời nhắn cho học viên (không bắt buộc)" }),
      "Mang tai nghe.",
    );
    expect(screen.getByText("14/500")).toBeVisible();
    expect(
      reads().getByText("Khoảng 4 câu trả lời cần chấm tay (2 câu × 2 học viên)."),
    ).toBeVisible();
    await user.click(
      await screen.findByRole("button", { name: "Giao cho 2 học viên" }),
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/assignments"),
    );
    expect(router.state.location.search).toBe("?status=open");
    expect(requests.posted).toHaveLength(1);
    expect(requests.posted[0]).toMatchObject({
      draft: false,
      targets: { classIds: [CLASS_A], studentIds: [] },
      durationMinutes: 65,
      maxAttempts: 2,
      integrity: { maxFocusLoss: -1, onLimitExceeded: "auto_submit", minAwayMs: 5000 },
      studentNote: "Mang tai nghe.",
    });
  });

  it("sends the teacher back to what is missing", async () => {
    const user = userEvent.setup();
    const router = renderWizard(`/teacher/assignments/new?test=${TEST_ID}&step=4`);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /^Đề thi/ })).toHaveTextContent(
        "Unit 5 Reading",
      ),
    );
    await user.click(screen.getByRole("button", { name: "Giao bài" }));
    expect(router.state.location.search).toBe(`?test=${TEST_ID}&step=2`);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Chọn ít nhất một lớp hoặc một học viên.",
    );
    expect(requests.posted).toEqual([]);
  });
});

describe("editing in the wizard", () => {
  function serve(publishedAt: string | null) {
    const saved = {
      ...stored({
        testVersionId: VERSIONS[TEST_ID]!.id,
        targets: { classIds: [CLASS_A], studentIds: [] },
        window: { opensAt: "2099-09-01T01:00:00Z", closesAt: "2099-09-03T14:00:00Z" },
        durationMinutes: 90,
        maxAttempts: 2,
        shuffleQuestions: false,
        shuffleOptions: true,
        review: {
          showScore: true,
          showCorrectAnswers: true,
          showExplanations: false,
          release: "after_close",
          showClassAverage: true,
        },
        integrity: {
          requireFullscreen: false,
          blockCopyPaste: true,
          maxFocusLoss: 3,
          onLimitExceeded: "warn",
          minAwayMs: 3000,
        },
        studentNote: "Đọc kỹ đề.",
        draft: publishedAt === null,
      }),
      publishedAt,
    };
    server.use(
      http.get(`${BASE}/teacher/assignments/${CREATED}`, () =>
        contractJson("/teacher/assignments/{id}", "get", 200, saved),
      ),
    );
  }

  it("opens a published assignment with its stored policy and saves it in place", async () => {
    serve("2026-09-01T00:00:00Z");
    const user = userEvent.setup();
    const router = renderWizard(`/teacher/assignments/${CREATED}/edit?step=4`);
    expect(
      await screen.findByRole("heading", { level: 1, name: "Chỉnh sửa bài giao" }),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Sau khi bài đóng" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      screen.getByRole("switch", { name: "Điểm trung bình của lớp" }),
    ).toBeChecked();
    expect(stepper("lần được phép")).toHaveAttribute("aria-valuenow", "3");
    expect(
      screen.getByRole("textbox", { name: "Lời nhắn cho học viên (không bắt buộc)" }),
    ).toHaveValue("Đọc kỹ đề.");
    expect(screen.queryByRole("button", { name: "Lưu nháp" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/teacher/assignments/${CREATED}`),
    );
    expect(requests.patched).toHaveLength(1);
    expect(requests.patched[0]).toMatchObject({
      draft: false,
      durationMinutes: 90,
      review: { release: "after_close", showClassAverage: true },
      integrity: { maxFocusLoss: 3, onLimitExceeded: "warn" },
      studentNote: "Đọc kỹ đề.",
    });
  });

  it("keeps a draft a draft when it is saved again", async () => {
    serve(null);
    const user = userEvent.setup();
    const router = renderWizard(`/teacher/assignments/${CREATED}/edit`);
    await user.click(await screen.findByRole("button", { name: "Lưu nháp" }));
    await waitFor(() => expect(router.state.location.search).toBe("?status=draft"));
    expect(requests.patched[0]).toMatchObject({ draft: true });
  });

  it("announces a window that closes before it opens once, beside the field", async () => {
    server.use(
      http.get(`${BASE}/teacher/assignments/${CREATED}`, () =>
        contractJson("/teacher/assignments/{id}", "get", 200, {
          ...stored({
            testVersionId: VERSIONS[TEST_ID]!.id,
            targets: { classIds: [CLASS_A], studentIds: [] },
            window: {
              opensAt: "2099-09-03T14:00:00Z",
              closesAt: "2099-09-01T01:00:00Z",
            },
            durationMinutes: 60,
            maxAttempts: 1,
            shuffleQuestions: false,
            shuffleOptions: false,
            review: {
              showScore: true,
              showCorrectAnswers: false,
              showExplanations: false,
              release: "on_submit",
              showClassAverage: false,
            },
            integrity: {
              requireFullscreen: false,
              blockCopyPaste: false,
              maxFocusLoss: 0,
              onLimitExceeded: "flag",
              minAwayMs: 0,
            },
            studentNote: null,
            draft: true,
          }),
          publishedAt: null,
        }),
      ),
    );
    const user = userEvent.setup();
    const router = renderWizard(`/teacher/assignments/${CREATED}/edit?step=4`);
    await user.click(await screen.findByRole("button", { name: /^Giao/ }));

    await waitFor(() => expect(router.state.location.search).toBe("?step=3"));
    expect(
      screen
        .getAllByRole("alert")
        .filter((alert) => alert.textContent === "Giờ đóng phải sau giờ mở."),
    ).toHaveLength(1);
    expect(requests.patched).toEqual([]);
  });
});

describe("the grading estimate", () => {
  it("counts one answer in the singular", () => {
    const estimate = (count: number) =>
      i18n.t("assignments.wizard.aside.estimate", {
        lng: "en",
        count,
        questions: "1 question",
        students: "1 student",
      });
    expect(estimate(1)).toBe(
      "About 1 answer to grade by hand (1 question × 1 student).",
    );
    expect(estimate(4)).toBe(
      "About 4 answers to grade by hand (1 question × 1 student).",
    );
  });
});
