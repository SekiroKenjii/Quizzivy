import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import {
  previewGroup,
  previewQuestions,
  previewSection,
} from "../support/groupPreview";

const testID = "01935000-0000-7000-8000-000000000008";

async function setup(page: Page) {
  await stubApi(page, {
    ...sessionAs(adminUser),
    [`GET /teacher/tests/${testID}`]: {
      body: {
        skills: [],
        id: testID,
        title: "Đề kiểm tra ngữ liệu",
        status: "published",
        currentVersion: 1,
        nextVersion: 2,
        totalPoints: 3,
        questionCount: 3,
        audioCount: 2,
        assignments: { live: 0, scheduled: 0, closed: 0 },
        unpublishedChanges: 0,
        sections: [],
        createdAt: "2026-09-24T00:00:00Z",
        updatedAt: "2026-09-24T00:00:00Z",
      },
    },
    [`GET /teacher/tests/${testID}/versions`]: {
      body: {
        items: [2, 1].map((version) => ({
          id: `01935000-0000-7000-8000-00000000000${version}`,
          version,
          totalPoints: 3,
          questionCount: 3,
          audioCount: 2,
          manualCount: 0,
          skills: [],
          assignmentCount: 0,
          changeNote: null,
          publishedAt: "2026-09-24T00:00:00Z",
          publishedBy: "Giáo viên",
        })),
      },
    },
    [`GET /teacher/tests/${testID}/versions/2/diff`]: {
      body: {
        from: { kind: "version", version: 1, publishedAt: "2026-09-24T00:00:00Z" },
        to: { kind: "version", version: 2, publishedAt: "2026-09-24T00:00:00Z" },
        changes: [],
      },
    },
    [`GET /teacher/tests/${testID}/preview`]: (route) => {
      const version = Number(
        new URL(route.request().url()).searchParams.get("version") ?? 1,
      );
      return route.fulfill({
        json: {
          version,
          questions: previewQuestions,
          sections: [previewSection],
          groups: [{ ...previewGroup, title: `${previewGroup.title} · v${version}` }],
        },
      });
    },
  });
  await page.route("https://assets.example/**", (route) => route.abort());
  await page.goto(`/teacher/tests/${testID}`);
}

for (const width of [768, 1440]) {
  test(`the group preview draws the engine in the frame, by the frame's width, at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await setup(page);
    const frame = page.getByRole("region", { name: "Màn hình của học viên" });
    await expect(
      frame.getByText(previewQuestions[0]!.prompt, { exact: true }),
    ).toBeVisible();
    await expect(frame.getByText("Góc nhìn học viên", { exact: true })).toBeVisible();
    await expect(
      frame.getByRole("img", { name: "Thời gian làm bài được đặt khi giao bài" }),
    ).toBeVisible();
    await expect(frame.getByRole("button", { name: "Nộp bài" })).toHaveCount(0);
    expect(await page.locator(":focus").count()).toBe(0);

    await frame.getByRole("button", { name: "Câu sau", exact: true }).click();
    await frame.getByRole("button", { name: "Câu sau", exact: true }).click();
    await expect(
      frame.getByText(previewQuestions[2]!.prompt, { exact: true }),
    ).toBeVisible();
    const material = frame.getByRole("heading", {
      name: previewGroup.stimuli[0]!.title,
    });
    const handle = frame.getByRole("button", { name: "Mở ngữ liệu", exact: true });
    const wideFrame = (await frame.boundingBox())!.width >= 768;
    if (wideFrame) {
      await expect(material).toBeVisible();
      await expect(handle).toHaveCount(0);
      const gap = frame.getByRole("button", { name: "Chỗ trống 3, trống" });
      await gap.focus();
      await page.keyboard.press("Enter");
      await expect(page.locator(":focus")).toHaveAttribute(
        "id",
        `answer-question-${previewQuestions[2]!.id}`,
      );
    } else {
      await handle.click();
      await expect(material).toBeInViewport();
      await frame.getByRole("button", { name: "Đóng", exact: true }).click();
      await expect(handle).toBeFocused();
    }

    await page.getByRole("button", { name: "Điện thoại", exact: true }).click();
    const viewport = page.locator('[data-preview-viewport="phone"]');
    await expect(viewport).toHaveCSS("width", "390px");
    const overflow = await viewport.evaluate(
      (element) => element.scrollWidth > element.clientWidth + 1,
    );
    expect(overflow).toBe(false);
    await expect(
      frame.getByText(previewQuestions[2]!.prompt, { exact: true }),
    ).toBeVisible();
    await expect(material).not.toBeInViewport();
    await handle.click();
    await expect(material).toBeInViewport();
    await page.screenshot({
      path: test.info().outputPath(`group-preview-phone-${width}.png`),
    });
    await frame.getByRole("button", { name: "Đóng", exact: true }).click();
    await page.getByRole("button", { name: "Máy tính", exact: true }).click();
    await expect(page.locator('[data-preview-viewport="desktop"]')).toBeVisible();
    if (width > 1024) {
      await page.getByRole("button", { name: /^Phiên bản 2/ }).click();
      await expect(
        frame.getByText(previewQuestions[0]!.prompt, { exact: true }),
      ).toBeVisible();
      await frame.getByRole("button", { name: "Câu sau", exact: true }).click();
      await expect(
        frame.getByText(`${previewGroup.title} · v2`, { exact: true }),
      ).toBeVisible();
      await expect(
        frame.getByText(`${previewGroup.title} · v1`, { exact: true }),
      ).toHaveCount(0);
    }
  });
}
