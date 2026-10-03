import { expect, test, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";

const requestId = "019535d9-3df7-79fb-b466-fa907fa17f9e";

async function fits(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(() =>
        [
          document.documentElement,
          document.querySelector("[data-scale='deck']"),
          document.querySelector("main"),
        ].every((node) => node === null || node.scrollWidth <= node.clientWidth + 1),
      ),
    )
    .toBe(true);
}

for (const width of [320, 360]) {
  test(`a failed student page fits a phone at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 700 });
    await stubApi(page, {
      ...sessionAs(studentUser),
      "GET /app/classes": { body: { items: [] } },
      "GET /app/assignments": {
        status: 500,
        body: { error: { code: "INTERNAL", message: "boom", requestId } },
      },
    });
    await page.goto("/app");
    await expect(page.getByRole("alert")).toBeVisible({ timeout: 15_000 });
    await fits(page);
    const id = await page.getByText(requestId).boundingBox();
    expect(id!.height).toBeLessThanOrEqual(44);
    const label = await page.getByText("Mã lỗi", { exact: true }).boundingBox();
    expect(label!.height).toBeLessThanOrEqual(24);
  });
}
