import { expect, test, type Page } from "@playwright/test";
import type { Dashboard, TeacherSummary } from "@/features/dashboard/api";
import { adminUser, sessionAs, stubApi } from "./support/api";
import {
  ASSIGNMENT_ID,
  ATTEMPT_ID,
  assignment,
  monitor,
  review,
} from "../units/attempts/fixtures";

const path = `/teacher/assignments/${ASSIGNMENT_ID}`;
const title =
  "Bài kiểm tra đọc hiểu và nghe giữa kỳ dành cho lớp luyện thi IELTS nền tảng buổi tối";
const student = "Phạm Gia Hân Nguyễn Thị Hoàng Anh";

async function prepare(
  page: Page,
  theme: "light" | "dark" = "light",
  collapsed = false,
) {
  await page.addInitScript(
    ({ theme, collapsed }) => {
      localStorage.setItem("quizzivy.theme", theme);
      localStorage.setItem("quizzivy.locale", "vi");
      if (collapsed) localStorage.setItem("quizzivy.sidebar", "collapsed");
    },
    { theme, collapsed },
  );
  const user = { ...adminUser, preferences: { theme } };
  let note: string | null = "Ghi chú đã lưu";
  const rows = monitor().rows.map((row) =>
    row.attemptId === ATTEMPT_ID ? { ...row, fullName: student } : row,
  );
  const data = review();
  await stubApi(page, {
    ...sessionAs(user),
    "GET /teacher/dashboard": {
      body: {
        openAssignments: 1,
        awaitingGrading: 2,
        activeStudents: 3,
        flaggedAttempts: 1,
        newestFlaggedAttempt: null,
        recentAttempts: [],
        takingNow: { students: 0, assignments: 0, assignmentId: null },
        submissions: {
          days: Array.from({ length: 14 }, (_, index) => ({
            date: new Date(Date.UTC(2026, 8, 23 + index)).toISOString().slice(0, 10),
            count: 0,
          })),
          total: 0,
          averagePercent: null,
        },
        today: [],
        recentActivity: [],
      } satisfies Dashboard,
    },
    "GET /teacher/summary": {
      body: {
        liveAssignments: 1,
        answersToGrade: 2,
        unreadNotifications: 0,
      } satisfies TeacherSummary,
    },
    [`GET /teacher/assignments/${ASSIGNMENT_ID}`]: {
      body: assignment({ testTitle: title }),
    },
    [`GET /teacher/assignments/${ASSIGNMENT_ID}/attempts`]: { body: monitor(rows) },
    [`GET /teacher/tests/${assignment().testId}/versions`]: { body: { items: [] } },
    [`GET /teacher/attempts/${ATTEMPT_ID}`]: async (route) => {
      await route.fulfill({
        json: {
          ...data,
          student: { ...data.student, fullName: student },
          teacherNote: note,
        },
      });
    },
    [`GET /teacher/attempts/${ATTEMPT_ID}/events`]: {
      body: { startedAt: data.attempt.startedAt, summary: data.integrity, events: [] },
    },
    [`PATCH /teacher/attempts/${ATTEMPT_ID}/note`]: async (route) => {
      const body = route.request().postDataJSON() as { note: string | null };
      note = body.note;
      await route.fulfill({ json: { note } });
    },
  });
}

async function noOverflow(page: Page) {
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
}

