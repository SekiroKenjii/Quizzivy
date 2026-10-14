import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { signInAsAdmin } from "./support/live";

const AUDIO = fileURLToPath(new URL("./fixtures/unit5-listening.mp3", import.meta.url));

async function saved(page: Page) {
  await expect(page.locator('[role="status"][data-state="saved"]')).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
}

test("mixed builder saves new sections, moves complete groups, copies context and publishes through the real API", async ({
  page,
}, testInfo) => {
  test.setTimeout(150_000);
  await signInAsAdmin(page);
  await page.goto("/teacher/tests");
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);
  const builderPath = new URL(page.url()).pathname;
  await page.getByRole("button", { name: "Tên đề thi", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Tên đề thi", exact: true })
    .fill(`Đề nhóm ${Date.now()}`);
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("Enter");
  const sections = page.locator("[data-outline-section]");
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);
  const sectionCount = await sections.count();
  expect(sectionCount).toBeGreaterThan(0);
  await page.getByRole("button", { name: "Tên đề thi", exact: true }).click();
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("End");
  await page
    .getByRole("textbox", { name: "Tên đề thi", exact: true })
    .pressSequentially(" kiểm tra");
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("Enter");
  await saved(page);
  await expect(sections).toHaveCount(sectionCount);
  await page
    .getByRole("button", { name: "Thêm nhóm câu dùng chung", exact: true })
    .click();
  const title = `Bài đọc lớp ${Date.now()}`;
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill(title);
  await page.getByRole("button", { name: "Thêm ngữ liệu", exact: true }).click();
  await page.getByLabel("Tên ngữ liệu", { exact: true }).fill("Lịch sinh hoạt");
  await page
    .getByRole("textbox", { name: "Nội dung ngữ liệu", exact: true })
    .fill("Câu lạc bộ mở cửa vào thứ Bảy.");
  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  const prompt = page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true });
  await prompt.fill("Câu lạc bộ mở cửa vào ngày nào?");
  await saved(page);
  const original = page.locator("[data-outline-group]").first();
  const originalId = await original.getAttribute("data-outline-group");
  expect(originalId).toBeTruthy();
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);
  const moving = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      /\/teacher\/tests\/[^/]+$/.test(new URL(response.url()).pathname),
  );
  await original
    .getByRole("button", { name: `Thao tác với nhóm ${title}`, exact: true })
    .click();
  await page.getByRole("menuitem", { name: "Di chuyển xuống", exact: true }).click();
  await prompt.fill("Theo lịch, câu lạc bộ mở cửa vào ngày nào?");
  expect((await moving).status()).toBe(200);
  await saved(page);
  await expect(
    sections.last().locator(`[data-outline-group="${originalId}"]`),
  ).toBeVisible();
  await page.getByRole("button", { name: "Xem trước", exact: true }).click();
  const preview = page.getByRole("dialog");
  await expect(
    preview.getByText("Câu lạc bộ mở cửa vào thứ Bảy.", { exact: true }),
  ).toBeVisible();
  await expect(
    preview.getByText("Theo lịch, câu lạc bộ mở cửa vào ngày nào?", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await page
    .getByRole("button", { name: "Lưu bản sao vào ngân hàng", exact: true })
    .click();
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: "Đã lưu bản sao độc lập vào ngân hàng." }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Từ ngân hàng câu hỏi", exact: true }).click();
  await page.getByRole("button", { name: "Chọn cả nhóm câu hỏi", exact: true }).click();
  const picker = page.getByRole("dialog");
  await picker.getByRole("textbox").fill(title);
  await picker.getByRole("button", { name: new RegExp(title) }).click();
  await expect(picker).toBeHidden();
  await expect(page.locator("[data-outline-group]")).toHaveCount(2);
  await expect(page).toHaveURL(new RegExp(`${builderPath}$`));
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.screenshot({
    path: testInfo.outputPath("builder-groups-desktop.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 768, height: 1000 });
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    )
    .toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("builder-groups-tablet.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 1440, height: 1000 });
  const deleted = page.waitForResponse(
    (response) =>
      response.request().method() === "DELETE" &&
      new URL(response.url()).pathname === `/teacher/question-groups/${originalId}`,
  );
  await page
    .locator(`[data-outline-group="${originalId}"]`)
    .getByRole("button", { name: `Thao tác với nhóm ${title}`, exact: true })
    .click();
  await page.getByRole("menuitem", { name: "Xoá nhóm", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Xoá", exact: true })
    .click();
  expect((await deleted).status()).toBe(204);
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(page.locator("[data-outline-group]")).toHaveCount(1);
  await page.reload();
  await expect(page.locator("[data-outline-group]")).toHaveCount(1);
  await page.getByRole("button", { name: "Xem trước", exact: true }).click();
  await expect(
    page
      .getByRole("dialog")
      .getByText("Câu lạc bộ mở cửa vào thứ Bảy.", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  const beforeRemoval = await sections.count();
  await sections
    .last()
    .getByRole("button", { name: "Thao tác với phần", exact: true })
    .click();
  await page.getByRole("menuitem", { name: "Gỡ phần", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(sections).toHaveCount(beforeRemoval - 1);
  await saved(page);
  const published = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname.endsWith("/publish"),
  );
  await page.getByRole("button", { name: "Phát hành", exact: true }).click();
  await page
    .getByRole("dialog", { name: "Phát hành đề thi?", exact: true })
    .getByRole("button", { name: "Phát hành", exact: true })
    .click();
  expect((await published).status()).toBe(201);
  await expect(page.getByText("Đã phát hành. Giờ bạn có thể giao bài.")).toBeVisible();
  await page.goto(builderPath.replace(/\/edit$/, ""));
  await expect(page).toHaveURL(builderPath.replace(/\/edit$/, ""));
});

test("the Shared content dialog saves a recording, then a passage over it, in the shapes the real API accepts", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signInAsAdmin(page);
  await page.goto("/teacher/tests");
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);
  await page
    .getByRole("button", { name: "Thêm nhóm câu dùng chung", exact: true })
    .click();
  const title = `Nghe và đọc ${Date.now()}`;
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill(title);
  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .fill("Người nói hẹn gặp ở đâu?");
  await saved(page);
  const group = page.locator("[data-outline-group]").first();
  const dialogName = `Nội dung dùng chung của “${title}”`;
  const groupSaved = () =>
    page.waitForResponse(
      (response) =>
        response.request().method() === "PUT" &&
        /\/teacher\/question-groups\/[^/]+$/.test(new URL(response.url()).pathname),
    );

  await group
    .getByRole("button", { name: `Thao tác với nhóm ${title}`, exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Nội dung dùng chung", exact: true })
    .click();
  let dialog = page.getByRole("dialog", { name: dialogName });
  await dialog.getByRole("button", { name: "Bài nghe", exact: true }).click();
  await dialog.getByLabel("Chọn tệp từ máy").setInputFiles(AUDIO);
  await expect(dialog.getByText("unit5-listening.mp3", { exact: true })).toBeVisible({
    timeout: 30_000,
  });
  let response = groupSaved();
  await dialog.getByRole("button", { name: "Lưu", exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(dialog).toBeHidden();
  const audioLine = group.getByRole("button", {
    name: "Bài nghe · unit5-listening.mp3 · 2 lượt nghe",
    exact: true,
  });
  await expect(audioLine).toBeVisible();

  await audioLine.click();
  dialog = page.getByRole("dialog", { name: dialogName });
  await dialog.getByRole("button", { name: "Bài đọc", exact: true }).click();
  await dialog
    .getByRole("textbox", { name: "Tiêu đề", exact: true })
    .fill("Lịch câu lạc bộ");
  await dialog
    .getByRole("textbox", { name: "Bài đọc", exact: true })
    .fill("Câu lạc bộ mở cửa vào thứ Bảy.");
  await dialog.getByRole("button", { name: "Lưu", exact: true }).click();
  const confirm = page.getByRole("dialog", { name: "Thay nội dung dùng chung?" });
  await expect(confirm).toContainText("unit5-listening.mp3");
  response = groupSaved();
  await confirm.getByRole("button", { name: "Thay và lưu", exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(dialog).toBeHidden();
  await expect(
    group.getByRole("button", { name: "Bài đọc · Lịch câu lạc bộ", exact: true }),
  ).toBeVisible();

  await page.reload();
  await expect(
    page
      .locator("[data-outline-group]")
      .first()
      .getByRole("button", { name: "Bài đọc · Lịch câu lạc bộ", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Người nói hẹn gặp ở đâu?").first()).toBeVisible();
});

interface Json {
  [key: string]: unknown;
}

function memberAt(group: Json, index: number) {
  const bundle = group["bundle"] as {
    group: { members: { questionId: string }[] };
    questions: { id: string; input: Json }[];
  };
  const id = bundle.group.members[index]!.questionId;
  return { id, input: bundle.questions.find((question) => question.id === id)!.input };
}

test("dragging a bank question into a group copies it under a new id, bumps the test and the group once, and publishes it inside the passage", async ({
  page,
}) => {
  test.setTimeout(180_000);
  await signInAsAdmin(page);
  await page.goto("/teacher/tests");
  const creating = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/teacher/tests",
  );
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).click();
  const creation = await creating;
  const origin = new URL(creation.url()).origin;
  const authorization = (await creation.request().allHeaders())["authorization"]!;
  const read = async (path: string): Promise<Json> => {
    const response = await page.request.get(`${origin}${path}`, {
      headers: { authorization },
    });
    expect(response.status(), path).toBe(200);
    return (await response.json()) as Json;
  };
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);
  const testId = new URL(page.url()).pathname.split("/").at(-2)!;
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);

  const token = `E1M${Date.now()}`;
  const bankPrompt = `Câu độc lập ${token}`;
  const bankCreated = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/teacher/questions",
  );
  await page.getByRole("button", { name: /^Thêm câu hỏi/ }).click();
  const sourceId = ((await (await bankCreated).json()) as Json)["id"] as string;
  await page.getByRole("spinbutton", { name: "Điểm", exact: true }).fill("3");
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .fill(bankPrompt);
  await saved(page);

  await page
    .getByRole("button", { name: "Thêm nhóm câu dùng chung", exact: true })
    .click();
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill(`Nhóm ${token}`);
  await page.getByRole("button", { name: "Thêm ngữ liệu", exact: true }).click();
  await page.getByLabel("Tên ngữ liệu", { exact: true }).fill("Lịch sinh hoạt");
  await page
    .getByRole("textbox", { name: "Nội dung ngữ liệu", exact: true })
    .fill("Câu lạc bộ mở cửa vào thứ Bảy.");
  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  const resident = `Câu sẵn có ${token}`;
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .fill(resident);
  await saved(page);
  const groupId = (await page
    .locator("[data-outline-group]")
    .first()
    .getAttribute("data-outline-group"))!;
  await expect(page.locator(`[data-outline-question="${sourceId}"]`)).toBeVisible();

  const bankBefore = await read(`/teacher/questions/${sourceId}`);
  const groupBefore = await read(`/teacher/question-groups/${groupId}`);
  const testBefore = await read(`/teacher/tests/${testId}`);
  const revisionBefore = groupBefore["revision"] as number;
  const stampBefore = testBefore["updatedAt"] as string;
  const residentId = memberAt(groupBefore, 0).id;
  expect(bankBefore["prompt"]).toBe(bankPrompt);
  expect(bankBefore["points"]).toBe(3);

  const moving = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === `/teacher/tests/${testId}/questions/move`,
  );
  const grip = page.getByRole("button", {
    name: "Kéo để đổi vị trí câu 1",
    exact: true,
  });
  await grip.focus();
  await page.keyboard.press("Space");
  await expect(grip).toHaveAttribute("aria-pressed", "true");
  await page.keyboard.press("ArrowDown");
  await expect(page.locator("[data-outline-drop-into]")).toHaveClass(/bg-brand-soft/);
  await page.keyboard.press("Space");
  const move = await moving;
  expect(move.status()).toBe(200);

  expect(move.request().postDataJSON()).toEqual({
    expectedUpdatedAt: stampBefore,
    questionId: sourceId,
    to: {
      sectionId: (testBefore["sections"] as { id: string }[])[0]!.id,
      groupId,
      index: 0,
    },
    expectedGroupRevisions: [{ groupId, revision: revisionBefore }],
  });
  const result = (await move.json()) as {
    test: { updatedAt: string };
    questionId: string;
    copied: boolean;
    groups: { revision: number; bundle: { group: { id: string } } }[];
  };
  const copyId = result.questionId;
  expect(result.copied).toBe(true);
  expect(copyId).not.toBe(sourceId);
  expect(copyId).not.toBe(residentId);
  expect(result.groups).toHaveLength(1);
  expect(result.groups[0]!.bundle.group.id).toBe(groupId);
  expect(result.groups[0]!.revision).toBe(revisionBefore + 1);
  expect(Date.parse(result.test.updatedAt)).toBeGreaterThan(Date.parse(stampBefore));

  await expect(
    page.getByText("Đã sao chép câu hỏi vào nhóm. Bản gốc vẫn ở ngân hàng câu hỏi."),
  ).toBeVisible();
  const outlineGroup = page.locator(`[data-outline-group="${groupId}"]`);
  await expect(page.locator(`[data-outline-question="${sourceId}"]`)).toHaveCount(0);
  await expect(outlineGroup.locator("[data-outline-question]")).toHaveCount(2);
  await expect(
    outlineGroup.locator(`[data-outline-question="${copyId}"]`),
  ).toBeVisible();
  await saved(page);

  const testAfter = await read(`/teacher/tests/${testId}`);
  const groupAfter = await read(`/teacher/question-groups/${groupId}`);
  expect(testAfter["updatedAt"]).toBe(result.test.updatedAt);
  expect(groupAfter["revision"]).toBe(revisionBefore + 1);
  expect(memberAt(groupAfter, 0).id).toBe(copyId);
  expect(memberAt(groupAfter, 1).id).toBe(residentId);
  const copy = memberAt(groupAfter, 0).input as {
    prompt: string;
    points: number;
    options: { text: string; isCorrect: boolean }[];
  };
  const bank = bankBefore as unknown as typeof copy;
  expect(copy.prompt).toBe(bankPrompt);
  expect(copy.points).toBe(3);
  expect(copy.options.map(({ text, isCorrect }) => ({ text, isCorrect }))).toEqual(
    bank.options.map(({ text, isCorrect }) => ({ text, isCorrect })),
  );
  expect(
    (testAfter["sections"] as { units: { kind: string; id: string }[] }[])[0]!.units,
  ).toEqual([{ kind: "group", id: groupId }]);

  const { usedIn: usedBefore, usedInTests: countBefore, ...rowBefore } = bankBefore;
  const {
    usedIn: usedAfter,
    usedInTests: countAfter,
    ...rowAfter
  } = await read(`/teacher/questions/${sourceId}`);
  expect(rowAfter).toEqual(rowBefore);
  expect([countBefore, (usedBefore as unknown[]).length]).toEqual([1, 1]);
  expect([countAfter, (usedAfter as unknown[]).length]).toEqual([0, 0]);
  const listed = (await read(
    `/teacher/questions?q=${encodeURIComponent(token)}&limit=50`,
  )) as { items: { id: string }[] };
  expect(listed.items.map((item) => item.id)).toEqual([sourceId]);

  const edited = `${bankPrompt} đã sửa`;
  const saving = page.waitForResponse(
    (response) =>
      response.request().method() === "PUT" &&
      new URL(response.url()).pathname === `/teacher/question-groups/${groupId}`,
  );
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .fill(edited);
  expect((await saving).status()).toBe(200);
  await saved(page);
  expect((await read(`/teacher/question-groups/${groupId}`))["revision"]).toBe(
    revisionBefore + 2,
  );
  expect(((await read(`/teacher/questions/${sourceId}`)) as Json)["prompt"]).toBe(
    bankPrompt,
  );

  const published = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === `/teacher/tests/${testId}/publish`,
  );
  await page.getByRole("button", { name: "Phát hành", exact: true }).click();
  await page
    .getByRole("dialog", { name: "Phát hành đề thi?", exact: true })
    .getByRole("button", { name: "Phát hành", exact: true })
    .click();
  expect((await published).status()).toBe(201);
  await expect(page.getByText("Đã phát hành. Giờ bạn có thể giao bài.")).toBeVisible();

  const preview = (await read(`/teacher/tests/${testId}/preview`)) as {
    questions: { id: string; prompt: string }[];
    groups: { questionIds: string[]; stimuli: { title: string }[] }[];
  };
  expect(preview.questions).toHaveLength(2);
  expect(preview.groups).toHaveLength(1);
  const inGroup = preview.groups[0]!.questionIds.map(
    (id) => preview.questions.find((question) => question.id === id)!.prompt,
  );
  expect(inGroup).toEqual([edited, resident]);

  await page.goto(`/teacher/tests/${testId}`);
  const frame = page.getByRole("region", { name: "Màn hình của học viên" });
  await expect(frame.getByText(edited, { exact: true })).toBeVisible();
  await expect(frame.getByRole("heading", { name: "Lịch sinh hoạt" })).toBeVisible();
  await expect(frame.getByText("Câu lạc bộ mở cửa vào thứ Bảy.")).toBeVisible();
});

interface Setup {
  testId: string;
  groupId: string;
  sourceId: string;
  token: string;
  bankPrompt: string;
  read: (path: string) => Promise<Json>;
  statusOf: (path: string) => Promise<number>;
}

async function startWithABankQuestionBesideAGroup(page: Page): Promise<Setup> {
  await signInAsAdmin(page);
  await page.goto("/teacher/tests");
  const creating = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/teacher/tests",
  );
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).click();
  const creation = await creating;
  const api = new URL(creation.url()).origin;
  const headers = {
    authorization: (await creation.request().allHeaders())["authorization"]!,
  };
  const read = async (path: string): Promise<Json> => {
    const response = await page.request.get(`${api}${path}`, { headers });
    expect(response.status(), path).toBe(200);
    return (await response.json()) as Json;
  };
  const statusOf = async (path: string) =>
    (await page.request.get(`${api}${path}`, { headers })).status();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);
  const testId = new URL(page.url()).pathname.split("/").at(-2)!;
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);

  const token = `E1R${Date.now()}`;
  const bankPrompt = `Câu độc lập ${token}`;
  const bankCreated = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/teacher/questions",
  );
  await page.getByRole("button", { name: /^Thêm câu hỏi/ }).click();
  const sourceId = ((await (await bankCreated).json()) as Json)["id"] as string;
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .fill(bankPrompt);
  await saved(page);

  await page
    .getByRole("button", { name: "Thêm nhóm câu dùng chung", exact: true })
    .click();
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill(`Nhóm ${token}`);
  await page.getByRole("button", { name: "Thêm ngữ liệu", exact: true }).click();
  await page.getByLabel("Tên ngữ liệu", { exact: true }).fill("Lịch sinh hoạt");
  await page
    .getByRole("textbox", { name: "Nội dung ngữ liệu", exact: true })
    .fill("Câu lạc bộ mở cửa vào thứ Bảy.");
  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .fill(`Câu sẵn có ${token}`);
  await saved(page);
  const groupId = (await page
    .locator("[data-outline-group]")
    .first()
    .getAttribute("data-outline-group"))!;
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);
  expect((await read(`/teacher/tests/${testId}`))["sections"]).toHaveLength(2);
  return { testId, groupId, sourceId, token, bankPrompt, read, statusOf };
}

interface MoveAnswer {
  questionId: string;
  copied: boolean;
  restored: boolean;
  editedCopy: boolean;
}

async function answerOf(page: Page, setup: Setup, gesture: () => Promise<void>) {
  const moving = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname ===
        `/teacher/tests/${setup.testId}/questions/move`,
  );
  await gesture();
  const response = await moving;
  expect(response.status()).toBe(200);
  return {
    body: response.request().postDataJSON() as Json,
    answer: (await response.json()) as MoveAnswer,
  };
}

