import { fileURLToPath } from "node:url";
import { expect, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./api";
import {
  previewGroup,
  previewQuestions,
  previewSection,
} from "../../support/groupPreview";
import type { components } from "../../../src/lib/api/schema";

export type Session = components["schemas"]["AttemptSession"];

/** paper is an attempt in progress on three questions: one choice, one fill-in, one short answer. */
export function paper(): Session {
  const now = new Date().toISOString();
  return {
    attempt: {
      id: "paper",
      assignmentId: "assignment",
      studentId: studentUser.id,
      testVersionId: "version",
      attemptNo: 1,
      status: "in_progress",
      startedAt: now,
      deadlineAt: new Date(Date.now() + 3600000).toISOString(),
    },
    testTitle: "Bài kiểm tra thao tác",
    sections: [{ id: "part", title: "Phần 1", instructions: null }],
    questions: [
      {
        id: "q1",
        sectionId: "part",
        type: "single_choice",
        prompt: "Chọn đáp án",
        points: 1,
        options: [
          { id: "a", text: "Đáp án A" },
          { id: "b", text: "Đáp án B" },
        ],
      },
      {
        id: "q2",
        sectionId: "part",
        type: "fill_blank",
        prompt: "She {{1}} yesterday.",
        points: 1,
        blanks: [{ id: "blank", ordinal: 1, caseSensitive: false }],
      },
      {
        id: "q3",
        sectionId: "part",
        type: "short_answer",
        prompt: "Viết một câu",
        points: 1,
      },
    ],
    sessionId: "session",
    beaconToken: "beacon",
    serverTime: now,
    audioPlays: {},
    answers: {},
    remainingAttempts: 1,
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: false,
      maxFocusLoss: 0,
      onLimitExceeded: "warn",
      minAwayMs: 3000,
    },
  };
}

/** start opens the engine on a paper whose first question is the choice between "Đáp án A" and "Đáp án B". */
export async function start(page: Page, data: Session = paper()) {
  await stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/attempts/paper": { body: data },
    "PATCH /app/attempts/paper/answers": {
      body: {
        serverTime: data.serverTime,
        savedAt: data.serverTime,
        deadlineAt: data.attempt.deadlineAt,
      },
    },
    "POST /app/attempts/paper/events": { body: {} },
  });
  await page.goto("/app/attempts/paper");
  await expect(page.getByRole("radio", { name: "Đáp án A" })).toBeVisible();
}

const groupAttemptId = "01935000-0000-7000-8000-000000000088";
const audio = fileURLToPath(
  new URL("../fixtures/unit5-listening.mp3", import.meta.url),
);
const recordingId = previewGroup.recordings[0]!.id;

/**
 * startGroupPaper opens the engine on two questions that share a passage and
 * a recording, and returns the play ids the server has seen. The first play
 * request is lost on the way back.
 */
export async function startGroupPaper(page: Page) {
  const seen = new Set<string>();
  let loseFirstResponse = true;
  const now = new Date().toISOString();
  const payload: Session = {
    attempt: {
      id: groupAttemptId,
      assignmentId: groupAttemptId,
      studentId: studentUser.id,
      testVersionId: groupAttemptId,
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
    sessionId: groupAttemptId,
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
    [`GET /app/attempts/${groupAttemptId}`]: (route) =>
      route.fulfill({
        json: { ...payload, groupAudioPlays: { [recordingId]: seen.size } },
      }),
    [`PATCH /app/attempts/${groupAttemptId}/answers`]: {
      body: { savedAt: now, serverTime: now, deadlineAt: payload.attempt.deadlineAt },
    },
    [`POST /app/attempts/${groupAttemptId}/group-audio-play`]: async (route) => {
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
  await page.goto(`/app/attempts/${groupAttemptId}`);
  return seen;
}

/**
 * engineFits waits until nothing in the engine scrolls sideways: the
 * document, the deck surface, `<main>` and every scroller inside it, which is
 * where a pane's overflow goes, and an open dialog with every scroller inside
 * it, which the document cannot show because a dialog is fixed. A rich table's
 * own scroll box is left out, because scrolling there is how a wide table is
 * read.
 */
export async function engineFits(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(() => {
        const scrolls = (node: Element) =>
          /auto|scroll/.test(getComputedStyle(node).overflowX) &&
          node.closest(".content-table-scroll") === null;
        return [
          document.documentElement,
          document.querySelector("[data-scale='deck']"),
          document.querySelector("main"),
          document.querySelector("[role='dialog']"),
          ...[...document.querySelectorAll("main *, [role='dialog'] *")].filter(
            scrolls,
          ),
        ].every((node) => node === null || node.scrollWidth <= node.clientWidth + 1);
      }),
    )
    .toBe(true);
}
