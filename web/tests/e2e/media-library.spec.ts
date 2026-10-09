import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import type { components } from "../../src/lib/api/schema";

type LibraryAsset = components["schemas"]["LibraryAsset"];

const RECORDING = fileURLToPath(
  new URL("./fixtures/unit5-listening.mp3", import.meta.url),
);
const SOURCE = "https://assets.example/unit5-listening.mp3";
const MB = 1024 * 1024;
const GIB = 1024 * MB;

function asset(index: number, overrides: Partial<LibraryAsset>): LibraryAsset {
  return {
    id: `018f0000-0000-7000-8000-0000000002${String(index).padStart(2, "0")}`,
    kind: "audio",
    url: SOURCE,
    mimeType: "audio/mpeg",
    bytes: 159_711,
    durationMs: 10_004,
    originalFilename: "unit5-listening.mp3",
    createdAt: "2026-10-01T00:00:00Z",
    displayName: "unit5-listening.mp3",
    defaultMaxPlays: 2,
    width: null,
    height: null,
    questionCount: 4,
    usageCount: 0,
    usedIn: [],
    ...overrides,
  };
}

const ITEMS = [
  asset(0, { displayName: "Cambridge 15 · Test 2 · Part 1.mp3" }),
  asset(1, {
    kind: "image",
    url: "https://assets.example/map.png",
    mimeType: "image/png",
    bytes: 240 * 1024,
    durationMs: null,
    displayName: "Bản đồ chỉ đường tới thư viện thành phố_phiên_bản_cuối_cùng.png",
    defaultMaxPlays: null,
    width: 1200,
    height: 800,
    questionCount: 0,
  }),
  asset(2, { displayName: "Podcast · city parks.mp3", defaultMaxPlays: 0 }),
];

async function prepare(page: Page, theme: "light" | "dark", width: number) {
  await page.setViewportSize({ width, height: 900 });
  await page.addInitScript((theme) => {
    localStorage.setItem("quizzivy.theme", theme);
    localStorage.setItem("quizzivy.locale", "vi");
  }, theme);
  await stubApi(page, {
    ...sessionAs({ ...adminUser, preferences: { theme } } as typeof adminUser),
    "GET /teacher/summary": {
      body: { liveAssignments: 0, answersToGrade: 0, unreadNotifications: 0 },
    },
    "GET /teacher/media": {
      body: {
        items: ITEMS,
        page: 1,
        pageSize: 24,
        total: ITEMS.length,
        totalBytes: ITEMS.reduce((sum, row) => sum + row.bytes, 0),
        facets: { all: 3, audio: 2, image: 1, unused: 1 },
        usage: {
          audioBytes: Math.round(1.05 * GIB),
          imageBytes: Math.round(0.2 * GIB),
          quotaBytes: 5 * GIB,
        },
      },
    },
  });
  const fetched = { audio: 0 };
  await page.route(SOURCE, (route) => {
    fetched.audio += 1;
    return route.fulfill({ path: RECORDING, contentType: "audio/mpeg" });
  });
  await page.route("https://assets.example/map.png", (route) =>
    route.fulfill({ status: 404, body: "" }),
  );
  return fetched;
}

for (const theme of ["light", "dark"] as const) {
  for (const width of [360, 768, 1024, 1280, 1440]) {
    test(`the Media page at ${width}px in ${theme} fits and follows the theme`, async ({
      page,
    }) => {
      await prepare(page, theme, width);
      await page.goto("/teacher/media");

      await expect(
        page.getByRole("heading", { level: 1, name: "Thư viện media" }),
      ).toBeVisible();
      await expect(page.getByText("Dùng trong 4 câu hỏi").first()).toBeVisible();
      await expect(page.locator("[data-scale='deck']").first()).toBeVisible();
      expect(
        await page.evaluate(() => document.documentElement.classList.contains("dark")),
      ).toBe(theme === "dark");
      await expect
        .poll(() =>
          page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        )
        .toBe(true);
    });
  }
}

test("a card's play button starts the recording in a real browser, one card at a time", async ({
  page,
}) => {
  const fetched = await prepare(page, "light", 1280);
  await page.goto("/teacher/media");
  await expect(page.getByText("Dùng trong 4 câu hỏi").first()).toBeVisible();
  await page.waitForLoadState("networkidle");
  expect(fetched.audio, "no recording is requested before a click").toBe(0);

  await page
    .getByRole("button", { name: "Phát Cambridge 15 · Test 2 · Part 1.mp3" })
    .click();
  await expect(
    page.getByRole("button", { name: "Tạm dừng Cambridge 15 · Test 2 · Part 1.mp3" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Phát Podcast · city parks.mp3" }).click();
  await expect(
    page.getByRole("button", { name: "Tạm dừng Podcast · city parks.mp3" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Phát Cambridge 15 · Test 2 · Part 1.mp3" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () =>
        [...document.querySelectorAll("audio")].filter((audio) => !audio.paused).length,
    ),
  ).toBe(1);
});