function dragIntoTheGroup(page: Page, setup: Setup) {
  return answerOf(page, setup, async () => {
    const handle = page.getByRole("button", {
      name: "Kéo để đổi vị trí câu 1",
      exact: true,
    });
    await handle.focus();
    await page.keyboard.press("Space");
    await expect(handle).toHaveAttribute("aria-pressed", "true");
    await page.keyboard.press("ArrowDown");
    await expect(page.locator("[data-outline-drop-into]")).toHaveClass(/bg-brand-soft/);
    await page.keyboard.press("Space");
  });
}

function dragOutToTheNextSection(page: Page, setup: Setup) {
  return answerOf(page, setup, async () => {
    const handle = page.getByRole("button", { name: "Kéo câu 1", exact: true });
    const zone = page.getByText("Kéo câu hỏi vào đây", { exact: true });
    const from = (await handle.boundingBox())!;
    const to = (await zone.boundingBox())!;
    await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
    await page.mouse.down();
    await page.mouse.move(from.x + from.width / 2 + 12, from.y + from.height / 2 + 12, {
      steps: 4,
    });
    await page.mouse.move(to.x + to.width / 2, to.y + to.height / 2, { steps: 24 });
    await expect(zone).toHaveClass(/bg-brand-soft/);
    await page.mouse.up();
  });
}

