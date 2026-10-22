import { expect, test, type Page } from "@playwright/test";
import {
  ASSIGNMENT,
  chooseOption,
  freshAttempt,
  goToQuestion,
  gradeAttempt,
  signInAsAdmin,
  signInAsStudent,
  submitAttempt,
} from "./support/live";

/**
 * E2E 11 (§14, T-R6.14): the paper with a question of every type
 * (seed/10-dev-question-types.sql) from start to result. A student answers
 * each type, a "Choose two" replacing its oldest pick and a short answer over
 * its word limit, and submits; the auto-marks follow §7 (T-R6.6); a teacher
 * marks the short answer and grades the essay by its four criteria; the
 * result shows the band, the criteria, the part tiles, the correct matches and
 * the explanation's image. Live, because marking and the student projection
 * are the server's behaviour meeting the engine's.
 */

async function answerEveryType(page: Page) {
  // 1 · True / False / Not given, labelled in Vietnamese by position: the key.
  await chooseOption(page, "Sai");

  // 2 · matching: A → ii and C → i are right, B → iii is wrong.
  await goToQuestion(page, 2);
  const card = page.getByRole("group", { name: "Câu 2", exact: true });
  await card
    .getByRole("radiogroup", { name: "A Paragraph A" })
    .getByRole("radio", { name: "ii", exact: true })
    .click();
  await card
    .getByRole("radiogroup", { name: "B Paragraph B" })
    .getByRole("radio", { name: "iii", exact: true })
    .click();
  await card
    .getByRole("radiogroup", { name: "C Paragraph C" })
    .getByRole("radio", { name: "i", exact: true })
    .click();
  await expect(card.getByText("Đã nối 3/3")).toBeVisible();

  // 3 · true / false: wrong.
  await goToQuestion(page, 3);
  await chooseOption(page, "Sai");

  // 4 · single choice under a prompt image: right.
  await goToQuestion(page, 4);
  await expect(
    page
      .getByRole("group", { name: "Câu 4", exact: true })
      .getByText("A shop sign that reads: Closed on Sundays"),
  ).toBeVisible();
  await chooseOption(page, "On Sundays");

  // 5 · "Choose two": the third pick replaces the oldest, so harvest (right)
  // goes and delta (wrong) and flood (right) stay.
  await goToQuestion(page, 5);
  const picks = page.getByRole("group", { name: "Câu 5", exact: true });
  await expect(picks.getByText("Chọn hai đáp án.")).toBeVisible();
  await picks.locator("label").filter({ hasText: "harvest" }).click();
  await picks.locator("label").filter({ hasText: "delta" }).click();
  await picks.locator("label").filter({ hasText: "flood" }).click();
  await expect(picks.getByRole("checkbox", { name: "harvest" })).not.toBeChecked();
  await expect(picks.getByRole("checkbox", { name: "delta" })).toBeChecked();
  await expect(picks.getByRole("checkbox", { name: "flood" })).toBeChecked();

  // 6 · fill in the blank: one gap of two.
  await goToQuestion(page, 6);
  await page.getByRole("textbox", { name: "Chỗ trống 1" }).fill("have lived");
  await page.getByRole("textbox", { name: "Chỗ trống 2" }).fill("2018");

  // 7 · short answer: four words against a limit of three, kept whole.
  await goToQuestion(page, 7);
  await expect(page.getByText("Viết KHÔNG QUÁ BA TỪ.")).toBeVisible();
  const short = page.getByRole("textbox", { name: "Bài làm của bạn" });
  await short.fill("Rice and many shrimps");
  await expect(page.getByText("4/3 từ · quá số từ")).toBeVisible();
  await expect(short).toHaveValue("Rice and many shrimps");

  // 8 · essay.
  await goToQuestion(page, 8);
  await page
    .getByRole("textbox", { name: "Bài luận của bạn" })
    .fill(
      "Rivers feed the delta and its people, so protecting them matters more than new roads.",
    );
  await expect(page.getByText("15 từ", { exact: true })).toBeVisible();
}

