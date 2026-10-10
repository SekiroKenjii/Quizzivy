import { expect, test, type Page, type Route } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import { contractJson } from "../support/contractResponse";
import { dashboard23Summary } from "../support/dashboard23";

const PUBLISHED_ID = "018f0000-0000-7000-8000-0000000000a2";
const ARCHIVED_ID = "018f0000-0000-7000-8000-0000000000a3";
const COPY_ID = "018f0000-0000-7000-8000-0000000000a4";
const FAILURE = "Máy chủ đang gặp sự cố.";

type Status = "draft" | "published" | "archived";

function testRow(id: string, title: string, status: Status) {
  return {
    id,
    title,
    description: null,
    status,
    currentVersion: status === "draft" ? 0 : 3,
    totalPoints: 30,
    questionCount: 24,
    audioCount: 0,
    skills: [],
    assignments: { live: 0, scheduled: 0, closed: status === "archived" ? 1 : 0 },
    unpublishedChanges: null,
    sections: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

async function json(
  route: Route,
  path: string,
  method: "get" | "post",
  status: number,
  body: unknown,
) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(await contractJson(path, method, status, body).json()),
  });
}

async function fail(route: Route) {
  await route.fulfill({
    status: 500,
    contentType: "application/json",
    body: JSON.stringify({
      error: { code: "INTERNAL", message: FAILURE, requestId: "e2e" },
    }),
  });
}

async function prepare(page: Page) {
  const rows = [
    testRow(PUBLISHED_ID, "Unit 5", "published"),
    testRow(ARCHIVED_ID, "Old mock", "archived"),
  ];
  let duplicates = 0;
  let deletes = 0;
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
    "GET /teacher/tests": (route) =>
      json(route, "/teacher/tests", "get", 200, {
        items: rows,
        page: 1,
        pageSize: 24,
        total: rows.length,
        facets: {
          all: rows.length,
          draft: rows.filter((row) => row.status === "draft").length,
          published: rows.filter((row) => row.status === "published").length,
          archived: rows.filter((row) => row.status === "archived").length,
        },
        tags: [],
      }),
    [`POST /teacher/tests/${PUBLISHED_ID}/duplicate`]: async (route) => {
      duplicates += 1;
      if (duplicates === 1) return fail(route);
      const copy = testRow(COPY_ID, "Unit 5 (bản sao)", "draft");
      rows.unshift(copy);
      return json(route, "/teacher/tests/{id}/duplicate", "post", 201, copy);
    },
    [`DELETE /teacher/tests/${ARCHIVED_ID}`]: async (route) => {
      deletes += 1;
      if (deletes === 1) return fail(route);
      rows.splice(
        rows.findIndex((row) => row.id === ARCHIVED_ID),
        1,
      );
      return route.fulfill({ status: 204 });
    },
  });
  await page.goto("/teacher/tests");
  await expect(page.getByRole("link", { name: "Unit 5" })).toBeVisible();
}

test("a Duplicate that fails says why, and the next try duplicates", async ({
  page,
}) => {
  await prepare(page);
  await page.getByRole("button", { name: "Nhân bản Unit 5" }).click();
  await expect(page.getByText(FAILURE)).toBeVisible();
  await expect(page.getByRole("link", { name: "Unit 5 (bản sao)" })).toHaveCount(0);

  await page.getByRole("button", { name: "Nhân bản Unit 5" }).click();
  await expect(page.getByRole("link", { name: "Unit 5 (bản sao)" })).toBeVisible();
  await expect(page.getByText("Vừa nhân bản")).toHaveCount(2);
});

test("a Duplicate from the menu that fails says why, and the next try duplicates", async ({
  page,
}) => {
  await prepare(page);
  await page.getByRole("button", { name: "Thao tác với Unit 5" }).click();
  await page.getByRole("menuitem", { name: "Nhân bản" }).click();
  await expect(page.getByText(FAILURE)).toBeVisible();

  await page.getByRole("button", { name: "Thao tác với Unit 5" }).click();
  await page.getByRole("menuitem", { name: "Nhân bản" }).click();
  await expect(page.getByRole("link", { name: "Unit 5 (bản sao)" })).toBeVisible();
});

test("a bulk delete that fails lists what failed, and Retry deletes it", async ({
  page,
}) => {
  await prepare(page);
  await page.getByRole("checkbox", { name: "Chọn Old mock" }).check();
  await page.getByRole("button", { name: "Xoá vĩnh viễn" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Xác nhận 1 mục" }).click();

  await expect(dialog.getByRole("alert")).toHaveText(`Old mock: ${FAILURE}`);
  await expect(
    page.getByRole("link", { name: "Old mock", includeHidden: true }),
  ).toHaveCount(1);

  await dialog.getByRole("button", { name: "Thử lại 1 mục chưa thành công" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("link", { name: "Old mock" })).toHaveCount(0);
  await expect(page.getByText("Đã xử lý 1 mục.")).toBeVisible();
});
