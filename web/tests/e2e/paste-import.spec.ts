import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import { contractJson } from "../support/contractResponse";
import { dashboard23Summary } from "../support/dashboard23";

const IMPORT_ID = "018f0000-0000-7000-8000-0000000000e1";
const AT = "2026-10-10T01:00:00Z";

const TEXT = [
  "Unit 5 Quick Check",
  "Part 1. Choose the best answer.",
  "1. She ___ in Hanoi since 2019.",
  "A. lives",
  "B. has lived *",
  "C. lived",
  "2. They ___ here for years.",
  "Answer: have lived",
].join("\n");

function wordImport(revision: number, pasted: boolean) {
  return {
    id: IMPORT_ID,
    title: "Unit 5 Quick Check",
    status: "awaiting_sources",
    revision,
    sourceRevision: pasted ? 1 : 0,
    sources: pasted ? [pastedSource()] : [],
    pendingUploads: 0,
    createdBy: adminUser.id,
    createdAt: AT,
    updatedAt: AT,
  };
}

function pastedSource() {
  return {
    id: "018f0000-0000-7000-8000-0000000000e3",
    role: "exam",
    filename: "pasted-text.txt",
    format: "text",
    characters: TEXT.length,
    bytes: TEXT.length,
    sha256: "a".repeat(64),
    uploadedBy: adminUser.id,
    createdAt: AT,
  };
}

async function body(
  path: string,
  method: "get" | "post",
  status: number,
  value: unknown,
) {
  return JSON.stringify(await contractJson(path, method, status, value).json());
}

async function prepare(page: Page) {
  const sent: string[] = [];
  await stubApi(page, {
    ...sessionAs(adminUser),
    "GET /teacher/summary": { body: dashboard23Summary() },
    "GET /me/summary": { body: { unreadNotifications: 0 } },
    "GET /teacher/imports/capabilities": {
      body: {
        intakeEnabled: true,
        processingEnabled: true,
        retention: { afterCommitDays: 30, afterCancelDays: 7, idleDays: 60 },
      },
    },
    "GET /teacher/imports/limits": {
      body: {
        pasteMaxCharacters: 100000,
        maxBytes: 26214400,
        formats: ["docx", "pdf"],
      },
    },
    "POST /teacher/imports": async (route) => {
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: await body("/teacher/imports", "post", 201, wordImport(1, false)),
      });
    },
    [`POST /teacher/imports/${IMPORT_ID}/sources/text`]: async (route) => {
      sent.push((route.request().postDataJSON() as { text: string }).text);
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: await body("/teacher/imports/{id}/sources/text", "post", 201, {
          import: wordImport(2, true),
          source: pastedSource(),
          sourceRevision: 1,
        }),
      });
    },
    [`POST /teacher/imports/${IMPORT_ID}/process`]: async (route) => {
      await route.fulfill({
        status: 202,
        contentType: "application/json",
        body: await body("/teacher/imports/{id}/process", "post", 202, {
          ...wordImport(3, true),
          status: "queued",
        }),
      });
    },
    [`GET /teacher/imports/${IMPORT_ID}`]: {
      body: { ...wordImport(3, true), status: "queued" },
    },
  });
  return sent;
}

test("a real paste event fills the box, and Ctrl+Enter starts the import", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const sent = await prepare(page);
  await page.goto("/teacher/imports/new?source=paste");
  const box = page.getByRole("textbox", { name: "Nội dung đề" });
  await expect(box).toBeVisible();
  await page.evaluate((text) => navigator.clipboard.writeText(text), TEXT);
  await box.focus();
  await page.keyboard.press("ControlOrMeta+V");
  await expect(box).toHaveValue(TEXT);
  await expect(page.getByText("Đã tìm thấy")).toBeVisible();
  await expect(page.getByLabel("Tên đề")).toHaveValue("Unit 5 Quick Check");
  await expect(page.getByText(/^Tìm thấy 2 câu hỏi\./)).toBeVisible();
  await box.press("ControlOrMeta+Enter");
  await expect(page).toHaveURL(new RegExp(`/teacher/imports/${IMPORT_ID}$`));
  expect(sent).toEqual([TEXT]);
});

test("a refused clipboard says how to paste by hand", async ({ page, context }) => {
  await prepare(page);
  const cdp = await context.newCDPSession(page);
  const { targetInfo } = await cdp.send("Target.getTargetInfo");
  await cdp.send("Browser.setPermission", {
    permission: { name: "clipboard-read" },
    setting: "denied",
    ...(targetInfo.browserContextId === undefined
      ? {}
      : { browserContextId: targetInfo.browserContextId }),
  });
  await page.goto("/teacher/imports/new?source=paste");
  await page.getByRole("button", { name: "Dán từ bộ nhớ tạm" }).click();
  await expect(
    page.getByText(
      "Trình duyệt đã chặn bộ nhớ tạm. Hãy bấm vào ô và nhấn Ctrl+V (Cmd+V trên Mac).",
    ),
  ).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Nội dung đề" })).toHaveValue("");
});
