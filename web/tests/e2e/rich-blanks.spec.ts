import { expect, test } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import type { components } from "../../src/lib/api/schema";

const ID = "018f0000-0000-7000-8000-0000000000b1";

test("rich blanks retain answers through conversion, table editing, undo, save and reload on a tablet", async ({
  page,
}) => {
  let question: components["schemas"]["AdminQuestion"] = {
    id: ID,
    type: "fill_blank",
    prompt: "**They** {{2}}, she {{1}}.",
    points: 2,
    tags: [],
    blanks: [1, 2].map((ordinal) => ({
      id: `018f0000-0000-7000-8000-0000000000e${ordinal}`,
      ordinal,
      acceptedAnswers: [ordinal === 1 ? "one" : "two"],
      caseSensitive: false,
    })),
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
  let writes = 0;
  await stubApi(page, {
    ...sessionAs(adminUser),
    [`GET /admin/questions/${ID}`]: (route) => route.fulfill({ json: question }),
    [`PATCH /admin/questions/${ID}`]: async (route) => {
      const body = route
        .request()
        .postDataJSON() as components["schemas"]["QuestionInput"];
      question = {
        ...question,
        prompt: body.prompt,
        promptContent: body.promptContent ?? null,
        blanks: (body.blanks ?? []).map((blank) => ({
          ...blank,
          id: blank.id ?? `018f0000-0000-7000-8000-0000000000e${blank.ordinal}`,
        })),
      };
      writes++;
      await route.fulfill({ json: question });
    },
  });
  await page.setViewportSize({ width: 768, height: 900 });
  await page.goto(`/admin/question-bank/${ID}`);
  await page
    .getByRole("button", { name: "Định dạng: Nội dung câu hỏi", exact: true })
    .click();
  expect(writes).toBe(0);
  await page.getByRole("button", { name: "Dùng nội dung này", exact: true }).click();
  const prompt = page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true });
  await expect(prompt.locator(".content-gap")).toHaveText(["2", "1"]);
  await prompt.locator("p").first().click();
  await page.keyboard.press("Control+End");
  await page.keyboard.press("Enter");
  await expect(prompt.locator(":scope > p")).toHaveCount(2);
  await page.getByRole("button", { name: "Thêm bảng", exact: true }).click();
  await prompt.locator("th").first().click();
  await page.getByRole("button", { name: "Thêm ô trống", exact: true }).click();
  await expect(prompt.locator("table .content-gap")).toHaveText("3");
  await expect(prompt.locator(".content-gap")).toHaveText(["2", "1", "3"]);
  await page
    .getByRole("textbox", {
      name: "Đáp án được chấp nhận cho chỗ trống 3",
      exact: true,
    })
    .fill("three");
  await prompt.locator("th").first().click();
  await page.keyboard.press("Home");
  await page.keyboard.press("Shift+ArrowRight");
  await page.keyboard.press("Backspace");
  await expect(
    page.getByText("Ô này đã được bỏ khỏi nội dung.", { exact: false }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Hoàn tác", exact: true }).click();
  await expect(prompt.locator("table .content-gap")).toHaveText("3");
  await expect(
    page.getByRole("textbox", {
      name: "Đáp án được chấp nhận cho chỗ trống 3",
      exact: true,
    }),
  ).toHaveValue("three");
  await page.getByRole("button", { name: "Xong", exact: true }).click();
  await page.getByRole("button", { name: "Lưu", exact: true }).click();
  await expect.poll(() => writes).toBe(1);
  expect(question.blanks?.map((blank) => blank.acceptedAnswers)).toEqual([
    ["one"],
    ["two"],
    ["three"],
  ]);
  const gaps = question.blanks?.map((blank) => blank.gapId);
  expect(new Set(gaps).size).toBe(3);
  await page.reload();
  await page
    .getByRole("button", { name: "Chỉnh sửa: Nội dung câu hỏi", exact: true })
    .click();
  await expect(prompt.locator(".content-gap")).toHaveText(["2", "1", "3"]);
  await expect(
    page.getByRole("textbox", {
      name: "Đáp án được chấp nhận cho chỗ trống 1",
      exact: true,
    }),
  ).toHaveValue("one");
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
  expect(question.blanks?.map((blank) => blank.gapId)).toEqual(gaps);
  await page.screenshot({
    path: test.info().outputPath("rich-blanks-tablet.png"),
    fullPage: true,
  });
});
