import { expect, test } from "@playwright/test";
import { engineFits, startGroupPaper } from "./support/engine";
import { previewGroup, previewQuestions } from "../support/groupPreview";

for (const width of [320, 768, 1440]) {
  test(`shared materials, audio recovery and gap navigation at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const seen = await startGroupPaper(page);
    const material = page.getByRole("heading", {
      name: previewGroup.stimuli[0]!.title,
    });
    const handle = page.getByRole("button", { name: "Mở ngữ liệu", exact: true });
    const card = page.getByRole("group", { name: "Hội thoại mẫu", exact: true });
    const audio = page.locator("audio");
    if (width === 320) {
      await expect(material).not.toBeInViewport();
      await handle.click();
      await expect(material).toBeInViewport();
      await engineFits(page);
    } else {
      await expect(material).toBeVisible();
      await expect(handle).toHaveCount(0);
    }
    await card.scrollIntoViewIfNeeded();
    await expect(card.getByText("Lượt nghe 1/2", { exact: true })).toBeVisible();
    await expect(
      card.getByText("Bấm phát khi bạn đã sẵn sàng.", { exact: true }),
    ).toBeVisible();
    await card.getByRole("button", { name: "Phát", exact: true }).click();
    await expect(
      card.getByRole("button", { name: "Tạm dừng", exact: true }),
    ).toBeVisible();
    await expect(card.getByText("Đang phát", { exact: true })).toBeVisible();
    if (width === 320) {
      await page.getByRole("button", { name: "Đóng", exact: true }).click();
      await expect(material).not.toBeInViewport();
      await expect(handle).toBeFocused();
    }
    await page.getByRole("button", { name: "Câu sau", exact: true }).click();
    await expect(
      page.getByText(previewQuestions[2]!.prompt, { exact: true }),
    ).toBeVisible();
    await expect(audio).toHaveCount(1);
    expect(await audio.evaluate((element: HTMLAudioElement) => element.paused)).toBe(
      false,
    );
    await page.reload();
    if (width === 320) {
      await expect(material).not.toBeInViewport();
      await handle.click();
      await expect(material).toBeInViewport();
    }
    await card.scrollIntoViewIfNeeded();
    await expect(
      card.getByText("Đã nghe xong · còn 1 lượt", { exact: true }),
    ).toBeVisible();
    await expect(
      card.getByRole("button", { name: "Phát lại", exact: true }),
    ).toBeVisible();
    await expect(page.getByText("Lượt nghe đang chờ đồng bộ.")).toHaveCount(0);
    expect(seen.size).toBe(1);
    const gap = page.getByRole("button", { name: "Chỗ trống 2, trống" });
    await gap.focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(":focus")).toHaveAttribute(
      "id",
      `answer-question-${previewQuestions[2]!.id}`,
    );
    await engineFits(page);
    if (width >= 768) {
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
