import { expect, test, type Locator, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import {
  dashboard23,
  dashboard23Assignments,
  dashboard23Summary,
  DASHBOARD23_IDS,
} from "../support/dashboard23";
import { contractJson } from "../support/contractResponse";
import type { DashboardRange } from "../../src/features/dashboard/view";

async function prepare(
  page: Page,
  theme: "light" | "dark",
  reduced = false,
  delayed: boolean | number = false,
  zero = false,
) {
  await page.addInitScript(
    (value) => localStorage.setItem("quizzivy.theme", value),
    theme,
  );
  await page.emulateMedia({
    colorScheme: theme,
    reducedMotion: reduced ? "reduce" : "no-preference",
  });
  let homeReads = 0;
  await stubApi(page, {
    ...sessionAs(adminUser),
    "GET /teacher/dashboard": async (route) => {
      homeReads++;
      if (delayed)
        await new Promise((resolve) =>
          setTimeout(resolve, typeof delayed === "number" ? delayed : 500),
        );
      const range = new URL(route.request().url()).searchParams.get(
        "range",
      ) as DashboardRange;
      const body = dashboard23(range);
      if (zero) {
        body.takingNow.students = 0;
        body.takingNow.assignments = 0;
        body.takingNow.assignmentId = null;
      }
      if (homeReads > 1)
        body.submissions.days = body.submissions.days.map((day, index) => ({
          ...day,
          count: index === 0 ? 5 : day.count,
        }));
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          await contractJson("/teacher/dashboard", "get", 200, body).json(),
        ),
      });
    },
    "GET /teacher/summary": { body: dashboard23Summary() },
    "GET /me/summary": { body: { unreadNotifications: 0 } },
    "GET /teacher/assignments": { body: dashboard23Assignments() },
  });
}

async function sample(node: Locator, duration: number) {
  return node.evaluate(
    (element, ms) =>
      new Promise<
        Readonly<{
          elapsed: number;
          opacity: number;
          height: number;
          color: string;
          background: string;
          animation: string;
          duration: string;
          easing: string;
          play: string;
          transform: string;
          transitionDuration: string;
          transitionEasing: string;
        }>[]
      >((resolve) => {
        const start = performance.now();
        const frames: Readonly<{
          elapsed: number;
          opacity: number;
          height: number;
          color: string;
          background: string;
          animation: string;
          duration: string;
          easing: string;
          play: string;
          transform: string;
          transitionDuration: string;
          transitionEasing: string;
        }>[] = [];
        const frame = () => {
          const css = getComputedStyle(element);
          const elapsed = performance.now() - start;
          frames.push({
            elapsed,
            opacity: Number(css.opacity),
            height: element.getBoundingClientRect().height,
            color: css.borderColor,
            background: css.backgroundPosition,
            animation: css.animationName,
            duration: css.animationDuration,
            easing: css.animationTimingFunction,
            play: css.animationPlayState,
            transform: css.transform,
            transitionDuration: css.transitionDuration,
            transitionEasing: css.transitionTimingFunction,
          });
          if (elapsed >= ms) resolve(frames);
          else requestAnimationFrame(frame);
        };
        requestAnimationFrame(frame);
      }),
    duration,
  );
}

