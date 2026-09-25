import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import {
  previewGroup,
  previewQuestions,
  previewSection,
} from "../support/groupPreview";

const testID = "01935000-0000-7000-8000-000000000008";

async function setup(page: Page) {
  await stubApi(page, {
    ...sessionAs(adminUser),
    [`GET /admin/tests/${testID}`]: {
      body: {
        id: testID,
        title: "Đề kiểm tra ngữ liệu",
        status: "published",
        currentVersion: 1,
        totalPoints: 3,
        questionCount: 3,
        audioCount: 2,
        sections: [],
        createdAt: "2026-09-24T00:00:00Z",
        updatedAt: "2026-09-24T00:00:00Z",
      },
    },
    [`GET /admin/tests/${testID}/versions`]: {
      body: {
        items: [2, 1].map((version) => ({
          id: `01935000-0000-7000-8000-00000000000${version}`,
          version,
          totalPoints: 3,
          questionCount: 3,
          audioCount: 2,
          manualCount: 0,
          publishedAt: "2026-09-24T00:00:00Z",
          publishedBy: "Giáo viên",
        })),
      },
    },
    [`GET /admin/tests/${testID}/preview`]: (route) => {
      const version = Number(
        new URL(route.request().url()).searchParams.get("version") ?? 1,
      );
      return route.fulfill({
        json: {
          version,
          questions: previewQuestions,
          sections: [previewSection],
          groups: [{ ...previewGroup, title: `${previewGroup.title} · v${version}` }],
        },
      });
    },
  });
  await page.route("https://assets.example/**", (route) => route.abort());
  await page.goto(`/admin/tests/${testID}`);
}

for (const width of [768, 1440]) {
  test(`group preview preserves context and keyboard targets at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await setup(page);
    await expect(
      page.getByRole("heading", { name: `${previewGroup.title} · v1` }),
    ).toBeVisible();
    await expect(page.getByText("Ngữ liệu dùng chung · Câu 2–3")).toBeVisible();
    const gap = page.getByRole("link", { name: "Ô A — chuyển đến câu 3" });
    await gap.focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(":focus")).toContainText("Câu 3");
    await page.getByRole("radio", { name: "Điện thoại", exact: true }).click();
    const viewport = page.locator('[data-preview-viewport="phone"]');
    await expect(viewport).toHaveCSS("width", "320px");
    const overflow = await viewport.evaluate(
      (element) => element.scrollWidth > element.clientWidth + 1,
    );
    expect(overflow).toBe(false);
    await gap.focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(":focus")).toContainText("Câu 3");
    const clock = page.getByText("0:00 / 0:01", { exact: true });
    expect(
      await clock.evaluate((element) => element.getBoundingClientRect().height),
    ).toBeLessThan(24);
    await page
      .getByRole("heading", { name: `${previewGroup.title} · v1` })
      .scrollIntoViewIfNeeded();
    await page.screenshot({
      path: test.info().outputPath(`group-preview-phone-${width}.png`),
    });
    await page.getByRole("radio", { name: "Máy tính", exact: true }).click();
    await expect(page.locator('[data-preview-viewport="desktop"]')).toBeVisible();
    if (width > 1024) {
      await page.getByRole("button", { name: /^v2/ }).click();
      await expect(
        page.getByRole("heading", { name: `${previewGroup.title} · v2` }),
      ).toBeVisible();
      await expect(
        page.getByRole("heading", { name: `${previewGroup.title} · v1` }),
      ).toHaveCount(0);
    }
  });
}
