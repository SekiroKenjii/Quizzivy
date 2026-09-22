import { expect, test } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import type { components } from "../../src/lib/api/schema";

const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";

test("builder flushes rich edits before switching questions and previews the saved marks", async ({
  page,
}) => {
  const testID = "018f0000-0000-7000-8000-0000000000a1";
  const secondID = "018f0000-0000-7000-8000-0000000000b2";
  let question: components["schemas"]["AdminQuestion"] = {
    id: QUESTION_ID,
    type: "single_choice",
    prompt: "First question",
    points: 1,
    tags: [],
    options: [
      {
        id: "018f0000-0000-7000-8000-0000000000d1",
        ordinal: 0,
        text: "think",
        isCorrect: true,
        content: {
          format: "semantic_v1",
          blocks: [
            {
              type: "paragraph",
              content: [{ type: "text", text: "think", marks: [] }],
            },
          ],
        },
      },
      {
        id: "018f0000-0000-7000-8000-0000000000d2",
        ordinal: 1,
        text: "other",
        isCorrect: false,
      },
    ],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
  let finishSave: () => void = () => undefined;
  const gate = new Promise<void>((resolve) => {
    finishSave = resolve;
  });
  let received = false;
  await stubApi(page, {
    ...sessionAs(adminUser),
    [`GET /admin/tests/${testID}`]: {
      body: {
        id: testID,
        title: "Rich builder",
        status: "draft",
        currentVersion: 0,
        totalPoints: 2,
        questionCount: 2,
        audioCount: 0,
        sections: [
          {
            id: "018f0000-0000-7000-8000-0000000000c1",
            ordinal: 0,
            title: "Part 1",
            instructions: null,
            questionIds: [QUESTION_ID, secondID],
          },
        ],
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
    },
    [`GET /admin/questions/${QUESTION_ID}`]: (route) =>
      route.fulfill({ json: question }),
    [`GET /admin/questions/${secondID}`]: (route) =>
      route.fulfill({ json: { ...question, id: secondID, prompt: "Second question" } }),
    "GET /admin/questions": {
      body: {
        items: [],
        tags: [],
        page: 1,
        pageSize: 20,
        total: 0,
        bankTotal: 0,
        facets: {
          all: 0,
          single_choice: 0,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: 0,
        },
      },
    },
    [`PATCH /admin/questions/${QUESTION_ID}`]: async (route) => {
      const body = route
        .request()
        .postDataJSON() as components["schemas"]["QuestionInput"];
      question = {
        ...question,
        options: body.options!.map((option, ordinal) => ({
          ...option,
          ordinal,
          id: question.options![ordinal]!.id,
        })),
      };
      received = true;
      await gate;
      await route.fulfill({ json: question });
    },
  });
  await page.goto(`/admin/tests/${testID}/edit`);
  await page.getByRole("button", { name: "Sửa định dạng phương án 1" }).click();
  const editor = page.getByRole("textbox", { name: "Lựa chọn 1", exact: true });
  await editor.click();
  await page.keyboard.press("Control+a");
  await page.getByRole("button", { name: "Gạch chân", exact: true }).click();
  await expect(editor.locator("u")).toHaveText("think");
  await page.getByRole("button", { name: "Second question", exact: true }).click();
  await expect.poll(() => received).toBe(true);
  await expect(
    page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true }),
  ).toHaveValue("First question");
  finishSave();
  await expect(
    page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true }),
  ).toHaveValue("Second question");
  await page.getByRole("button", { name: "First question", exact: true }).click();
  await page.getByRole("button", { name: "Sửa định dạng phương án 1" }).click();
  await expect(editor.locator("u")).toHaveText("think");
  await page.getByRole("button", { name: "Xong", exact: true }).click();
  await page.getByRole("button", { name: "Xem như học viên", exact: true }).click();
  await expect(page.getByRole("dialog").locator("u").first()).toHaveText("think");
});

