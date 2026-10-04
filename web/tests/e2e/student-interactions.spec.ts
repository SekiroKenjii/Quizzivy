import { expect, test } from "@playwright/test";
import { engineFits, paper, start, type Session } from "./support/engine";

test("rich table blanks preserve frozen answer bindings and reload on a 320px phone", async ({
  page,
}, info) => {
  await page.setViewportSize({ width: 320, height: 850 });
  const data = paper();
  data.questions[1] = {
    id: "q2",
    sectionId: "part",
    type: "fill_blank",
    points: 2,
    prompt: "[2]\t[1]",
    promptContent: {
      format: "semantic_v1",
      blocks: [
        {
          type: "table",
          rows: [
            [
              {
                header: false,
                rowSpan: 1,
                colSpan: 1,
                content: [
                  {
                    type: "paragraph",
                    content: [{ type: "gap", id: "gap-b", label: "2" }],
                  },
                ],
              },
              {
                header: false,
                rowSpan: 1,
                colSpan: 1,
                content: [
                  {
                    type: "paragraph",
                    content: [{ type: "gap", id: "gap-a", label: "1" }],
                  },
                ],
              },
            ],
          ],
        },
      ],
    },
    blanks: [
      { id: "frozen-a", gapId: "gap-a", ordinal: 1, caseSensitive: false },
      { id: "frozen-b", gapId: "gap-b", ordinal: 2, caseSensitive: false },
    ],
  };
  await start(page, data);
  await page.route("**/app/attempts/paper/answers", async (route) => {
    const payload = route.request().postDataJSON() as { answers?: Session["answers"] };
    data.answers = { ...data.answers, ...payload.answers };
    await route.fulfill({
      json: {
        serverTime: data.serverTime,
        savedAt: data.serverTime,
        deadlineAt: data.attempt.deadlineAt,
      },
    });
  });
  await page.getByRole("button", { name: "Câu sau", exact: true }).click();
  const first = page.getByRole("textbox", { name: "Chỗ trống 1", exact: true });
  const second = page.getByRole("textbox", { name: "Chỗ trống 2", exact: true });
  await first.fill("one");
  await second.fill("two");
  await expect
    .poll(() => data.answers["q2"])
    .toEqual({ type: "fill_blank", values: { "frozen-a": "one", "frozen-b": "two" } });
  await expect(page.locator("table input").first()).toHaveValue("two");
  await engineFits(page);
  await page.reload();
  await expect(first).toHaveValue("one");
  await expect(second).toHaveValue("two");
  await page.screenshot({
    path: info.outputPath("rich-blank-learner-320.png"),
    fullPage: true,
  });
});

for (const width of [320, 360, 768, 1024, 1440]) {
  test(`student keyboard and fill-blank at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 850 });
    await start(page);
    await page.locator("label").filter({ hasText: "Đáp án A" }).click();
    await page.keyboard.press("b");
    await expect(page.getByRole("radio", { name: "Đáp án B" })).toBeChecked();
    await page.keyboard.press("f");
    await page.keyboard.press("ArrowRight");
    const blank = page.getByRole("textbox", { name: "Chỗ trống 1" });
    await blank.pressSequentially("went");
    await expect(blank).toBeFocused();
    await expect(blank).toHaveValue("went");
    await engineFits(page);
    await page.screenshot({
      path: info.outputPath(`fill-blank-${width}.png`),
      fullPage: true,
    });
    await blank.press("ArrowLeft");
    await blank.press("X");
    await expect(blank).toHaveValue("wenXt");
    await page.getByRole("button", { name: "Câu sau", exact: true }).click();
    await page
      .getByRole("textbox", { name: "Bài làm của bạn" })
      .pressSequentially("A full sentence.");
    await engineFits(page);
    const finish = page.getByRole("button", { name: "Hoàn tất", exact: true });
    const bounds = await finish.boundingBox();
    expect(bounds).not.toBeNull();
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width);
    await finish.click();
    const dialog = page.getByRole("dialog", { name: /^Nộp bài/ });
    await expect(dialog).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Nộp bài", exact: true }),
    ).toBeInViewport();
    await page.keyboard.press("ArrowLeft");
    await expect(dialog).toBeVisible();
  });
}

test("a long paper keeps the Submit dialog's actions reachable on a 320px phone", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 700 });
  const data = paper();
  data.questions = Array.from({ length: 80 }, (_, index) => ({
    ...data.questions[0]!,
    id: `question-${index}`,
  }));
  await start(page, data);
  await page
    .getByRole("banner")
    .getByRole("button", { name: "Nộp bài", exact: true })
    .click();
  const dialog = page.getByRole("dialog", { name: /^Nộp bài/ });
  await expect(dialog).toContainText("Nộp bài khi còn 80 câu chưa trả lời?");
  await expect(
    dialog.getByRole("button", { name: "Nộp bài", exact: true }),
  ).toBeInViewport();
  await expect(
    dialog.getByRole("button", { name: "Quay lại làm tiếp", exact: true }),
  ).toBeInViewport();
});

for (const [width, floored] of [
  [1023, true],
  [1024, false],
] as const) {
  test(`Return to fullscreen ${floored ? "keeps" : "is past"} the 44px floor at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 800 });
    const data = paper();
    data.integrity.requireFullscreen = true;
    await start(page, data);
    const box = (await page
      .getByRole("button", { name: "Quay lại toàn màn hình", exact: true })
      .boundingBox())!;
    if (floored) expect(box.height).toBeGreaterThanOrEqual(44);
    else expect(box.height).toBeLessThan(44);
  });
}

test("a reload opens the question that was open", async ({ page }) => {
  await start(page);
  const next = page.getByRole("button", { name: "Câu sau", exact: true });
  await next.click();
  await next.click();
  const third = page.getByText("Viết một câu", { exact: true });
  await expect(third).toBeVisible();
  await page.reload();
  await expect(third).toBeVisible();
  await expect(page.getByText("Chọn đáp án", { exact: true })).toBeHidden();
});