for (const theme of ["light", "dark"] as const) {
  for (const width of [360, 768, 1024, 1280, 1440]) {
    test(`actual dashboard ${width}px ${theme}`, async ({ page }, info) => {
      await prepare(page, theme);
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/teacher?other=A&other=B#work");
      const main = page.getByRole("main");
      await expect(main.getByRole("heading", { name: "Bài đang mở" })).toBeVisible();
      await expect(page.getByRole("link", { name: /Bài bị gắn cờ/ })).toHaveAttribute(
        "href",
        `/teacher/assignments/${DASHBOARD23_IDS.assignment}?tab=students&attempt=${DASHBOARD23_IDS.attempt}`,
      );
      await expect(page.getByRole("link", { name: /Đang làm bài/ })).toHaveAttribute(
        "href",
        `/teacher/assignments/${DASHBOARD23_IDS.taking}?tab=students`,
      );
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      ).toBe(true);
      expect(await main.evaluate((node) => node.scrollWidth <= node.clientWidth)).toBe(
        true,
      );
      const range = page.getByRole("button", { name: "7 ngày" });
      await range.focus();
      await page.keyboard.press("Enter");
      await expect(page).toHaveURL(/other=A&other=B&range=7d#work$/);
      await expect(range).toBeFocused();
      await expect(page.getByText(/7 trong 7 ngày qua/)).toBeVisible();
      await expect(main.locator('[data-slot="page"]')).toHaveCSS("max-width", "1320px");
      await page.screenshot({
        path: info.outputPath(`dashboard-${width}-${theme}.png`),
        fullPage: true,
      });
    });
  }
  test(`natural loading shimmer and static settled cards ${theme}`, async ({
    page,
  }, info) => {
    await prepare(page, theme, false, 2000);
    await page.goto("/teacher");
    const skeleton = page.locator('[data-slot="skeleton"]').first();
    await expect(skeleton).toBeVisible();
    const loading = await sample(skeleton, 500);
    expect(loading.every((frame) => frame.animation === "qz-shimmer")).toBe(true);
    expect(loading[0]!.duration).toBe("1.4s");
    expect(loading[0]!.easing).toBe("linear");
    expect(new Set(loading.map((frame) => frame.background)).size).toBeGreaterThan(10);
    const progress = page.getByRole("progressbar");
    await expect(progress).toBeVisible();
    expect(
      await progress
        .locator("span")
        .evaluate((node) => getComputedStyle(node).transitionDuration),
    ).toBe("0s");
    await expect(page.locator('[data-slot="teacher-dashboard"]')).toHaveCSS(
      "animation-name",
      "none",
    );
    await info.attach("natural-loading", {
      body: JSON.stringify(loading),
      contentType: "application/json",
    });
  });
  test(`zero-work plain live tile pauses without a new tabstop ${theme}`, async ({
    page,
  }, info) => {
    await prepare(page, theme, false, false, true);
    await page.goto("/teacher");
    const tile = page
      .locator('[class~="group/kpi"]')
      .filter({ has: page.getByText("Đang làm bài", { exact: true }) });
    const dot = tile.locator(".qz-live-dot");
    await expect(dot).toBeVisible();
    expect(await tile.evaluate((node) => node.tagName)).toBe("DIV");
    await expect(tile).not.toHaveAttribute("tabindex");
    await expect(tile.getByRole("link")).toHaveCount(0);
    const idle = await sample(dot, 500);
    expect(new Set(idle.map((frame) => frame.opacity.toFixed(2))).size).toBeGreaterThan(
      2,
    );
    await tile.hover();
    const frozen = await sample(dot, 300);
    expect(frozen.every((frame) => frame.play === "paused")).toBe(true);
    expect(
      Math.max(...frozen.map((frame) => frame.opacity)) -
        Math.min(...frozen.map((frame) => frame.opacity)),
    ).toBeLessThan(0.001);
    await page.getByRole("heading", { level: 1 }).hover();
    const resumed = await sample(dot, 500);
    expect(
      new Set(resumed.map((frame) => frame.opacity.toFixed(2))).size,
    ).toBeGreaterThan(2);
    await info.attach("plain-live-motion", {
      body: JSON.stringify({ idle, frozen, resumed }),
      contentType: "application/json",
    });
  });
  test(`natural KPI border and live-dot pause/resume ${theme}`, async ({
    page,
  }, info) => {
    await prepare(page, theme);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/teacher");
    const tile = page.getByRole("link", { name: /Đang làm bài/ });
    const dot = tile.locator(".qz-live-dot");
    await expect(dot).toBeVisible();
    const idle = await sample(dot, 1650);
    expect(new Set(idle.map((frame) => frame.opacity.toFixed(2))).size).toBeGreaterThan(
      10,
    );
    expect(idle[0]!.duration).toBe("1.6s");
    expect(idle[0]!.easing).toBe("ease");
    const before = await tile.evaluate((node) => getComputedStyle(node).borderColor);
    await tile.hover();
    const [frozen, border] = await Promise.all([sample(dot, 300), sample(tile, 170)]);
    expect(frozen.every((frame) => frame.play === "paused")).toBe(true);
    expect(
      Math.max(...frozen.map((frame) => frame.opacity)) -
        Math.min(...frozen.map((frame) => frame.opacity)),
    ).toBeLessThan(0.001);
    expect(new Set(border.map((frame) => frame.color)).size).toBeGreaterThan(2);
    expect(border.at(-1)!.color).not.toBe(before);
    expect(border[0]!.transitionDuration).toBe("0.15s");
    expect(border[0]!.transitionEasing).toBe("ease");
    expect(border.every((frame) => frame.transform === "none")).toBe(true);
    await page.getByRole("heading", { level: 1 }).hover();
    expect(
      await page.getByRole("main").evaluate((node) => node.matches(":hover")),
    ).toBe(true);
    const resumed = await sample(dot, 500);
    expect(
      new Set(resumed.map((frame) => frame.opacity.toFixed(2))).size,
    ).toBeGreaterThan(2);
    for (let at = 0; at < 35; at++) {
      await page.keyboard.press("Tab");
      if (await tile.evaluate((node) => document.activeElement === node)) break;
    }
    await expect(tile).toBeFocused();
    const focused = await sample(dot, 250);
    expect(focused.every((frame) => frame.play === "paused")).toBe(true);
    await page.keyboard.press("Tab");
    const blur = await sample(dot, 500);
    expect(blur.every((frame) => frame.play === "running")).toBe(true);
    expect(new Set(blur.map((frame) => frame.opacity.toFixed(2))).size).toBeGreaterThan(
      2,
    );
    await info.attach("natural-live-motion", {
      body: JSON.stringify({ idle, frozen, border, resumed, focused, blur }),
      contentType: "application/json",
    });
  });
  test(`natural retained-column height transition on actual home poll ${theme}`, async ({
    page,
  }, info) => {
    test.setTimeout(60_000);
    await prepare(page, theme);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/teacher");
    const chart = page.getByRole("region", { name: "Bài nộp" });
    const bar = chart.locator('[aria-hidden="true"] div[title]').first().locator("div");
    await expect(bar).toHaveCSS("height", "0px");
    const unchanged = await bar.elementHandle();
    await page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/teacher/dashboard" &&
        response.status() === 200,
      { timeout: 35_000 },
    );
    const frames = await sample(bar, 260);
    expect(frames[0]!.transitionDuration).toBe("0.2s");
    expect(frames[0]!.transitionEasing).toBe("ease");
    expect(frames.at(-1)!.height).toBeGreaterThan(0);
    expect(
      frames.some((frame) => frame.height > 0 && frame.height < frames.at(-1)!.height),
    ).toBe(true);
    expect(await unchanged!.evaluate((node) => node.isConnected)).toBe(true);
    await info.attach("natural-chart-motion", {
      body: JSON.stringify(frames),
      contentType: "application/json",
    });
  });
  test(`reduced dashboard is static from first visible loading and live frames ${theme}`, async ({
    page,
  }, info) => {
    await prepare(page, theme, true, true);
    await page.goto("/teacher");
    const skeleton = page.locator('[data-slot="skeleton"]').first();
    await expect(skeleton).toBeVisible();
    expect(
      await skeleton.evaluate((node) => getComputedStyle(node).animationName),
    ).toBe("none");
    const tile = page.getByRole("link", { name: /Đang làm bài/ });
    const dot = tile.locator(".qz-live-dot");
    await expect(dot).toBeVisible();
    const frames = await sample(dot, 350);
    expect(
      frames.every((frame) => frame.animation === "none" && frame.opacity === 1),
    ).toBe(true);
    await expect(
      page
        .getByRole("region", { name: "Bài nộp" })
        .locator('[aria-hidden="true"] div[title]')
        .first()
        .locator("div"),
    ).toHaveCSS("transition-property", "none");
    await info.attach("reduced-static", {
      body: JSON.stringify(frames),
      contentType: "application/json",
    });
  });
}
