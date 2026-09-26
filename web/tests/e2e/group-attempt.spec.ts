import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";
import {
  previewGroup,
  previewQuestions,
  previewSection,
} from "../support/groupPreview";
import type { components } from "../../src/lib/api/schema";

const attemptId = "01935000-0000-7000-8000-000000000088";
const audio = fileURLToPath(new URL("./fixtures/unit5-listening.mp3", import.meta.url));
const recordingId = previewGroup.recordings[0]!.id;

async function setup(page: Page) {
  const seen = new Set<string>();
  let loseFirstResponse = true;
  const now = new Date().toISOString();
  const payload: components["schemas"]["AttemptSession"] = {
    attempt: {
      id: attemptId,
      assignmentId: attemptId,
      studentId: studentUser.id,
      testVersionId: attemptId,
      attemptNo: 1,
      status: "in_progress",
      startedAt: now,
      deadlineAt: new Date(Date.now() + 3600000).toISOString(),
    },
    testTitle: "Đọc và nghe theo nhóm",
    sections: [previewSection],
    questions: previewQuestions.slice(1),
    groups: [previewGroup],
    audioPlays: {},
    answers: {},
    sessionId: attemptId,
    beaconToken: "synthetic",
    serverTime: now,
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: false,
      maxFocusLoss: 0,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
  };
  await stubApi(page, {
    ...sessionAs(studentUser),
    [`GET /app/attempts/${attemptId}`]: (route) =>
      route.fulfill({
        json: { ...payload, groupAudioPlays: { [recordingId]: seen.size } },
      }),
    [`PATCH /app/attempts/${attemptId}/answers`]: {
      body: { savedAt: now, serverTime: now },
    },
    [`POST /app/attempts/${attemptId}/group-audio-play`]: async (route) => {
      const input = route
        .request()
        .postDataJSON() as components["schemas"]["GroupAudioPlayInput"];
      seen.add(input.playId);
      if (loseFirstResponse) {
        loseFirstResponse = false;
        await route.abort("failed");
      } else
        await route.fulfill({
          json: { playId: input.playId, plays: seen.size, maxPlays: 2 },
        });
    },
  });
  await page.route("https://assets.example/synthetic.mp3", (route) =>
    route.fulfill({ path: audio, contentType: "audio/mpeg" }),
  );
  await page.goto(`/app/attempts/${attemptId}`);
  return seen;
}

for (const width of [320, 1440]) {
  test(`shared materials, audio recovery and gap navigation at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const seen = await setup(page);
    const material = page.getByRole("heading", { name: previewGroup.title });
    await expect(material).toBeVisible();
    await expect(page.getByText("Còn 2 lượt nghe")).toBeVisible();
    await page.getByRole("button", { name: "Phát", exact: true }).click();
    await expect(page.getByText("Còn 1 lượt nghe")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Tạm dừng", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Câu sau", exact: true }).click();
    await expect(
      page.getByText(previewQuestions[2]!.prompt, { exact: true }),
    ).toBeVisible();
    await expect(page.locator("audio")).toHaveCount(1);
    await expect(
      page.getByRole("button", { name: "Tạm dừng", exact: true }),
    ).toBeVisible();
    await page.reload();
    await expect(page.getByText("Còn 1 lượt nghe")).toBeVisible();
    await expect(page.getByText("Lượt nghe đang chờ đồng bộ.")).toHaveCount(0);
    expect(seen.size).toBe(1);
    if (width === 320) {
      await page.getByRole("button", { name: "Thu gọn ngữ liệu" }).click();
      await expect(page.getByText("Lịch hoạt động", { exact: true })).toBeHidden();
      await expect(
        page.getByRole("button", { name: "Phát", exact: true }),
      ).toBeVisible();
      await page.getByRole("button", { name: "Mở ngữ liệu" }).click();
    }
    const gap = page.getByRole("button", { name: "Ô A — chuyển đến câu 2" });
    await gap.focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(":focus")).toHaveAttribute(
      "id",
      `answer-question-${previewQuestions[2]!.id}`,
    );
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1,
    );
    expect(overflow).toBe(false);
    if (width === 1440) {
      const contextBox = await page
        .getByLabel(previewGroup.title, { exact: true })
        .boundingBox();
      const questionBox = await page
        .locator(`#answer-question-${previewQuestions[2]!.id}`)
        .boundingBox();
      expect(contextBox!.x + contextBox!.width).toBeLessThanOrEqual(questionBox!.x);
    }
    await page.screenshot({
      path: test.info().outputPath(`group-attempt-${width}.png`),
    });
  });
}
