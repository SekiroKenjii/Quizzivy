import { expect, test, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";
import type { AttemptResult } from "../../src/features/results/api";
import type { components } from "../../src/lib/api/schema";

type Assignment = components["schemas"]["StudentAssignmentCard"];
const classId = "018f0000-0000-7000-8000-0000000000c1";
const classes = [
  {
    id: classId,
    name: "Lớp luyện tập buổi tối",
    description: "Thứ ba và thứ năm",
    teacherName: "Cô Thương",
    joinedAt: "2026-01-01T00:00:00Z",
  },
];
function assignment(index: number, live: boolean): Assignment {
  return {
    id: `assignment-${index}`,
    testTitle: `Bài luyện tập ${index}`,
    classId,
    className: classes[0]!.name,
    status: "open",
    opensAt: new Date(Date.now() - 3600000).toISOString(),
    closesAt: new Date(Date.now() + 86400000 * 15).toISOString(),
    durationMinutes: 45,
    questionCount: 24,
    totalPoints: 30,
    attemptsUsed: live ? 1 : 0,
    maxAttempts: 2,
    hasLiveAttempt: live,
    liveDeadlineAt: live ? new Date(Date.now() + index * 600000).toISOString() : null,
  };
}
async function student(page: Page) {
  await stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/classes": { body: { items: classes } },
    "GET /app/assignments": {
      body: {
        dueNow: [assignment(1, true), assignment(2, true), assignment(3, false)],
        upcoming: [],
        completed: [],
      },
    },
  });
}
async function fits(page: Page) {
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
}

for (const width of [320, 360, 1024, 1440, 1920]) {
  test(`student discovery uses available space at ${width}px`, async ({
    page,
  }, info) => {
    await page.setViewportSize({ width, height: 900 });
    await student(page);
    await page.goto("/app");
    await expect(page.getByRole("button", { name: "Tiếp tục làm bài" })).toHaveCount(2);
    await expect(page.getByRole("link", { name: "Xem chi tiết" })).toHaveCount(1);
    await fits(page);
    const main = await page.getByRole("main").boundingBox();
    expect(main!.width).toBeGreaterThan(width * 0.95);
    await expect(page.getByRole("complementary")).toHaveCount(0);
    if (width < 1024) {
      for (const control of await page
        .getByRole("main")
        .locator("button, [data-slot='button']")
        .all()) {
        const box = await control.boundingBox();
        expect(box!.height).toBeGreaterThanOrEqual(44);
        expect(box!.width).toBeGreaterThanOrEqual(44);
      }
    }
    await page.screenshot({
      path: info.outputPath(`home-${width}.png`),
      fullPage: true,
    });
    await page.getByRole("link", { name: "Lớp", exact: true }).click();
    await expect(page.getByText("3 bài đang mở")).toBeVisible();
    await page.getByRole("link", { name: "Xem bài của lớp" }).click();
    await expect(page).toHaveURL(new RegExp(`classId=${classId}`));
    await expect(page.getByRole("combobox", { name: "Lớp học" })).toHaveText(
      classes[0]!.name,
    );
    await fits(page);
  });
}

test("unsaved settings survive crossing the desktop breakpoint and password visibility is reversible", async ({
  page,
}, info) => {
  await student(page);
  await page.setViewportSize({ width: 1024, height: 900 });
  await page.goto("/app/settings");
  const name = page.getByRole("textbox", { name: "Họ và tên" });
  await name.fill("Tên đang chỉnh sửa");
  await page.setViewportSize({ width: 320, height: 900 });
  await expect(name).toHaveValue("Tên đang chỉnh sửa");
  await fits(page);
  const password = page.getByLabel("Mật khẩu mới", { exact: true });
  await password.fill("Test-only-password");
  await page.getByRole("button", { name: "Hiện mật khẩu" }).last().click();
  await expect(password).toHaveAttribute("type", "text");
  await page.getByRole("button", { name: "Ẩn mật khẩu" }).click();
  await expect(password).toHaveAttribute("type", "password");
  await page.setViewportSize({ width: 1440, height: 900 });
  await expect(name).toHaveValue("Tên đang chỉnh sửa");
  await expect(password).toHaveValue("Test-only-password");
  await page.screenshot({ path: info.outputPath("settings-1440.png"), fullPage: true });
});

test("result filters survive resizing, explain empty results and retain the full title", async ({
  page,
}) => {
  const result: AttemptResult = {
    testTitle: "Bài kiểm tra có tiêu đề dài cần hiển thị đầy đủ trên điện thoại",
    maxAttempts: 2,
    review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
    attempt: {
      id: "result",
      assignmentId: "assignment",
      studentId: studentUser.id,
      testVersionId: "version",
      attemptNo: 1,
      status: "submitted",
      startedAt: "2026-09-22T00:00:00Z",
      deadlineAt: "2026-09-22T01:00:00Z",
      submittedAt: "2026-09-22T00:30:00Z",
      score: { earned: 1, total: 1, pendingManual: 0 },
    },
    questions: [
      {
        id: "question",
        type: "short_answer",
        prompt: "Câu trả lời đã chấm",
        points: 1,
        answer: { type: "text", value: "Đã trả lời" },
        earned: 1,
        pendingManual: false,
      },
    ],
  };
  await stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/attempts/result/result": { body: result },
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/app/attempts/result/result");
  const wrong = page.getByRole("button", { name: /^Sai/ });
  await wrong.click();
  await page.setViewportSize({ width: 320, height: 900 });
  await expect(wrong).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(result.testTitle);
  await expect(page.getByText("Không có câu sai trong bài này.")).toBeVisible();
  await fits(page);
  await page.getByRole("button", { name: "Xem tất cả câu" }).click();
  await expect(page.getByText("Câu trả lời đã chấm")).toBeVisible();
});

test("English student controls fit a 320px phone", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("quizzivy.locale", "en"));
  await student(page);
  await page.setViewportSize({ width: 320, height: 900 });
  await page.goto("/app");
  await expect(page.getByRole("button", { name: "Continue the test" })).toHaveCount(2);
  await fits(page);
  await page.goto("/app/settings");
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();
  await fits(page);
});