test("bank formatting survives save and reload without changing the answer key", async ({
  page,
}) => {
  let question: components["schemas"]["AdminQuestion"] = {
    id: QUESTION_ID,
    type: "single_choice",
    prompt: "Choose the underlined sound",
    points: 1,
    tags: [],
    options: [
      {
        id: "018f0000-0000-7000-8000-0000000000d1",
        ordinal: 0,
        text: "think",
        isCorrect: true,
        content: {
          format: "semantic_v1",
          blocks: [
            {
              type: "paragraph",
              content: [
                { type: "text", text: "th", marks: ["underline"] },
                { type: "text", text: "ink", marks: [] },
              ],
            },
          ],
        },
      },
      {
        id: "018f0000-0000-7000-8000-0000000000d2",
        ordinal: 1,
        text: "other",
        isCorrect: false,
      },
    ],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
  let writes = 0;
  await stubApi(page, {
    ...sessionAs(adminUser),
    [`GET /admin/questions/${QUESTION_ID}`]: (route) =>
      route.fulfill({ json: question }),
    "GET /admin/questions": {
      body: {
        items: [],
        tags: [],
        page: 1,
        pageSize: 20,
        total: 0,
        bankTotal: 0,
        facets: {
          all: 0,
          single_choice: 0,
          multiple_choice: 0,
          true_false: 0,
          fill_blank: 0,
          short_answer: 0,
        },
      },
    },
    [`PATCH /admin/questions/${QUESTION_ID}`]: async (route) => {
      const body = route
        .request()
        .postDataJSON() as components["schemas"]["QuestionInput"];
      question = {
        ...question,
        prompt: body.prompt,
        options: body.options!.map((option, ordinal) => ({
          ...option,
          id: question.options![ordinal]!.id,
          ordinal,
        })),
      };
      writes++;
      await route.fulfill({ json: question });
    },
  });
  await page.setViewportSize({ width: 768, height: 900 });
  await page.goto(`/admin/question-bank/${QUESTION_ID}`);
  await page.getByRole("button", { name: "Sửa định dạng phương án 1" }).click();
  const editor = page.getByRole("textbox", { name: "Lựa chọn 1", exact: true });
  await expect(editor.locator("u")).toHaveText("th");
  await editor.click();
  await page.keyboard.press("Control+End");
  await expect(
    page.getByRole("button", { name: "Gạch chân", exact: true }),
  ).toHaveAttribute("aria-pressed", "false");
  await page.keyboard.insertText(" mới");
  await expect(editor).toHaveText("think mới");
  await page.getByRole("button", { name: "Hoàn tác", exact: true }).click();
  await expect(editor).toHaveText("think");
  await page.getByRole("button", { name: "Làm lại", exact: true }).click();
  await expect(editor).toHaveText("think mới");
  await expect(
    page.getByRole("button", { name: "Chèn bảng", exact: true }),
  ).toHaveCount(0);
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
  await page.screenshot({
    path: test.info().outputPath("rich-option-768.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Xong", exact: true }).click();
  await page.getByRole("button", { name: "Định dạng phương án 2" }).click();
  const second = page.getByRole("textbox", { name: "Lựa chọn 2", exact: true });
  await second.click();
  await page.keyboard.press("Control+a");
  await second.evaluate((element) => {
    const data = new DataTransfer();
    data.setData("text/plain", "Tiếng\nViệt");
    element.dispatchEvent(
      new ClipboardEvent("paste", {
        clipboardData: data,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await expect(second.locator("p")).toHaveCount(1);
  await expect(second.locator("br")).toHaveCount(1);
  await expect(second).toHaveText("TiếngViệt");
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("mới");
  await expect(second.locator("p")).toHaveCount(1);
  await expect(second.locator("br")).toHaveCount(2);
  await page.getByRole("button", { name: "Xong", exact: true }).click();
  await page.getByRole("button", { name: "Lưu", exact: true }).click();
  await expect.poll(() => writes).toBe(1);
  expect(question.options![0]!.isCorrect).toBe(true);
  expect(question.options![1]!.isCorrect).toBe(false);
  expect(question.options![1]!.text).toBe("Tiếng\nViệt\nmới");
  await page.reload();
  await page.getByRole("button", { name: "Sửa định dạng phương án 1" }).click();
  await expect(editor).toHaveText("think mới");
  await expect(editor.locator("u")).toHaveText("th");
});
