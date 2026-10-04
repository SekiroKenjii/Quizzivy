import { expect, test, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";
import { paper } from "./support/engine";
import { assignment, classes, fits, gradedResult } from "./support/student";
import type { components } from "../../src/lib/api/schema";

type Card = components["schemas"]["StudentAssignmentCard"];
type Detail = components["schemas"]["StudentAssignmentDetail"];

async function signIn(page: Page) {
  const open = assignment(3, false);
  const done: Card = {
    ...assignment(4, false),
    attemptsUsed: 2,
    lastAttemptId: gradedResult.attempt.id,
    lastSubmittedAt: gradedResult.attempt.submittedAt ?? null,
    score: gradedResult.attempt.score ?? null,
  };
  const intro: Detail = {
    id: open.id,
    testTitle: open.testTitle,
    className: classes[0]!.name,
    status: open.status,
    opensAt: open.opensAt,
    closesAt: open.closesAt,
    durationMinutes: open.durationMinutes,
    questionCount: open.questionCount,
    totalPoints: open.totalPoints,
    attemptsUsed: open.attemptsUsed,
    maxAttempts: open.maxAttempts,
    hasLiveAttempt: false,
    review: gradedResult.review,
    integrity: paper().integrity,
    hasAudio: false,
    showsTranscript: false,
  };
  await stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/classes": { body: { items: classes } },
    "GET /app/assignments": {
      body: { dueNow: [assignment(1, true), open], upcoming: [], completed: [done] },
    },
    [`GET /app/assignments/${open.id}`]: { body: intro },
    "GET /app/attempts/result/result": { body: gradedResult },
  });
  await page.goto("/app");
  await expect(page.getByRole("button", { name: "Tiếp tục làm bài" })).toBeVisible();
}

const tabBar = (page: Page) =>
  page.getByRole("navigation", { name: "Điều hướng chính" });

test("the destinations are a tab bar at the foot, and each tab opens its screen", async ({
  page,
}) => {
  await signIn(page);
  const tabs = tabBar(page);
  const banner = page.getByRole("banner");
  await expect(tabs).toHaveCount(1);
  await expect(banner.getByRole("navigation")).toHaveCount(0);
  await expect(banner.getByRole("button")).toHaveCount(1);
  await expect(banner.getByRole("button", { name: /^Tài khoản của/ })).toBeVisible();
  await expect(tabs.getByRole("link")).toHaveText([/Trang chủ$/, "Lớp", "Tôi"]);
  const bar = (await tabs.boundingBox())!;
  expect(bar.y + bar.height).toBeCloseTo(page.viewportSize()!.height, 0);

  const home = tabs.getByRole("link", { name: /Trang chủ$/ });
  const classesTab = tabs.getByRole("link", { name: "Lớp", exact: true });
  await expect(home).toHaveAttribute("aria-current", "page");
  await expect(home).toContainText("1 bài đến hạn trong 7 ngày");
  await expect(tabs.locator("[aria-current]")).toHaveCount(1);

  await classesTab.tap();
  await expect(page).toHaveURL(/\/app\/classes$/);
  await expect(
    page.getByRole("heading", { level: 2, name: classes[0]!.name }),
  ).toBeVisible();
  await expect(classesTab).toHaveAttribute("aria-current", "page");
  await expect(tabs.locator("[aria-current]")).toHaveCount(1);

  await home.tap();
  await expect(page).toHaveURL(/\/app$/);
  await expect(home).toHaveAttribute("aria-current", "page");
  await expect(tabs.locator("[aria-current]")).toHaveCount(1);
});

test("Me opens settings as a detail screen, and signing out stays behind the avatar", async ({
  page,
}) => {
  await signIn(page);
  await expect(page.getByRole("button", { name: "Đăng xuất" })).toHaveCount(0);

  await tabBar(page).getByRole("link", { name: "Tôi", exact: true }).tap();
  await expect(page).toHaveURL(/\/app\/settings$/);
  await expect(
    page.getByRole("banner").getByText("Cài đặt", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Họ và tên" })).toBeVisible();
  await expect(tabBar(page)).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Đăng xuất" })).toHaveCount(0);
  await page.getByRole("button", { name: /^Tài khoản của/ }).tap();
  await expect(page.getByRole("menuitem", { name: "Đăng xuất" })).toBeVisible();
});

test("a detail screen swaps the logo for a back arrow and its title, and back returns Home", async ({
  page,
}) => {
  await signIn(page);
  const banner = page.getByRole("banner");
  const logo = banner.getByRole("link", { name: "Trang chủ Quizzivy" });
  const back = banner.getByRole("link", { name: "Quay lại" });
  const ownBack = page.getByRole("main").getByRole("link", {
    name: "Trang chủ",
    exact: true,
  });
  await expect(logo).toBeVisible();
  await expect(back).toHaveCount(0);

  await page
    .getByRole("main")
    .getByRole("link", { name: /Bài luyện tập 3/ })
    .tap();
  await expect(page).toHaveURL(/\/app\/assignments\/assignment-3$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Bài luyện tập 3");
  await expect(banner.getByText("Bài kiểm tra", { exact: true })).toBeVisible();
  await expect(logo).toHaveCount(0);
  await expect(tabBar(page)).toHaveCount(0);
  await expect(ownBack).toHaveCount(0);
  await back.tap();
  await expect(page).toHaveURL(/\/app$/);
  await expect(logo).toBeVisible();
  await expect(tabBar(page)).toBeVisible();

  await page
    .getByRole("main")
    .getByRole("link", { name: /Bài luyện tập 4/ })
    .tap();
  await expect(page).toHaveURL(/\/app\/attempts\/result\/result$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(
    gradedResult.testTitle,
  );
  await expect(banner.getByText("Kết quả", { exact: true })).toBeVisible();
  await expect(logo).toHaveCount(0);
  await expect(tabBar(page)).toHaveCount(0);
  await expect(ownBack).toHaveCount(0);
  await back.tap();
  await expect(page).toHaveURL(/\/app$/);
  await expect(tabBar(page)).toBeVisible();
});

test("Home, Classes, the intro and the result do not scroll sideways", async ({
  page,
}) => {
  await signIn(page);
  await fits(page);

  await tabBar(page).getByRole("link", { name: "Lớp", exact: true }).tap();
  await expect(
    page.getByRole("heading", { level: 2, name: classes[0]!.name }),
  ).toBeVisible();
  await fits(page);

  await page.goto("/app/assignments/assignment-3");
  await expect(
    page.getByRole("button", { name: "Bắt đầu làm bài", exact: true }),
  ).toBeVisible();
  await fits(page);

  await page.goto("/app/attempts/result/result");
  await expect(page.getByText("Câu trả lời đã chấm")).toBeVisible();
  await fits(page);
});

test("every tab is at least 44px tall", async ({ page }) => {
  await signIn(page);
  const heights = await tabBar(page)
    .getByRole("link")
    .evaluateAll((tabs) => tabs.map((tab) => tab.getBoundingClientRect().height));
  expect(heights).toHaveLength(3);
  for (const height of heights) expect(height).toBeGreaterThanOrEqual(44);
});
