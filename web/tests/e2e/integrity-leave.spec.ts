import { expect, test, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";
import { paper, start } from "./support/engine";
import { assignment, classes } from "./support/student";

interface Flushed {
  sessionId: string;
  events?: { kind: string; clientSeq: number; meta?: { awayMs?: number } }[];
}

async function leaveTheWindowAndComeBack(page: Page) {
  await page.evaluate(async () => {
    window.dispatchEvent(new Event("blur"));
    await new Promise((resolve) => setTimeout(resolve, 3500));
    window.dispatchEvent(new Event("focus"));
  });
  await page.getByRole("button", { name: "Quay lại bài làm" }).click();
}

function beaconBodies(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    Promise.all(
      (window as unknown as { beaconsSent: Blob[] }).beaconsSent.map((body) =>
        body.text(),
      ),
    ),
  );
}

test("an absence recorded before Leave reaches the server, and a return by Back numbers on", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const beaconsSent: Blob[] = [];
    const send = navigator.sendBeacon.bind(navigator);
    navigator.sendBeacon = (url, data) => {
      if (data instanceof Blob) beaconsSent.push(data);
      return send(url, data);
    };
    Object.assign(window, { beaconsSent });
  });
  const data = paper();
  await start(page, data);

  let arrived = 0;
  const saves: Flushed[] = [];
  await stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/attempts/paper": { body: data },
    "PATCH /app/attempts/paper/answers": async (route) => {
      saves.push(route.request().postDataJSON() as Flushed);
      await route.fulfill({
        json: {
          serverTime: data.serverTime,
          savedAt: data.serverTime,
          deadlineAt: data.attempt.deadlineAt,
        },
      });
    },
    "POST /app/attempts/paper/events": async (route) => {
      arrived += 1;
      await route.fulfill({ status: 202 });
    },
    "GET /app/classes": { body: { items: classes } },
    "GET /app/assignments": {
      body: { dueNow: [assignment(1, true)], upcoming: [], completed: [] },
    },
  });

  await leaveTheWindowAndComeBack(page);
  await page.getByRole("button", { name: "Thoát khỏi bài làm", exact: true }).click();
  await page
    .getByRole("dialog", { name: "Thoát khỏi bài làm?" })
    .getByRole("button", { name: "Thoát", exact: true })
    .click();
  await expect(page.getByRole("button", { name: "Tiếp tục làm bài" })).toBeVisible();

  await expect.poll(() => arrived).toBe(1);
  const bodies = await beaconBodies(page);
  expect(bodies).toHaveLength(1);
  const beacon = JSON.parse(bodies[0]!) as Flushed & { beaconToken: string };
  expect(beacon.sessionId).toBe("session");
  expect(beacon.beaconToken).toBe("beacon");
  expect(beacon.events?.map((event) => event.kind)).toEqual([
    "window_blur",
    "window_focus",
  ]);
  for (const event of beacon.events!) {
    expect(Number.isInteger(event.clientSeq)).toBe(true);
    expect(event.clientSeq).toBeLessThan(2_147_483_647);
  }
  expect(
    beacon.events![1]!.clientSeq - beacon.events![0]!.clientSeq,
  ).toBeGreaterThanOrEqual(3000);
  expect(beacon.events?.[1]?.meta?.awayMs).toBeGreaterThanOrEqual(3000);
  expect(saves).toEqual([]);

  await page.goBack();
  await expect(page.getByRole("radio", { name: "Đáp án A" })).toBeVisible();
  await leaveTheWindowAndComeBack(page);
  await page.locator("label").filter({ hasText: "Đáp án A" }).click();

  await expect.poll(() => saves.length).toBeGreaterThan(0);
  expect(saves[0]?.sessionId).toBe("session");
  expect(saves[0]?.events?.map((event) => event.kind)).toEqual([
    "window_blur",
    "window_focus",
  ]);
  expect(saves[0]!.events![0]!.clientSeq).toBeGreaterThan(beacon.events![1]!.clientSeq);
  expect(saves[0]!.events![1]!.clientSeq).toBeGreaterThan(
    saves[0]!.events![0]!.clientSeq,
  );
  expect(arrived).toBe(1);
});
