import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import type { components } from "../../src/lib/api/schema";

const ID = "018f0000-0000-7000-8000-0000000000b1";
const SECOND = "018f0000-0000-7000-8000-0000000000b2";
const TEST = "018f0000-0000-7000-8000-0000000000a1";
type Question = components["schemas"]["AdminQuestion"];
type QuestionInput = components["schemas"]["QuestionInput"];

function sample(): Question {
  return {
    id: ID,
    type: "short_answer",
    prompt: "**Đọc kỹ**\n\nDòng thứ hai.",
    explanation: "*Giải thích* bằng tiếng Việt.",
    points: 1,
    tags: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

async function setup(page: Page, initial = sample(), delayed = false) {
  let question = initial;
  let writes = 0;
  let release = () => undefined as void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await stubApi(page, {
    ...sessionAs(adminUser),
    [`GET /admin/questions/${ID}`]: (route) => route.fulfill({ json: question }),
    [`GET /admin/questions/${SECOND}`]: (route) =>
      route.fulfill({ json: { ...sample(), id: SECOND, prompt: "Câu thứ hai" } }),
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
    [`PATCH /admin/questions/${ID}`]: async (route) => {
      const body = route.request().postDataJSON() as QuestionInput;
      question = {
        ...question,
        prompt: body.prompt,
        promptContent: body.promptContent ?? null,
        explanation: body.explanation ?? null,
        explanationContent: body.explanationContent ?? null,
      };
      writes++;
      if (delayed) await gate;
      await route.fulfill({ json: question });
    },
    [`GET /admin/tests/${TEST}`]: {
      body: {
        id: TEST,
        title: "Soạn nội dung",
        status: "draft",
        currentVersion: 0,
        totalPoints: 2,
        questionCount: 2,
        audioCount: 0,
        sections: [
          {
            id: "018f0000-0000-7000-8000-0000000000c1",
            ordinal: 0,
            title: "Phần 1",
            instructions: null,
            questionIds: [ID, SECOND],
          },
        ],
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
    },
  });
  return {
    getQuestion: () => question,
    writes: () => writes,
    release: () => release(),
  };
}

test("bank saves a confirmed structured paste and retains formatting after reload", async ({
  page,
}) => {
  const state = await setup(page);
  await page.goto(`/admin/question-bank/${ID}`);
  await page
    .getByRole("button", { name: "Định dạng: Nội dung câu hỏi", exact: true })
    .click();
  await page.getByRole("button", { name: "Dùng nội dung này", exact: true }).click();
  const editor = page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true });
  await editor.click();
  await page.keyboard.press("Control+a");
  await editor.evaluate((element) => {
    const data = new DataTransfer();
    data.setData(
      "text/html",
      "<p><u>Nội dung dán</u></p><table><tr><th>Cột A</th><th>Cột B</th></tr><tr><td>một</td><td>hai</td></tr></table>",
    );
    element.dispatchEvent(
      new ClipboardEvent("paste", {
        clipboardData: data,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Áp dụng nội dung dán" })
    .click();
  await expect(editor.locator("u")).toHaveText("Nội dung dán");
  expect(state.writes()).toBe(0);
  await page.getByRole("button", { name: "Xong", exact: true }).click();
  await page.getByRole("button", { name: "Lưu", exact: true }).click();
  await expect.poll(state.writes).toBe(1);
  expect(state.getQuestion().prompt).toBe("Nội dung dán\n\nCột A\tCột B\nmột\thai");
  expect(state.getQuestion().promptContent?.blocks[1]?.type).toBe("table");
  await page.reload();
  await expect(page.locator("u")).toHaveText("Nội dung dán");
  await expect(page.getByRole("cell", { name: "hai", exact: true })).toBeVisible();
});

test("bank previews Markdown conversion, preserves tables and explanations after saving and reload", async ({
  page,
}) => {
  const state = await setup(page);
  await page.setViewportSize({ width: 768, height: 900 });
  await page.goto(`/admin/question-bank/${ID}`);
  await page
    .getByRole("button", { name: "Định dạng: Nội dung câu hỏi", exact: true })
    .click();
  await expect(
    page.getByText("Kiểm tra bản chuyển đổi bên dưới trước khi áp dụng.", {
      exact: false,
    }),
  ).toBeVisible();
  expect(state.writes()).toBe(0);
  await page.getByRole("button", { name: "Dùng nội dung này", exact: true }).click();
  const prompt = page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true });
  await expect(prompt.locator("strong")).toHaveText("Đọc kỹ");
  await expect(
    page.getByRole("button", { name: "Thêm ô trống", exact: true }),
  ).toHaveCount(0);
  await prompt.click();
  await page.keyboard.press("Control+End");
  await page.keyboard.press("Enter");
  await page.getByRole("button", { name: "Thêm bảng", exact: true }).click();
  await expect(prompt.locator("table")).toHaveCount(1);
  await prompt.locator("th").first().click();
  await page.keyboard.insertText("Mục");
  await page.getByRole("button", { name: "Hoàn tác", exact: true }).click();
  await expect(prompt).not.toContainText("Mục");
  await page.getByRole("button", { name: "Làm lại", exact: true }).click();
  await expect(prompt.locator("th").first()).toContainText("Mục");
  await page.getByRole("button", { name: "Xong", exact: true }).click();
  await page
    .getByRole("button", { name: "Định dạng: Giải thích", exact: true })
    .click();
  await page.getByRole("button", { name: "Dùng nội dung này", exact: true }).click();
  const explanation = page.getByRole("textbox", { name: "Giải thích", exact: true });
  await expect(explanation.locator("em")).toHaveText("Giải thích");
  await explanation.click();
  await page.keyboard.press("Control+End");
  await page.keyboard.insertText(" Đã kiểm tra.");
  await page.getByRole("button", { name: "Xong", exact: true }).click();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
  await page.getByRole("button", { name: "Lưu", exact: true }).click();
  await expect.poll(state.writes).toBe(1);
  expect(state.getQuestion().promptContent?.format).toBe("semantic_v1");
  expect(state.getQuestion().explanation).toContain("Đã kiểm tra.");
  await page.reload();
  await page
    .getByRole("button", { name: "Chỉnh sửa: Nội dung câu hỏi", exact: true })
    .click();
  await expect(prompt.locator("table")).toHaveCount(1);
  await expect(prompt.locator("th").first()).toContainText("Mục");
  await page.screenshot({
    path: test.info().outputPath("question-prose-768.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 1024, height: 850 });
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
  await page.screenshot({
    path: test.info().outputPath("question-prose-1024.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Chuyển về Markdown", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Văn bản vẫn được giữ");
  await page.getByRole("button", { name: "Huỷ", exact: true }).click();
  await expect(prompt.locator("table")).toHaveCount(1);
  expect(state.writes()).toBe(1);
  await page.getByRole("button", { name: "Chuyển về Markdown", exact: true }).click();
  await page
    .getByRole("button", { name: "Bỏ định dạng và chuyển đổi", exact: true })
    .click();
  const markdown = prompt.and(page.locator("textarea"));
  await expect(markdown).toHaveValue(/Đọc kỹ/);
  expect(state.writes()).toBe(1);
  await page.getByRole("button", { name: "Lưu", exact: true }).click();
  await expect.poll(state.writes).toBe(2);
  expect(state.getQuestion().promptContent).toBeNull();
  expect(state.getQuestion().explanationContent?.format).toBe("semantic_v1");
  await page.reload();
  await expect(markdown).toHaveValue(/Mục/);
});

test("unsupported Markdown conversion leaves the original editable and never writes", async ({
  page,
}) => {
  const original = "Keep `code` and ![picture](https://example.com/p.png)";
  const state = await setup(page, { ...sample(), prompt: original });
  await page.goto(`/admin/question-bank/${ID}`);
  await page
    .getByRole("button", { name: "Định dạng: Nội dung câu hỏi", exact: true })
    .click();
  await expect(page.getByRole("alert")).toContainText("Bản gốc được giữ nguyên");
  await page.getByRole("button", { name: "Giữ Markdown", exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true }),
  ).toHaveValue(original);
  expect(state.writes()).toBe(0);
});

test("builder flushes rich prose before switching and preview retains the saved text", async ({
  page,
}) => {
  const content: components["schemas"]["QuestionContent"] = {
    format: "semantic_v1",
    blocks: [
      {
        type: "paragraph",
        content: [{ type: "text", text: "Câu thứ nhất", marks: ["underline"] }],
      },
    ],
  };
  const state = await setup(
    page,
    { ...sample(), prompt: "Câu thứ nhất", promptContent: content },
    true,
  );
  await page.goto(`/admin/tests/${TEST}/edit`);
  await page
    .getByRole("button", { name: "Chỉnh sửa: Nội dung câu hỏi", exact: true })
    .click();
  const prompt = page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true });
  await prompt.click();
  await page.keyboard.press("Control+End");
  await page.keyboard.insertText(" đã sửa");
  await page.getByRole("button", { name: /Câu thứ hai/ }).click();
  await expect.poll(state.writes).toBe(1);
  await expect(prompt).toContainText("đã sửa");
  state.release();
  await expect(
    page
      .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
      .and(page.locator("textarea")),
  ).toHaveValue("Câu thứ hai");
  await page.getByRole("button", { name: /Câu thứ nhất đã sửa/ }).click();
  await page
    .getByRole("button", { name: "Chỉnh sửa: Nội dung câu hỏi", exact: true })
    .click();
  await expect(prompt).toContainText("đã sửa");
  await page.getByRole("button", { name: "Xem như học viên", exact: true }).click();
  await expect(page.getByRole("dialog").locator("u")).toContainText("Câu thứ nhất");
});
