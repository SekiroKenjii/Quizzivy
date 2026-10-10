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
 * Assigns a published test to the seeded class and lands on the list.
 *
 * The wizard picks the test and the class and saves a draft; the edit form
 * then assigns it, until the wizard has its own Assign step (T-R4.27b).
 */
export async function assignToClass(page: Page, title: string) {
  const id = await saveDraftFromWizard(page, title);
  await page.goto(`/teacher/assignments/${id}/edit`);
  await page.getByRole("button", { name: "Giao bài", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/teacher/assignments/${id}$`), {
    timeout: 30_000,
  });
  await page.goto("/teacher/assignments");
}

/**
 * Picks `title` and the seeded class in the new-assignment wizard, saves a
 * draft and returns its id. `query` is the address's query string, such as
 * `?test=<id>`; a test it preselects is checked, not clicked.
 */
export async function saveDraftFromWizard(page: Page, title: string, query = "") {
  await page.goto(`/teacher/assignments/new${query}`);
  const test = page.getByRole("radio", { name: new RegExp(title) });
  if (query.includes("test=")) await expect(test).toBeChecked();
  else await test.click();
  await page.getByRole("button", { name: "Tiếp tục", exact: true }).click();
  await page
    .getByRole("checkbox", { name: /Tiếng Anh giao tiếp/ })
    .first()
    .click();
  const created = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/teacher/assignments"),
  );
  await page.getByRole("button", { name: "Lưu nháp", exact: true }).click();
  const assignment = (await (await created).json()) as { id: string };
  await expect(page).toHaveURL(/\/teacher\/assignments\?status=draft$/, {
    timeout: 30_000,
  });
  return assignment.id;
}

/**
 * publishInBuilder publishes the open builder's draft through "Publish test?",
 * waits for the builder to say so, and opens the test's own page.
 */
export async function publishInBuilder(page: Page) {
  const id = new URL(page.url()).pathname.split("/").at(-2);
  await page.getByRole("button", { name: "Phát hành", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Phát hành đề thi?", exact: true });
  await dialog.getByRole("button", { name: "Phát hành", exact: true }).click();
  await expect(page.getByText("Đã phát hành. Giờ bạn có thể giao bài.")).toBeVisible({
    timeout: 30_000,
  });
  await page.goto(`/teacher/tests/${id}`);
}