for (const theme of ["light", "dark"] as const) {
  for (const width of [360, 768, 1024, 1280, 1440]) {
    test(`real assignment route at ${width}px in ${theme}`, async ({ page }) => {
      await prepare(page, theme);
      await page.setViewportSize({ width, height: 900 });
      await page.goto(path);
      await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
      await expect(page.locator("html")).toHaveClass(
        theme === "dark" ? /dark/ : /^(?!.*\bdark\b)/,
      );
      const table = page.getByRole("table", { name: "Học viên được giao bài" });
      await expect(table).toBeVisible();
      await expect(table.getByRole("row").nth(1)).toHaveCSS("height", "54px");
      await noOverflow(page);
      const opener = page.getByRole("button", { name: new RegExp(`^${student}`) });
      await opener.click();
      const sheet = page.getByRole("dialog", { name: student });
      await expect(sheet).toBeVisible();
      await expect(sheet).toHaveCSS("width", `${Math.min(420, width)}px`);
      expect(
        await sheet.evaluate((element) => (element as HTMLElement).offsetWidth),
      ).toBe(Math.min(420, width));
      await expect(sheet.getByLabel("Ghi chú riêng")).toHaveValue("Ghi chú đã lưu");
      await page.keyboard.press("Escape");
      await expect(sheet).toBeHidden();
      await expect(opener).toBeFocused();
      await noOverflow(page);
    });
  }
}