async function bankHolding(setup: Setup): Promise<string[]> {
  const listed = (await setup.read(
    `/teacher/questions?q=${encodeURIComponent(setup.token)}&limit=50`,
  )) as { items: { id: string; prompt: string }[] };
  return listed.items
    .filter((item) => item.prompt.startsWith("Câu độc lập"))
    .map((item) => item.id);
}

test("dragging a bank question into a group and straight back out restores the original and leaves one bank question", async ({
  page,
}) => {
  test.setTimeout(180_000);
  const setup = await startWithABankQuestionBesideAGroup(page);
  const { sourceId, groupId } = setup;
  expect(await bankHolding(setup)).toEqual([sourceId]);

  const into = await dragIntoTheGroup(page, setup);
  const copyId = into.answer.questionId;
  expect(into.answer.copied).toBe(true);
  expect(into.answer.restored).toBe(false);
  expect(into.answer.editedCopy).toBe(false);
  expect(copyId).not.toBe(sourceId);
  expect(await bankHolding(setup)).toEqual([sourceId]);
  await expect(
    page.locator(
      `[data-outline-group="${groupId}"] [data-outline-question="${copyId}"]`,
    ),
  ).toBeVisible();
  await saved(page);

  const out = await dragOutToTheNextSection(page, setup);
  expect(out.body["questionId"]).toBe(copyId);
  expect(out.answer.restored).toBe(true);
  expect(out.answer.editedCopy).toBe(false);
  expect(out.answer.copied).toBe(false);
  expect(out.answer.questionId).toBe(sourceId);

  await expect(page.getByText("Đã trả câu hỏi gốc về đề thi.")).toBeVisible();
  await expect(page.locator(`[data-outline-question="${sourceId}"]`)).toBeVisible();
  await expect(page.locator(`[data-outline-question="${copyId}"]`)).toHaveCount(0);
  await saved(page);

  const after = await setup.read(`/teacher/tests/${setup.testId}`);
  const units = (
    after["sections"] as { units: { kind: string; id: string }[] }[]
  ).flatMap((section) => section.units);
  expect(units.filter((unit) => unit.kind === "question")).toEqual([
    { kind: "question", id: sourceId },
  ]);
  expect(await bankHolding(setup)).toEqual([sourceId]);
  expect(await setup.statusOf(`/teacher/questions/${copyId}`)).toBe(404);
});

