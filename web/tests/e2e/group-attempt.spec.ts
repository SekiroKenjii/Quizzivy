import { expect, test } from "@playwright/test";
import { startGroupPaper } from "./support/engine";
import { previewGroup, previewQuestions } from "../support/groupPreview";

for (const width of [320, 1440]) {
  test(`shared materials, audio recovery and gap navigation at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const seen = await startGroupPaper(page);
    const material = page.getByRole("heading", { name: previewGroup.title });
    const passageTab = page.getByRole("button", { name: "Ngữ liệu", exact: true });
    if (width === 320) {
      await expect(material).toBeHidden();
      await passageTab.click();
      await expect(material).toBeVisible();
      await page.getByRole("button", { name: "Câu 1", exact: true }).click();
      await expect(material).toBeHidden();
    } else {
      await expect(material).toBeVisible();
      await expect(passageTab).toHaveCount(0);
    }
    await expect(page.getByText("Còn 2 lượt nghe")).toBeVisible();
    await page.getByRole("button", { name: "Phát", exact: true }).click();
    await expect(page.getByText("Còn 1 lượt nghe")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Tạm dừng", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Câu sau", exact: true }).click();
    await expect(
      page.getByText(previewQuestions[2]!.prompt, { exact: true }),
    ).toBeVisible();
    await expect(page.locator("audio")).toHaveCount(1);
    await expect(
      page.getByRole("button", { name: "Tạm dừng", exact: true }),
    ).toBeVisible();
    await page.reload();
    await expect(page.getByText("Còn 1 lượt nghe")).toBeVisible();
    await expect(page.getByText("Lượt nghe đang chờ đồng bộ.")).toHaveCount(0);
    expect(seen.size).toBe(1);
    if (width === 320) {
      await expect(page.getByText("Lịch hoạt động", { exact: true })).toBeHidden();
      await expect(
        page.getByRole("button", { name: "Phát", exact: true }),
      ).toBeVisible();
      await passageTab.click();
      await expect(page.getByText("Lịch hoạt động", { exact: true })).toBeVisible();
    }
    const gap = page.getByRole("button", { name: "Ô A — chuyển đến câu 2" });
    await gap.focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(":focus")).toHaveAttribute(
      "id",
      `answer-question-${previewQuestions[2]!.id}`,
    );
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1,
    );
    expect(overflow).toBe(false);
    if (width === 1440) {
      const contextBox = await page
        .getByLabel(previewGroup.title, { exact: true })
        .boundingBox();
      const questionBox = await page
        .locator(`#answer-question-${previewQuestions[2]!.id}`)
        .boundingBox();
      expect(contextBox!.x + contextBox!.width).toBeLessThanOrEqual(questionBox!.x);
    }
    await page.screenshot({
      path: test.info().outputPath(`group-attempt-${width}.png`),
    });
  });
}
