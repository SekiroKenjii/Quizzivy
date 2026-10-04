import { expect, test, type Page } from "@playwright/test";
import {
  engineFits,
  paper,
  start,
  startGroupPaper,
  type Session,
} from "./support/engine";
import { previewGroup, previewQuestions } from "../support/groupPreview";

function eightQuestions(): Session {
  const data = paper();
  const first = data.questions[0]!;
  data.questions = Array.from({ length: 8 }, (_, index) => ({
    ...first,
    id: `q${index + 1}`,
  }));
  data.answers = Object.fromEntries(
    ["q1", "q2", "q3"].map((id) => [id, { type: "choice", optionIds: ["a"] }]),
  );
  return data;
}

const footer = (page: Page) => page.getByRole("navigation", { name: "Danh sách câu" });
const countButton = (page: Page) =>
  footer(page).getByRole("button", { name: /^Danh sách câu:/ });
const sheet = (page: Page) => page.getByRole("dialog", { name: "Danh sách câu" });
const questionLine = (page: Page, number: number) =>
  page.getByText(`Câu ${number} trên 8 · Chọn một đáp án`, { exact: true });

async function footerHeights(page: Page) {
  return footer(page)
    .getByRole("button")
    .evaluateAll((controls) =>
      controls.map((control) => control.getBoundingClientRect().height),
    );
}

test("the count button says where the student is, and opens the sheet of every question", async ({
  page,
}) => {
  await start(page, eightQuestions());
  const next = footer(page).getByRole("button", { name: "Câu sau", exact: true });
  for (let step = 0; step < 3; step += 1) await next.tap();
  await expect(questionLine(page, 4)).toBeVisible();
  await expect(countButton(page)).toHaveAccessibleName(
    "Danh sách câu: 4 / 8 · đã trả lời 3",
  );
  await expect(countButton(page)).toContainText("4 / 8 · đã trả lời 3");
  await expect(footer(page).getByRole("button")).toHaveCount(3);

  await expect(sheet(page)).toHaveCount(0);
  await countButton(page).tap();
  await expect(sheet(page)).toBeVisible();
  await expect(sheet(page)).toContainText("Đã trả lời 3 trên 8");
  await expect(sheet(page).getByRole("button")).toHaveCount(8);
  await expect(sheet(page).locator("[aria-current]")).toHaveCount(1);
  await expect(
    sheet(page).getByRole("button", { name: "Câu 4, đang xem", exact: true }),
  ).toHaveAttribute("aria-current", "true");
  await expect(
    sheet(page).getByRole("button", { name: "Câu 3, đã trả lời", exact: true }),
  ).toBeVisible();
});

test("a square in the sheet goes to its question and closes the sheet", async ({
  page,
}) => {
  await start(page, eightQuestions());
  await countButton(page).tap();
  await sheet(page).getByRole("button", { name: "Câu 7", exact: true }).tap();
  await expect(questionLine(page, 7)).toBeVisible();
  await expect(sheet(page)).toHaveCount(0);
  await expect(countButton(page)).toContainText("7 / 8 · đã trả lời 3");
});

test("the sheet closes on its backdrop and leaves the question where it was", async ({
  page,
}) => {
  await start(page, eightQuestions());
  await countButton(page).tap();
  await expect(sheet(page)).toBeVisible();
  await page
    .locator("[data-slot='dialog-overlay']")
    .tap({ position: { x: 24, y: 24 } });
  await expect(sheet(page)).toHaveCount(0);
  await expect(questionLine(page, 1)).toBeVisible();
});

