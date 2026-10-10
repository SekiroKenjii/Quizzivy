import { expect, type Page } from "@playwright/test";

/** Shared by the live specs, which talk to a real API over the seeded database. */

export const ADMIN = { email: "thuong@quizzivy.com", password: "quizzivy-dev" };
export const STUDENT = { email: "hocvien@quizzivy.com", password: "quizzivy-dev" };
export const TEACHER = { email: "giaovien@quizzivy.com", password: "quizzivy-dev" };

/** The assignments seed/04-dev-e2e.sql exists to provide. */
export const ASSIGNMENT = {
  timer: "01935000-0000-7000-8000-00000000ee01",
  integrity: "01935000-0000-7000-8000-00000000ee02",
  takeover: "01935000-0000-7000-8000-00000000ee03",
  persistence: "01935000-0000-7000-8000-00000000ee05",
  payload: "01935000-0000-7000-8000-00000000ee09",
};

export async function signIn(page: Page, who: typeof ADMIN, landing: RegExp) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(who.email);
  await page.getByLabel("Mật khẩu", { exact: true }).fill(who.password);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(landing);
}

export const signInAsStudent = (page: Page) => signIn(page, STUDENT, /\/app$/);
export const signInAsAdmin = (page: Page) => signIn(page, ADMIN, /\/teacher$/);

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
  const asks = ((await start.textContent()) ?? "").includes("Bắt đầu");
  await start.click();
  if (asks) {
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Bắt đầu", exact: true })
      .click();
  }
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

/** Hands the open attempt in through the Submit dialog and goes home. */
export async function submitAttempt(page: Page) {
  await page
    .getByRole("banner")
    .getByRole("button", { name: "Nộp bài", exact: true })
    .click();
  const dialog = page.getByRole("dialog", { name: /^Nộp bài/ });
  await expect(dialog).toBeVisible();
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

/**
 * Assigns a published test to the seeded class through the wizard, with
 * every other choice left at its default, and lands on the list.
 */
export async function assignToClass(page: Page, title: string) {
  await pickInWizard(page, title);
  await page.getByRole("button", { name: "Tiếp tục", exact: true }).click();
  await page.getByRole("button", { name: "Tiếp tục", exact: true }).click();
  await page.getByRole("button", { name: /^Giao (cho \d+ học viên|bài)$/ }).click();
  await expect(page).toHaveURL(/\/teacher\/assignments\?status=(open|scheduled)$/, {
    timeout: 30_000,
  });
  await page.goto("/teacher/assignments");
}

/**
 * Opens the new-assignment wizard with `query` (such as `?test=<id>`), picks
 * `title` unless the query preselected it, ticks the seeded class and stops
 * on the Students step.
 */
export async function pickInWizard(page: Page, title: string, query = "") {
  await page.goto(`/teacher/assignments/new${query}`);
  const test = page.getByRole("radio", { name: new RegExp(title) });
  if (query.includes("test=")) await expect(test).toBeChecked();
  else await test.click();
  await page.getByRole("button", { name: "Tiếp tục", exact: true }).click();
  await page
    .getByRole("checkbox", { name: /Tiếng Anh giao tiếp/ })
    .first()
    .click();
}

/** Types `value` into the stepper named `name` and commits it with Enter. */
export async function fillStepper(page: Page, name: string, value: string) {
  const field = page.getByRole("spinbutton", { name });
  await field.fill(value);
  await field.press("Enter");
  await expect(field).toHaveAttribute("aria-valuenow", value);
}
