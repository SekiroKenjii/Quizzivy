import { expect, test } from "@playwright/test";
import { DOCX_TYPE, wordDocument } from "./support/docx";
import { signInAsAdmin } from "./support/live";

/**
 * E2E 10: a teacher takes a Word exam and its answer key from the Imports page
 * through private storage, the worker and recognition to a review, and from
 * the review to a draft test whose questions carry their keys. Live because
 * the claim is about the browser, the API, MinIO and the worker meeting; the
 * stubbed suite cannot say that a real .docx ever reaches a review.
 *
 * The paper is written here, one line per paragraph, and never read from a
 * file: nothing in the repository is a real exam.
 */

const examOf = (title: string) => [
  title,
  "I. Choose the best answer.",
  "1. She ___ in Hanoi since 2019.",
  "A. lives",
  "B. has lived",
  "C. lived",
  "D. will live",
  "2. In which direction does the sun rise?",
  "A. east",
  "B. west",
  "C. north",
  "D. south",
  "II. Complete the sentence with one word.",
  "3. Water freezes at ………… degrees Celsius.",
];

const KEY = ["Answer Key", "1. B", "2. A", "3. zero"];

const WORKER_TIMEOUT = 120_000;

test("E2E 10: a Word exam and its key go from the Imports page to a draft test with its answers", async ({
  page,
}) => {
  test.setTimeout(240_000);
  const title = `E2E 10 ${Date.now()}`;
  await signInAsAdmin(page);

  await page.goto("/teacher/imports/new");
  await page.getByLabel("Tệp đề thi", { exact: true }).setInputFiles({
    name: `${title}.docx`,
    mimeType: DOCX_TYPE,
    buffer: wordDocument(examOf(title)),
  });
  await page.getByLabel("Tệp đáp án", { exact: true }).setInputFiles({
    name: `${title} key.docx`,
    mimeType: DOCX_TYPE,
    buffer: wordDocument(KEY),
  });
  await expect(page.getByLabel("Tên đề", { exact: true })).toHaveValue(title);
  const start = page.getByRole("button", { name: "Bắt đầu xử lý", exact: true });
  await expect(start).toBeEnabled();
  await start.click();

  await expect(page).toHaveURL(/\/teacher\/imports\/[0-9a-f-]+$/, { timeout: 60_000 });
  const review = page.getByRole("link", { name: "Bắt đầu rà soát", exact: true });
  const failed = page.getByText("Xử lý không thành công").first();
  await expect(review.or(failed)).toBeVisible({ timeout: WORKER_TIMEOUT });
  await expect(review, "the worker finished the run with a failure").toBeVisible();
  await review.click();

  await expect(page).toHaveURL(/\/teacher\/imports\/[0-9a-f-]+\/review$/);
  await expect(page.getByLabel("Tên đề", { exact: true })).toHaveValue(title);
  const finish = page.getByRole("button", { name: "Xem trước và hoàn tất" }).first();
  await expect(finish).toBeEnabled();
  await finish.click();

  await expect(page).toHaveURL(/\/teacher\/imports\/[0-9a-f-]+\/confirm$/);
  const create = page.getByRole("button", { name: "Tạo bản nháp đề", exact: true });
  await expect(create).toHaveAttribute("aria-disabled", "false");
  await create.click();
  await expect(page.getByRole("heading", { name: /^Đã tạo bản nháp: / })).toBeVisible({
    timeout: 60_000,
  });
  await page.getByRole("link", { name: "Mở trong trình soạn đề" }).click();

  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);
  await expect(page.getByText("3 câu · 3 điểm")).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true }),
  ).toContainText("She ___ in Hanoi since 2019.");
  await expect(
    page.getByLabel("Đánh dấu B là đáp án đúng", { exact: true }),
  ).toBeChecked();
  await expect(
    page.getByLabel("Đánh dấu A là đáp án đúng", { exact: true }),
  ).not.toBeChecked();

  await page.getByText("In which direction does the sun rise?").click();
  await expect(
    page.getByLabel("Đánh dấu A là đáp án đúng", { exact: true }),
  ).toBeChecked();
  await expect(
    page.getByLabel("Đánh dấu B là đáp án đúng", { exact: true }),
  ).not.toBeChecked();

  await page.getByText("Water freezes at").click();
  await expect(page.getByText("zero", { exact: true })).toBeVisible();
});