test("Finish on the last question opens the Submit dialog", async ({ page }) => {
  await start(page, eightQuestions());
  const finish = footer(page).getByRole("button", { name: "Hoàn tất", exact: true });
  const submit = page.getByRole("dialog", { name: /^Nộp bài/ });
  await expect(finish).toHaveCount(0);
  await countButton(page).tap();
  await sheet(page).getByRole("button", { name: "Câu 8", exact: true }).tap();
  await expect(countButton(page)).toContainText("8 / 8 · đã trả lời 3");
  await expect(
    footer(page).getByRole("button", { name: "Câu sau", exact: true }),
  ).toHaveCount(0);
  await expect(submit).toHaveCount(0);
  await finish.tap();
  await expect(submit).toBeVisible();
  await expect(submit).toContainText("Nộp bài khi còn 5 câu chưa trả lời?");
});

test("a passage and its question take turns on the screen, and the answer is kept", async ({
  page,
}) => {
  await startGroupPaper(page);
  const first = previewQuestions[1]!;
  const switcher = page.getByRole("group", { name: "Xem ngữ liệu hoặc câu hỏi" });
  const passageTab = switcher.getByRole("button", { name: "Ngữ liệu", exact: true });
  const questionTab = switcher.getByRole("button", { name: "Câu 1", exact: true });
  const passage = page.getByRole("heading", { name: previewGroup.title });
  const prompt = page.getByText(first.prompt, { exact: true });
  const option = page.getByRole("radio", { name: first.options![0]!.text });

  await expect(switcher.getByRole("button")).toHaveCount(2);
  await expect(questionTab).toHaveAttribute("aria-pressed", "true");
  await expect(passageTab).toHaveAttribute("aria-pressed", "false");
  await expect(prompt).toBeVisible();
  await expect(passage).toBeHidden();
  await expect(footer(page)).toBeVisible();

  await page.locator("label").filter({ hasText: first.options![0]!.text }).tap();
  await expect(option).toBeChecked();

  await passageTab.tap();
  await expect(passageTab).toHaveAttribute("aria-pressed", "true");
  await expect(questionTab).toHaveAttribute("aria-pressed", "false");
  await expect(passage).toBeVisible();
  await expect(prompt).toBeHidden();
  await expect(footer(page)).toBeHidden();
  await engineFits(page);

  await questionTab.tap();
  await expect(questionTab).toHaveAttribute("aria-pressed", "true");
  await expect(prompt).toBeVisible();
  await expect(passage).toBeHidden();
  await expect(footer(page)).toBeVisible();
  await expect(option).toBeChecked();
  await engineFits(page);
});

test("the engine does not scroll sideways, with the sheet open or closed", async ({
  page,
}) => {
  await start(page, eightQuestions());
  await engineFits(page);
  await countButton(page).tap();
  await expect(sheet(page)).toBeVisible();
  await engineFits(page);
});

test("every control in the engine's footer is at least 44px tall", async ({ page }) => {
  await start(page, eightQuestions());
  const stepping = await footerHeights(page);
  expect(stepping).toHaveLength(3);
  for (const height of stepping) expect(height).toBeGreaterThanOrEqual(44);

  await countButton(page).tap();
  await sheet(page).getByRole("button", { name: "Câu 8", exact: true }).tap();
  await expect(
    footer(page).getByRole("button", { name: "Hoàn tất", exact: true }),
  ).toBeVisible();
  const finishing = await footerHeights(page);
  expect(finishing).toHaveLength(3);
  for (const height of finishing) expect(height).toBeGreaterThanOrEqual(44);
});

test("the recording's play button and Return to fullscreen keep the 44px floor", async ({
  page,
}) => {
  await startGroupPaper(page);
  const play = (await page
    .getByRole("button", { name: "Phát", exact: true })
    .boundingBox())!;
  expect(play.height).toBeGreaterThanOrEqual(44);
  expect(play.width).toBeGreaterThanOrEqual(44);

  const data = eightQuestions();
  data.integrity.requireFullscreen = true;
  await start(page, data);
  const back = (await page
    .getByRole("button", { name: "Quay lại toàn màn hình", exact: true })
    .boundingBox())!;
  expect(back.height).toBeGreaterThanOrEqual(44);
  expect(back.width).toBeGreaterThanOrEqual(44);
});