test("a copy that was edited in the group stays a new bank question when it is dragged out", async ({
  page,
}) => {
  test.setTimeout(180_000);
  const setup = await startWithABankQuestionBesideAGroup(page);
  const { sourceId, groupId, bankPrompt } = setup;

  const into = await dragIntoTheGroup(page, setup);
  const copyId = into.answer.questionId;
  await saved(page);

  const edited = `${bankPrompt} đã sửa`;
  const saving = page.waitForResponse(
    (response) =>
      response.request().method() === "PUT" &&
      new URL(response.url()).pathname === `/teacher/question-groups/${groupId}`,
  );
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .fill(edited);
  expect((await saving).status()).toBe(200);
  await saved(page);

  const out = await dragOutToTheNextSection(page, setup);
  expect(out.body["questionId"]).toBe(copyId);
  expect(out.answer.restored).toBe(false);
  expect(out.answer.editedCopy).toBe(true);
  expect(out.answer.copied).toBe(false);
  expect(out.answer.questionId).toBe(copyId);

  await expect(
    page.getByText(
      "Câu hỏi đã được sửa nên được lưu thành câu hỏi mới trong ngân hàng.",
    ),
  ).toBeVisible();
  await saved(page);
  const bank = await bankHolding(setup);
  expect(bank).toHaveLength(2);
  expect(bank).toEqual(expect.arrayContaining([sourceId, copyId]));
  expect((await setup.read(`/teacher/questions/${copyId}`))["prompt"]).toBe(edited);
  expect((await setup.read(`/teacher/questions/${sourceId}`))["prompt"]).toBe(
    bankPrompt,
  );
});
