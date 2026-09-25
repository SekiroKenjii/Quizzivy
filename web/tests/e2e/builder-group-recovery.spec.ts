import { expect, test } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import type { components } from "../../src/lib/api/schema";

test("leaving an offline group keeps an acknowledged local draft and restores its outline before saving", async ({
  page,
}) => {
  const testId = "019535d9-3df7-79fb-b466-fa907fa17fa1";
  const sectionId = "019535d9-3df7-79fb-b466-fa907fa17fa2";
  const groupId = "019535d9-3df7-79fb-b466-fa907fa17fa3";
  const stamp = "2026-09-24T00:00:00Z";
  let offline = true;
  let stored: components["schemas"]["StoredQuestionGroup"] = {
    bundle: {
      group: {
        id: groupId,
        title: "Nhóm trên máy chủ",
        instructions: null,
        members: [],
        stimuli: [],
        recordings: [],
      },
      questions: [],
    },
    ownerSectionId: sectionId,
    revision: 1,
    testUpdatedAt: stamp,
    archivedAt: null,
    createdAt: stamp,
    updatedAt: stamp,
    assets: [],
    unavailableAssetIds: [],
  };
  const draft: components["schemas"]["Test"] = {
    id: testId,
    title: "Đề phục hồi",
    description: null,
    status: "draft",
    currentVersion: 0,
    totalPoints: 0,
    questionCount: 0,
    audioCount: 0,
    sections: [
      {
        id: sectionId,
        ordinal: 0,
        title: "Phần 1",
        questionIds: [],
        units: [{ kind: "group", id: groupId }],
      },
    ],
    createdAt: stamp,
    updatedAt: stamp,
  };
  await stubApi(page, {
    ...sessionAs(adminUser),
    "GET /admin/tests": {
      body: {
        items: [draft],
        total: 1,
        page: 1,
        limit: 20,
        tags: [],
        facets: { all: 1, draft: 1, published: 0, archived: 0 },
      },
    },
    [`GET /admin/tests/${testId}`]: (route) => route.fulfill({ json: draft }),
    [`GET /admin/question-groups/${groupId}`]: (route) =>
      route.fulfill({ json: stored }),
    [`PUT /admin/question-groups/${groupId}`]: (route) => {
      if (offline)
        return route.fulfill({
          status: 503,
          json: { error: { code: "UNKNOWN", message: "Mất kết nối thử nghiệm" } },
        });
      const input = route
        .request()
        .postDataJSON() as components["schemas"]["GroupUpdateInput"];
      expect(input.expectedRevision).toBe(stored.revision);
      stored = { ...stored, bundle: input.bundle, revision: stored.revision + 1 };
      return route.fulfill({ json: stored });
    },
  });
  await page.goto(`/admin/tests/${testId}/edit`);
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill("Nhóm cần khôi phục");
  await expect(
    page.getByText("Chưa đồng bộ · Đã lưu bản nháp trên máy", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Quay lại", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Rời đi", exact: true })
    .click();
  await expect(page).toHaveURL(/\/admin\/tests$/);
  expect(stored.bundle.group.title).toBe("Nhóm trên máy chủ");
  await page.goBack();
  await page.getByRole("button", { name: "Khôi phục bản nháp", exact: true }).click();
  await expect(page.getByLabel("Tên nhóm câu hỏi", { exact: true })).toHaveValue(
    "Nhóm cần khôi phục",
  );
  await expect(
    page
      .locator("[data-outline-group]")
      .getByText("Nhóm cần khôi phục", { exact: true }),
  ).toBeVisible();
  offline = false;
  await page.getByRole("button", { name: "Quay lại", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Lưu và rời trang", exact: true })
    .click();
  await expect(page).toHaveURL(/\/admin\/tests$/);
  expect(stored.bundle.group.title).toBe("Nhóm cần khôi phục");
});