test("the real route persists keyboard collapse and keeps its page and draft across resize", async ({
  page,
}) => {
  await prepare(page);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
  const collapse = page.getByRole("button", { name: "Thu gọn thanh bên" });
  await collapse.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("button", { name: "Mở rộng thanh bên" })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  expect(await page.evaluate(() => localStorage.getItem("quizzivy.sidebar"))).toBe(
    "collapsed",
  );
  await page.reload();
  await expect(page.getByRole("button", { name: "Mở rộng thanh bên" })).toBeVisible();
  await page.getByRole("button", { name: student, exact: true }).click();
  const note = page.getByLabel("Ghi chú riêng");
  await note.fill("Bản nháp vẫn còn khi đổi chiều rộng");
  await note.evaluate((node) => {
    node.setAttribute("data-kept-node", "yes");
  });
  for (const width of [767, 768, 360, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(note).toHaveAttribute("data-kept-node", "yes");
    await expect(note).toHaveValue("Bản nháp vẫn còn khi đổi chiều rộng");
    await expect(note).toBeFocused();
  }
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();
  await page.getByRole("button", { name: student, exact: true }).click();
  await expect(note).toHaveValue("Bản nháp vẫn còn khi đổi chiều rộng");
});

test("phone drawer opens by keyboard, traps focus, and returns to its trigger", async ({
  page,
}) => {
  await prepare(page);
  await page.setViewportSize({ width: 360, height: 900 });
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
  const trigger = page.getByRole("button", { name: "Mở menu" });
  await trigger.focus();
  await page.keyboard.press("Space");
  const drawer = page.getByRole("dialog");
  await expect(drawer).toBeVisible();
  await page.keyboard.press("Shift+Tab");
  expect(await drawer.evaluate((node) => node.contains(document.activeElement))).toBe(
    true,
  );
  await page.keyboard.press("Escape");
  await expect(drawer).toBeHidden();
  await expect(trigger).toBeFocused();
  await noOverflow(page);
});

for (const [column, threshold] of [
  ["Trạng thái", 560],
  ["Rời trang", 700],
  ["Thời gian", 820],
  ["Đã nộp", 940],
] as const) {
  test(`${column} changes at actual content width ${threshold}`, async ({ page }) => {
    await prepare(page);
    await page.setViewportSize({ width: threshold + 304 - 1, height: 900 });
    await page.goto(path);
    const table = page.getByRole("table");
    await expect(table).toBeVisible();
    await expect(
      table.getByRole("columnheader", { name: column, exact: true }),
    ).toHaveCount(0);
    await page.setViewportSize({ width: threshold + 304, height: 900 });
    await expect(
      table.getByRole("columnheader", { name: column, exact: true }),
    ).toBeVisible();
    await noOverflow(page);
  });
}

test("deep-linked sheet returns focus to the real page and close replaces only attempt", async ({
  page,
}) => {
  await prepare(page);
  await page.goto(`${path}?attempt=${ATTEMPT_ID}&q=kept&other=keep#anchor`);
  const sheet = page.getByRole("dialog", { name: student });
  await expect(sheet).toBeVisible();
  await expect(sheet.getByLabel("Ghi chú riêng")).toHaveValue("Ghi chú đã lưu");
  await sheet.getByRole("button", { name: "Đóng", exact: true }).click();
  await expect(sheet).toBeHidden();
  const url = new URL(page.url());
  expect(url.searchParams.has("attempt")).toBe(false);
  expect(url.searchParams.get("q")).toBe("kept");
  expect(url.searchParams.get("other")).toBe("keep");
  expect(url.hash).toBe("#anchor");
  await expect(page.getByRole("main")).toBeFocused();
});

type NativeMotion = {
  frames: { target: "sheet" | "overlay"; duration: number; easing: string }[];
  middleTransform: string;
  settledTransform: string;
  middleOpacity: string;
  settledOpacity: string;
};
type MotionWindow = Window & { r425Motion?: Promise<NativeMotion> };

async function observeSheetMotion(page: Page, state: "open" | "closed") {
  await page.evaluate((wanted) => {
    function nextFrame() {
      return new Promise<number>((resolve) => requestAnimationFrame(resolve));
    }
    async function measure(sheet: HTMLElement, overlay: HTMLElement) {
      const sheetAnimations = sheet.getAnimations();
      const overlayAnimations = overlay.getAnimations();
      const labeled = [
        ...sheetAnimations.map((animation) => ({
          animation,
          target: "sheet" as const,
        })),
        ...overlayAnimations.map((animation) => ({
          animation,
          target: "overlay" as const,
        })),
      ];
      const frames = labeled.map(({ animation, target }) => ({
        target,
        duration: Number(animation.effect!.getComputedTiming().duration),
        easing: getComputedStyle(target === "sheet" ? sheet : overlay)
          .animationTimingFunction,
      }));
      const animations = [...sheetAnimations, ...overlayAnimations];
      for (const animation of animations) {
        animation.pause();
        animation.currentTime = 100;
      }
      await nextFrame();
      const middleTransform = getComputedStyle(sheet).transform;
      const middleOpacity = getComputedStyle(overlay).opacity;
      for (const animation of animations) animation.currentTime = 200;
      await nextFrame();
      const settledTransform = getComputedStyle(sheet).transform;
      const settledOpacity = getComputedStyle(overlay).opacity;
      for (const animation of animations) animation.finish();
      return {
        frames,
        middleTransform,
        settledTransform,
        middleOpacity,
        settledOpacity,
      };
    }
    let resolveMotion!: (value: NativeMotion) => void;
    (window as MotionWindow).r425Motion = new Promise<NativeMotion>((resolve) => {
      resolveMotion = resolve;
    });
    const observer = new MutationObserver(() => {
      const sheet = document.querySelector<HTMLElement>('[data-slot="sheet"]');
      const overlay = document.querySelector<HTMLElement>(
        '[data-slot="sheet-overlay"]',
      );
      if (sheet?.dataset.state !== wanted || overlay === null) return;
      if (sheet.getAnimations().length === 0 || overlay.getAnimations().length === 0)
        return;
      observer.disconnect();
      void measure(sheet, overlay).then(resolveMotion);
    });
    observer.observe(document.body, {
      subtree: true,
      childList: true,
      attributes: true,
      attributeFilter: ["data-state"],
    });
  }, state);
}

test("sheet and overlay entry/exit have native 200ms ease intermediate frames and reduced static state", async ({
  page,
}) => {
  await prepare(page);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(path);
  await expect(page.getByRole("button", { name: student, exact: true })).toBeVisible();
  await observeSheetMotion(page, "open");
  await page.getByRole("button", { name: student, exact: true }).click();
  const entry = await page.evaluate(() => (window as MotionWindow).r425Motion!);
  expect(entry.frames.some((frame) => frame.target === "sheet")).toBe(true);
  expect(entry.frames.some((frame) => frame.target === "overlay")).toBe(true);
  for (const frame of entry.frames) {
    expect(frame.duration).toBe(200);
    expect(frame.easing).toBe("ease");
  }
  expect(entry.middleTransform).not.toBe(entry.settledTransform);
  expect(entry.middleOpacity).not.toBe(entry.settledOpacity);
  const sheet = page.getByRole("dialog");
  await observeSheetMotion(page, "closed");
  await sheet.getByRole("button", { name: "Đóng", exact: true }).click();
  const exit = await page.evaluate(() => (window as MotionWindow).r425Motion!);
  expect(exit.frames.some((frame) => frame.target === "sheet")).toBe(true);
  expect(exit.frames.some((frame) => frame.target === "overlay")).toBe(true);
  for (const frame of exit.frames) {
    expect(frame.duration).toBe(200);
    expect(frame.easing).toBe("ease");
  }
  expect(exit.middleTransform).not.toBe(exit.settledTransform);
  expect(exit.middleOpacity).not.toBe(exit.settledOpacity);
  await expect(sheet).toBeHidden();
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.getByRole("button", { name: student, exact: true }).click();
  await expect(sheet).toHaveCSS("animation-name", "none");
  await expect(page.locator('[data-slot="sheet-overlay"]')).toHaveCSS(
    "animation-name",
    "none",
  );
  expect(await sheet.evaluate((node) => node.getAnimations().length)).toBe(0);
});

for (const theme of ["light", "dark"] as const) {
  for (const width of [360, 768, 1024, 1280, 1440]) {
    test(`progress card at ${width}px in ${theme} draws the deck's card inside the page`, async ({
      page,
    }) => {
      await prepare(page, theme);
      await page.setViewportSize({ width, height: 900 });
      await page.goto(path);
      const card = page.locator('[data-slot="progress-card"]');
      await expect(card.getByText("Đã nộp 1/3")).toBeVisible();
      const box = await card.evaluate((node) => {
        const style = getComputedStyle(node);
        const outer = node.getBoundingClientRect();
        const inside = [...node.querySelectorAll("*")].every((child) => {
          const rect = child.getBoundingClientRect();
          return rect.left >= outer.left - 0.5 && rect.right <= outer.right + 0.5;
        });
        return {
          padding: style.padding,
          radius: style.borderRadius,
          main: document.querySelector("main")!.getBoundingClientRect().width,
          width: outer.width,
          inside,
        };
      });
      expect(box.padding).toBe("12px 16px");
      expect(box.radius).toBe("12px");
      expect(box.inside).toBe(true);
      expect(box.width).toBeLessThanOrEqual(box.main);
      await expect(card.getByText("Đã nộp 1/3")).toHaveCSS("font-size", "13.5px");
      await expect(card.getByText("Đã nộp 1/3")).toHaveCSS("font-weight", "600");
      await expect(card.locator('[data-slot="progress-bar"]')).toHaveCSS(
        "height",
        "8px",
      );
      const legend = card.getByRole("group", { name: "Lọc học viên" });
      const flagged = legend.getByRole("button", { name: "Bị gắn cờ 1" });
      await expect(flagged).toHaveCSS("height", "28px");
      await expect(flagged).toHaveCSS("border-radius", "7px");
      await expect(flagged).toHaveCSS("font-size", "12.5px");
      await expect(card.getByText("—")).toHaveCSS("font-size", "15px");
      await noOverflow(page);
    });
  }
}

test("the progress legend filters the roster from the keyboard and names the filter in the URL", async ({
  page,
}) => {
  await prepare(page);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`${path}?tab=questions&page=2`);
  const legend = page.getByRole("group", { name: "Lọc học viên" });
  const inProgress = legend.getByRole("button", { name: "Đang làm 1" });
  await inProgress.focus();
  await expect(inProgress).toHaveCSS("outline-style", "solid");
  await expect(inProgress).toHaveCSS("outline-width", "2px");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\?tab=students&roster=inProgress$/);
  await expect(inProgress).toHaveAttribute("aria-pressed", "true");
  const table = page.getByRole("table", { name: "Học viên được giao bài" });
  await expect(table.getByRole("row")).toHaveCount(2);
  await expect(table.getByText(student)).toBeVisible();
  await page.keyboard.press("Space");
  await expect(page).toHaveURL(/\?tab=students$/);
  await expect(table.getByRole("row")).toHaveCount(4);
});
