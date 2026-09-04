import { expect, type Page } from "@playwright/test";

/** Shared by the live specs, which talk to a real API over the seeded database. */

export const ADMIN = { email: "thuong@quizzivy.com", password: "quizzivy-dev" };
export const STUDENT = { email: "hocvien@quizzivy.com", password: "quizzivy-dev" };

/** The assignments seed/04-dev-e2e.sql exists to provide. */
export const ASSIGNMENT = {
  timer: "01935000-0000-7000-8000-00000000ee01",
  integrity: "01935000-0000-7000-8000-00000000ee02",
  takeover: "01935000-0000-7000-8000-00000000ee03",
  persistence: "01935000-0000-7000-8000-00000000ee05",
};

export async function signIn(page: Page, who: typeof ADMIN, landing: RegExp) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(who.email);
  await page.getByLabel("Mật khẩu").fill(who.password);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(landing);
}

export const signInAsStudent = (page: Page) => signIn(page, STUDENT, /\/app$/);
export const signInAsAdmin = (page: Page) => signIn(page, ADMIN, /\/admin$/);

/**
 * Opens an assignment by id and starts or resumes it, returning the attempt id.
 *
 * By id rather than by clicking the card: the fixtures share one test title, so
 * the home offers three cards that read the same.
 */
export async function startAttempt(page: Page, assignmentId: string): Promise<string> {
  await page.goto(`/app/assignments/${assignmentId}`);
  const start = page.getByRole("button", {
    name: /^(Bắt đầu làm bài|Tiếp tục làm bài)$/,
  });
  await expect(start).toBeVisible();
  await start.click();
  await expect(page).toHaveURL(/\/app\/attempts\/[0-9a-f-]+$/);
  return page.url().split("/").pop() ?? "";
}

/**
 * A brand-new attempt, whatever the last run left behind.
 *
 * A spec that asserts on counters has to start from zero, and the fixtures
 * allow fifty attempts precisely so each run can have its own. Any attempt
 * still live from a previous run is submitted first, because submitting is the
 * only way the product ends one.
 */
export async function freshAttempt(page: Page, assignmentId: string): Promise<string> {
  await page.goto(`/app/assignments/${assignmentId}`);
  // Waited for, not polled: isVisible() answers before the intro has rendered,
  // and the resume branch was silently skipped every run.
  const control = page.getByRole("button", {
    name: /^(Bắt đầu làm bài|Tiếp tục làm bài)$/,
  });
  await expect(control).toBeVisible();
  if (((await control.textContent()) ?? "").includes("Tiếp tục")) {
    await control.click();
    await expect(page).toHaveURL(/\/app\/attempts\/[0-9a-f-]+$/);
    await submitAttempt(page);
  }
  return startAttempt(page, assignmentId);
}

/** Takes the open attempt through the review screen, hands it in and goes home. */
export async function submitAttempt(page: Page) {
  await page.getByRole("button", { name: "Xem lại & nộp" }).first().click();
  await page.getByRole("button", { name: "Nộp bài", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Nộp bài?")).toBeVisible();
  await dialog.getByRole("button", { name: "Nộp bài", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Bài đã được nộp." })).toBeVisible({
    timeout: 30_000,
  });
  await page.getByRole("button", { name: "Về trang chủ" }).click();
  await expect(page).toHaveURL(/\/app$/);
}

/**
 * One away episode, as the monitor sees it: blur, then focus.
 *
 * The hook listens to both visibilitychange and window blur/focus and treats
 * them as one absence, so a blur/focus pair is an episode whether or not the
 * browser really backgrounded the tab. Playwright cannot hide a page it is
 * driving, which is why this drives the events the hook actually binds.
 */
export async function goAway(page: Page, ms: number) {
  await page.evaluate(() => window.dispatchEvent(new Event("blur")));
  await page.waitForTimeout(ms);
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
}

/**
 * Picks a choice option by its text.
 *
 * The radio itself is `sr-only`, so it is the label that gets clicked — which
 * is what a student clicks too.
 */
export async function chooseOption(page: Page, text: string) {
  await page.locator("label").filter({ hasText: text }).click();
}
