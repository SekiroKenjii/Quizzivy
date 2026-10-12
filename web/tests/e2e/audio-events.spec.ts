import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { API } from "./support/api";
import { paper, start } from "./support/engine";
import type { components } from "../../src/lib/api/schema";

type RecordedEvent = components["schemas"]["IntegrityEventInput"];

const RECORDING = fileURLToPath(
  new URL("./fixtures/unit5-listening.mp3", import.meta.url),
);
const SOURCE = "https://assets.example/unit5-listening.mp3";
const BLOCKED =
  "Trình duyệt chưa phát được âm thanh. Hãy bấm phát lại; nếu vẫn không nghe được, hãy kiểm tra quyền phát âm thanh của trang này.";

async function open(page: Page, silentForMs = 0) {
  const data = paper();
  data.questions[0] = {
    ...data.questions[0]!,
    media: {
      id: "recording",
      kind: "audio",
      url: SOURCE,
      mimeType: "audio/mpeg",
      bytes: 159_711,
      durationMs: 10_004,
      originalFilename: "unit5-listening.mp3",
      createdAt: data.serverTime,
    },
    audio: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: false },
  };
  let firstAsked: number | null = null;
  await page.route(SOURCE, async (route) => {
    firstAsked ??= Date.now();
    const silentFor = firstAsked + silentForMs - Date.now();
    if (silentFor > 0) await new Promise((resolve) => setTimeout(resolve, silentFor));
    await route.fulfill({ path: RECORDING, contentType: "audio/mpeg" });
  });
  await start(page, data);

  const events: RecordedEvent[] = [];
  let counted = 0;
  await page.route(`${API}/app/attempts/paper/answers`, async (route) => {
    const body = route.request().postDataJSON() as { events?: RecordedEvent[] };
    events.push(...(body.events ?? []));
    await route.fulfill({
      json: {
        serverTime: data.serverTime,
        savedAt: data.serverTime,
        deadlineAt: data.attempt.deadlineAt,
      },
    });
  });
  await page.route(`${API}/app/attempts/paper/audio-play`, async (route) => {
    counted += 1;
    await route.fulfill({ json: { plays: counted, maxPlays: 2 } });
  });
  return {
    audioEvents: () =>
      events
        .filter((event) => event.kind.startsWith("audio_"))
        .map((event) => [event.kind, event.questionId]),
    countedPlays: () => counted,
  };
}

test("a play that runs to the end is recorded with its end", async ({ page }) => {
  test.setTimeout(60_000);
  const recorded = await open(page);
  const play = page.getByRole("button", { name: "Phát", exact: true });

  await expect
    .poll(() =>
      page.locator("audio").evaluate((element: HTMLAudioElement) => element.readyState),
    )
    .toBeGreaterThan(0);
  await expect(page.getByText("0:00 / 0:10", { exact: true })).toBeVisible();

  await play.click();
  await expect(
    page.getByRole("button", { name: "Tạm dừng", exact: true }),
  ).toBeVisible();
  await expect(page.getByText(/^0:0[1-9] \/ 0:10$/)).toBeVisible();
  await expect(play).toBeVisible({ timeout: 15_000 });

  await page.locator("label").filter({ hasText: "Đáp án A" }).click();
  await expect.poll(recorded.audioEvents).toEqual([
    ["audio_play", "q1"],
    ["audio_ended", "q1"],
  ]);
  expect(recorded.countedPlays()).toBe(1);
});

test("a blocked play is recorded", async ({ page }) => {
  await page.addInitScript(() => {
    HTMLMediaElement.prototype.play = () =>
      Promise.reject(new DOMException("blocked", "NotAllowedError"));
  });
  const recorded = await open(page);

  await page.getByRole("button", { name: "Phát", exact: true }).click();
  await page.locator("label").filter({ hasText: "Đáp án A" }).click();

  await expect.poll(recorded.audioEvents).toEqual([
    ["audio_play", "q1"],
    ["audio_blocked", "q1"],
  ]);
});

test("a pause before the audio has started shows no error", async ({ page }) => {
  const recorded = await open(page, 3_000);
  const play = page.getByRole("button", { name: "Phát", exact: true });
  const recordingArrivedOrPlayerGone = () =>
    page.evaluate(() => {
      const element = document.querySelector("audio");
      return element === null || element.readyState > 0;
    });

  await play.click();
  await page.getByRole("button", { name: /^(Phát|Tạm dừng)$/ }).click();
  await expect.poll(recordingArrivedOrPlayerGone, { timeout: 10_000 }).toBe(true);

  await expect(page.getByText("hết hạn")).toHaveCount(0);
  await expect(play).toBeVisible();

  await page.locator("label").filter({ hasText: "Đáp án A" }).click();
  await expect.poll(recorded.audioEvents).toEqual([["audio_play", "q1"]]);
});

test("a blocked play says so and keeps the player", async ({ page }) => {
  await page.addInitScript(() => {
    HTMLMediaElement.prototype.play = () =>
      Promise.reject(new DOMException("blocked", "NotAllowedError"));
  });
  await open(page);
  const play = page.getByRole("button", { name: "Phát", exact: true });
  const retry = page.getByRole("button", { name: "Thử lại", exact: true });

  await play.click();

  await expect(page.getByText(BLOCKED, { exact: true })).toBeVisible();
  await expect(play).toBeVisible();
  await expect(retry).toHaveCount(0);
});