test("E2E 11: a paper of every type goes from the student's answers to a graded result", async ({
  browser,
}) => {
  test.setTimeout(240_000);
  const student = await browser.newPage();
  await signInAsStudent(student);
  const attemptId = await freshAttempt(student, ASSIGNMENT.everyType.run);
  await answerEveryType(student);
  await expect(student.getByText("Đã lưu tất cả câu trả lời")).toBeVisible();
  await submitAttempt(student);

  const teacher = await browser.newPage();
  await signInAsAdmin(teacher);
  await gradeAttempt(teacher, attemptId, {
    shortAnswer: "0.5",
    bands: ["6,0", "6,5", "7,0", "6,0"],
    comment: "Clear view; give an example for each side.",
  });
  await teacher.close();

  const read = student.waitForResponse(
    (r) =>
      r.request().method() === "GET" &&
      new URL(r.url()).origin === "http://localhost:8080" &&
      new URL(r.url()).pathname === `/app/attempts/${attemptId}/result`,
  );
  await student.goto(`/app/attempts/${attemptId}/result`);
  const response = await read;
  expect(response.status()).toBe(200);
  const result = (await response.json()) as {
    attempt: {
      status: string;
      score: { earned: number; total: number; pendingManual: number };
    };
    questions: {
      type: string;
      earned: number;
      band?: number;
      criteria?: { label: string; band: number }[];
      correctMatches?: unknown[];
      explanationAssets?: unknown[];
    }[];
  };

  // §7: choice all or nothing, matching and blanks per item, "Choose two" per
  // right pick; then 0.5 for the short answer and band 6.5 (the criteria's
  // mean 6.375, rounded half up to 0.5) at 9 points.
  expect(result.attempt.status).toBe("graded");
  expect(result.attempt.score).toMatchObject({
    earned: 13,
    total: 20,
    pendingManual: 0,
  });
  expect(result.questions.map((q) => [q.type, q.earned])).toEqual([
    ["tfng", 1],
    ["matching", 2],
    ["true_false", 0],
    ["single_choice", 1],
    ["multiple_choice", 1],
    ["fill_blank", 1],
    ["short_answer", 0.5],
    ["essay", 6.5],
  ]);
  const essay = result.questions[7];
  expect(essay?.band).toBe(6.5);
  expect(essay?.criteria?.map((c) => [c.label, c.band])).toEqual([
    ["Task response", 6],
    ["Coherence and cohesion", 6.5],
    ["Lexical resource", 7],
    ["Grammatical range and accuracy", 6],
  ]);
  expect(result.questions[1]?.correctMatches).toHaveLength(3);
  expect(result.questions[3]?.explanationAssets).toHaveLength(1);

  // The page: the total, a tile per part, the missed match, the explanation's
  // image (by its alt text: the seeded asset has no object behind it), and
  // the essay's band with each criterion.
  const main = student.getByRole("main");
  await expect(main).toContainText(/13\s*trên\s*20/);
  await expect(
    main.getByRole("term").filter({ hasText: "Phần 1 · Đọc hiểu" }),
  ).toBeVisible();
  await expect(
    main.getByRole("definition").filter({ hasText: /^3 \/ 5$/ }),
  ).toBeVisible();
  await expect(
    main.getByRole("definition").filter({ hasText: /^10 \/ 15$/ }),
  ).toBeVisible();
  // A True / False / Not given answer reads in the reader's language, as the
  // engine labelled it.
  const tfng = main.getByRole("article").filter({ hasText: "grown in the dry season" });
  await expect(tfng).toContainText("Bạn trả lời Sai");
  await expect(tfng).not.toContainText("False");
  const matching = main
    .getByRole("article")
    .filter({ hasText: "Match each paragraph" });
  await expect(matching.getByText("Đáp án đúng")).toBeVisible();
  await expect(
    matching
      .getByRole("listitem")
      .filter({ hasText: "B. Paragraph B" })
      .filter({ hasText: "Salt water moving inland" }),
  ).toBeVisible();
  const sign = main.getByRole("article").filter({ hasText: "Look at the sign." });
  await expect(
    sign
      .getByRole("figure")
      .filter({ hasText: "Opening hours: Monday to Saturday, 8:00 to 18:00" }),
  ).toBeVisible();
  const graded = main
    .getByRole("article")
    .filter({ hasText: "Write at least 250 words." });
  await expect(graded.getByText("Band 6,5")).toBeVisible();
  for (const [label, band] of [
    ["Task response", "6,0"],
    ["Coherence and cohesion", "6,5"],
    ["Lexical resource", "7,0"],
    ["Grammatical range and accuracy", "6,0"],
  ] as const) {
    await expect(graded.getByRole("term").filter({ hasText: label })).toBeVisible();
    await expect(
      graded.getByRole("definition").filter({ hasText: band }).first(),
    ).toBeVisible();
  }
  await expect(
    graded.getByText("Clear view; give an example for each side."),
  ).toBeVisible();
});
