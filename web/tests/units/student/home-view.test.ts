import { describe, expect, it } from "vitest";
import type { StudentAssignmentCard } from "@/features/assignments/api";
import {
  greetingPeriod,
  homeView,
  justSubmitted,
  minutesLeft,
  nextByClass,
} from "@/features/assignments/studentHome";

const NOW = new Date("2026-08-29T10:00:00.000Z");

function id(n: number) {
  return `018f0000-0000-7000-8000-${String(n).padStart(12, "0")}`;
}

function paper(n: number, over: Partial<StudentAssignmentCard> = {}) {
  return {
    id: id(n),
    testTitle: `Paper ${n}`,
    status: "open",
    opensAt: "2026-08-20T01:00:00Z",
    closesAt: "2026-09-05T00:00:00Z",
    durationMinutes: 45,
    questionCount: 24,
    totalPoints: 30,
    attemptsUsed: 0,
    maxAttempts: 2,
    hasLiveAttempt: false,
    ...over,
  } satisfies StudentAssignmentCard as StudentAssignmentCard;
}

function live(n: number, over: Partial<StudentAssignmentCard> = {}) {
  return paper(n, {
    hasLiveAttempt: true,
    lastAttemptId: id(900 + n),
    liveDeadlineAt: "2026-08-29T10:38:12Z",
    liveAnsweredCount: 3,
    ...over,
  });
}

function done(n: number, over: Partial<StudentAssignmentCard> = {}) {
  return paper(n, {
    status: "closed",
    closesAt: "2026-08-28T00:00:00Z",
    attemptsUsed: 1,
    maxAttempts: 1,
    lastAttemptId: id(900 + n),
    lastSubmittedAt: "2026-08-27T03:00:00Z",
    ...over,
  });
}

function view(
  lists: Partial<Record<"dueNow" | "upcoming" | "completed", StudentAssignmentCard[]>>,
  now = NOW,
) {
  return homeView({ dueNow: [], upcoming: [], completed: [], ...lists }, now);
}

function pillOf(card: StudentAssignmentCard, now = NOW) {
  const list = card.status === "scheduled" ? "upcoming" : "dueNow";
  return view({ [list]: [card] }, now).rows.map((r) => r.pill);
}

describe("the pill on an open paper", () => {
  it.each([
    ["the last millisecond of today", "2026-08-29T16:59:59.999Z", "dueToday"],
    ["midnight", "2026-08-29T17:00:00.000Z", "dueTomorrow"],
    ["exactly 24 hours away", "2026-08-30T10:00:00.000Z", "dueTomorrow"],
    ["a millisecond past 24 hours", "2026-08-30T10:00:00.001Z", "open"],
    ["tomorrow here, today in UTC", "2026-08-29T23:30:00.000Z", "dueTomorrow"],
  ] as const)("closing at %s reads %s", (_, closesAt, pill) => {
    expect(pillOf(paper(1, { closesAt }))).toEqual([pill]);
  });

  it("reads due today after midnight for a close later that night", () => {
    expect(
      pillOf(
        paper(1, { closesAt: "2026-08-29T20:00:00Z" }),
        new Date("2026-08-29T18:00:00Z"),
      ),
    ).toEqual(["dueToday"]);
  });

  it("leaves out a paper whose window has closed on this clock", () => {
    expect(pillOf(paper(1, { closesAt: "2026-08-29T10:00:00.000Z" }))).toEqual([]);
    expect(pillOf(paper(1, { closesAt: "2026-08-29T10:00:00.001Z" }))).toEqual([
      "dueToday",
    ]);
  });

  it("shows the closing day on the tile", () => {
    const [row] = view({
      dueNow: [paper(1, { closesAt: "2026-08-31T15:00:00Z" })],
    }).rows;
    expect(row?.moment).toBe("2026-08-31T15:00:00Z");
  });

  it("reads as open when the server says so, though its opening is ahead on this clock", () => {
    expect(pillOf(paper(1, { opensAt: "2026-08-29T10:05:00Z" }))).toEqual(["open"]);
  });

  it("keeps a paper with an attempt left as still to do", () => {
    expect(
      pillOf(
        paper(1, {
          attemptsUsed: 1,
          lastAttemptId: id(901),
          lastSubmittedAt: "2026-08-28T03:00:00Z",
        }),
      ),
    ).toEqual(["open"]);
  });
});

describe("the pill on a paper not yet open", () => {
  it("says when it opens and shows that day on the tile", () => {
    const [row] = view({
      upcoming: [paper(1, { status: "scheduled", opensAt: "2026-09-01T01:00:00Z" })],
    }).rows;
    expect(row?.pill).toBe("opens");
    expect(row?.moment).toBe("2026-09-01T01:00:00Z");
  });

  it("reads as open once its opening has passed on this clock", () => {
    const scheduled = paper(1, {
      status: "scheduled",
      opensAt: "2026-08-29T09:00:00Z",
      closesAt: "2026-09-05T00:00:00Z",
    });
    expect(pillOf(scheduled)).toEqual(["open"]);
    expect(
      pillOf(paper(1, { ...scheduled, opensAt: "2026-08-29T10:00:00.001Z" })),
    ).toEqual(["opens"]);
  });

  it("is open at the millisecond it opens", () => {
    expect(
      pillOf(paper(1, { status: "scheduled", opensAt: NOW.toISOString() })),
    ).toEqual(["open"]);
  });
});

describe("the resume card", () => {
  it("is the live attempt whose deadline comes first", () => {
    const later = live(1, { liveDeadlineAt: "2026-08-29T10:45:00Z" });
    const sooner = live(2, { liveDeadlineAt: "2026-08-29T10:15:00Z" });
    const v = view({ dueNow: [later, sooner] });
    expect(v.resume).toBe(sooner);
    expect(v.rows.map((r) => [r.card, r.pill, r.moment])).toEqual([
      [later, "inProgress", "2026-08-29T10:45:00Z"],
    ]);
  });

  it("falls back to the window's close without a deadline, then to the id", () => {
    const noDeadline = live(1, {
      liveDeadlineAt: null,
      closesAt: "2026-08-29T10:10:00Z",
    });
    const timed = live(2, { liveDeadlineAt: "2026-08-29T10:15:00Z" });
    expect(view({ dueNow: [timed, noDeadline] }).resume).toBe(noDeadline);

    const twin = live(3, { liveDeadlineAt: "2026-08-29T10:15:00Z" });
    expect(view({ dueNow: [twin, timed] }).resume).toBe(timed);
  });

  it("is the live attempt even when another paper closes sooner", () => {
    const waiting = paper(1, { closesAt: "2026-08-29T10:05:00Z" });
    const taking = live(2);
    const v = view({ dueNow: [waiting, taking] });
    expect(v.resume).toBe(taking);
    expect(v.rows.map((r) => r.card)).toEqual([waiting]);
  });

  it("is absent without a live attempt", () => {
    expect(view({ dueNow: [paper(1)] }).resume).toBeNull();
  });

  it("keeps a second live attempt whose window has closed on this clock", () => {
    const v = view({
      dueNow: [
        live(1, { liveDeadlineAt: "2026-08-29T09:50:00Z" }),
        live(2, {
          liveDeadlineAt: "2026-08-29T09:55:00Z",
          closesAt: "2026-08-29T09:55:00Z",
        }),
      ],
    });
    expect(v.rows.map((r) => r.pill)).toEqual(["inProgress"]);
  });

  it("keeps a second live attempt in progress when its deadline is within the day", () => {
    const v = view({
      dueNow: [
        live(1, { liveDeadlineAt: "2026-08-29T10:15:00Z" }),
        live(2, { liveDeadlineAt: "2026-08-29T10:20:00Z" }),
      ],
    });
    expect(v.rows.map((r) => r.pill)).toEqual(["inProgress"]);
  });
});

describe("coming up", () => {
  it("lists rows by the date on their tiles, then by id", () => {
    const opensTuesday = paper(1, {
      status: "scheduled",
      opensAt: "2026-09-01T01:00:00Z",
    });
    const closesMonday = paper(2, { closesAt: "2026-08-31T15:00:00Z" });
    const closesSunday = paper(3, { closesAt: "2026-08-30T01:00:00Z" });
    const twin = paper(4, { closesAt: "2026-08-31T15:00:00Z" });
    const v = view({
      upcoming: [opensTuesday],
      dueNow: [twin, closesMonday, closesSunday],
    });
    expect(v.rows.map((r) => r.card)).toEqual([
      closesSunday,
      closesMonday,
      twin,
      opensTuesday,
    ]);
  });

  it("does not count the paper on the resume card", () => {
    expect(view({ dueNow: [live(1), paper(2), paper(3)] }).rows).toHaveLength(2);
  });
});

describe("recent results", () => {
  it("are every finished paper, the one submitted last first", () => {
    const v = view({
      completed: [
        done(1, { lastSubmittedAt: "2026-08-25T03:00:00Z" }),
        done(2, { lastSubmittedAt: "2026-08-28T03:00:00Z" }),
        done(3, { lastSubmittedAt: "2026-08-26T03:00:00Z" }),
        done(5, { lastSubmittedAt: "2026-08-27T03:00:00Z" }),
      ],
    });
    expect(v.results.map((r) => r.card.id)).toEqual([id(2), id(5), id(3), id(1)]);
  });

  it("put an attempt the server has not closed yet first, so its result can be opened", () => {
    const v = view({
      completed: [
        done(3, { lastSubmittedAt: "2026-08-28T03:00:00Z" }),
        done(4, { lastSubmittedAt: "2026-08-27T03:00:00Z" }),
        done(2, { lastSubmittedAt: null }),
        done(1, { lastSubmittedAt: null }),
      ],
    });
    expect(v.results.map((r) => r.card.id)).toEqual([id(1), id(2), id(3), id(4)]);
  });

  it("include a submitted paper the teacher has since moved to open later", () => {
    const moved = done(1, { status: "scheduled", opensAt: "2026-09-01T01:00:00Z" });
    const v = view({ upcoming: [moved] });
    expect(v.results.map((r) => r.card)).toEqual([moved]);
    expect(v.rows.map((r) => r.pill)).toEqual([]);
  });

  it("include a paper with a retake left, and no paper being taken or never tried", () => {
    const retake = paper(1, {
      attemptsUsed: 1,
      lastAttemptId: id(901),
      lastSubmittedAt: "2026-08-28T03:00:00Z",
    });
    const v = view({ dueNow: [retake, live(2), paper(3)], completed: [done(4)] });
    expect(v.results.map((r) => r.card)).toEqual([retake, v.results[1]?.card]);
    expect(v.results[1]?.card.id).toBe(id(4));
    expect(v.rows.map((r) => r.card.id)).toEqual([id(1), id(3)]);
  });

  it.each([
    [{ earned: 27, total: 30, pendingManual: 0 }, "score"],
    [{ earned: 20, total: 30, pendingManual: 2 }, "grading"],
    [null, "submitted"],
    [undefined, "submitted"],
  ] as const)("read %j as %s", (score, outcome) => {
    const card = done(1, score === undefined ? {} : { score });
    expect(view({ completed: [card] }).results[0]?.outcome).toBe(outcome);
  });
});

describe("a result just submitted", () => {
  it.each([
    [-59_999, true],
    [-60_000, false],
    [5_000, true],
  ])("at %d ms from now is fresh: %s", (offset, fresh) => {
    const lastSubmittedAt = new Date(NOW.getTime() + offset).toISOString();
    expect(justSubmitted(done(1, { lastSubmittedAt }), NOW)).toBe(fresh);
  });

  it("is never fresh without a submission time", () => {
    expect(justSubmitted(done(1, { lastSubmittedAt: null }), NOW)).toBe(false);
  });
});

describe("the greeting", () => {
  it.each([
    ["2026-08-28T21:59:59Z", "evening"],
    ["2026-08-28T22:00:00Z", "morning"],
    ["2026-08-29T04:59:59Z", "morning"],
    ["2026-08-29T05:00:00Z", "afternoon"],
    ["2026-08-29T10:59:59Z", "afternoon"],
    ["2026-08-29T11:00:00Z", "evening"],
    ["2026-08-29T17:00:00Z", "evening"],
  ] as const)("at %s is %s", (at, period) => {
    expect(greetingPeriod(new Date(at))).toBe(period);
  });
});

describe("the minutes left", () => {
  it.each([
    [38 * 60_000 + 12_000, 38],
    [59_000, 1],
    [-5_000, 1],
    [95 * 60_000, 95],
    [60_000, 1],
    [120_000 - 1, 1],
    [120_000, 2],
  ])("for %d ms is %d", (ms, minutes) => {
    const deadline = new Date(NOW.getTime() + ms).toISOString();
    expect(minutesLeft(deadline, NOW)).toBe(minutes);
  });
});

describe("the line under the greeting", () => {
  it("is about the live attempt and its deadline", () => {
    expect(view({ dueNow: [live(1), paper(2)] }).sub).toEqual({
      kind: "live",
      closes: "2026-08-29T10:38:12Z",
    });
  });

  it("is about the live attempt even when another paper closes today", () => {
    expect(
      view({ dueNow: [live(1), paper(2, { closesAt: "2026-08-29T14:00:00Z" })] }).sub,
    ).toEqual({ kind: "live", closes: "2026-08-29T10:38:12Z" });
  });

  it("counts the papers closing today when nothing is live", () => {
    expect(
      view({
        dueNow: [
          paper(1, { closesAt: "2026-08-29T14:00:00Z" }),
          paper(2, { closesAt: "2026-08-29T16:00:00Z" }),
          paper(3, { closesAt: "2026-08-29T18:00:00Z" }),
        ],
      }).sub,
    ).toEqual({ kind: "dueToday", count: 2 });
  });

  it("names the next paper when nothing closes today", () => {
    expect(
      view({
        dueNow: [paper(1, { testTitle: "Later", closesAt: "2026-09-03T01:00:00Z" })],
        upcoming: [
          paper(2, {
            testTitle: "Sooner",
            status: "scheduled",
            opensAt: "2026-09-01T01:00:00Z",
          }),
        ],
      }).sub,
    ).toEqual({ kind: "next", title: "Sooner", moment: "2026-09-01T01:00:00Z" });
  });

  it("counts a paper that opens and closes later today as due today", () => {
    const window = { status: "scheduled", opensAt: "2026-08-29T11:00:00Z" } as const;
    expect(
      view({ upcoming: [paper(1, { ...window, closesAt: "2026-08-29T13:00:00Z" })] })
        .sub,
    ).toEqual({ kind: "dueToday", count: 1 });
    expect(
      view({ upcoming: [paper(1, { ...window, closesAt: "2026-08-29T17:00:00Z" })] })
        .sub,
    ).toEqual({ kind: "next", title: "Paper 1", moment: "2026-08-29T11:00:00Z" });
  });

  it("says nothing is due when only results are left", () => {
    expect(view({ completed: [done(1)] }).sub).toEqual({ kind: "nothing" });
  });

  it("is absent when the only paper has closed on this clock, as a fresh fetch would say", () => {
    expect(view({ dueNow: [paper(1, { closesAt: NOW.toISOString() })] }).sub).toEqual({
      kind: "none",
    });
  });

  it("is absent for a student with nothing assigned", () => {
    expect(view({}).sub).toEqual({ kind: "none" });
  });
});

describe("the paper a class names as next", () => {
  const A = id(701);
  const B = id(702);
  const next = (
    lists: Partial<
      Record<"dueNow" | "upcoming" | "completed", StudentAssignmentCard[]>
    >,
  ) =>
    [...nextByClass({ dueNow: [], upcoming: [], completed: [], ...lists }, NOW)].map(
      ([classId, row]) => [classId, row.card.id, row.pill],
    );

  it("is the attempt in progress before any other paper of the class", () => {
    expect(
      next({
        dueNow: [
          paper(1, { classId: A, closesAt: "2026-08-29T10:05:00Z" }),
          live(2, { classId: A }),
        ],
      }),
    ).toEqual([[A, id(2), "inProgress"]]);
  });

  it("is the earliest paper by the date on its tile, one for each class", () => {
    expect(
      next({
        upcoming: [
          paper(1, {
            classId: A,
            status: "scheduled",
            opensAt: "2026-08-30T01:00:00Z",
          }),
        ],
        dueNow: [
          paper(2, { classId: A, closesAt: "2026-09-03T01:00:00Z" }),
          paper(3, { classId: B, closesAt: "2026-08-29T14:00:00Z" }),
          paper(4, { classId: B, closesAt: "2026-08-29T12:00:00Z" }),
        ],
      }),
    ).toEqual([
      [B, id(4), "dueToday"],
      [A, id(1), "opens"],
    ]);
  });

  it("leaves out a paper closed on this clock, a finished one and one shared by two classes", () => {
    expect(
      next({
        dueNow: [
          paper(1, { classId: A, closesAt: NOW.toISOString() }),
          paper(2, { classId: null }),
          paper(3, {}),
        ],
        completed: [done(4, { classId: A })],
      }),
    ).toEqual([]);
  });

  it("is on both cards when two of the student's classes share it", () => {
    expect(next({ dueNow: [paper(1, { classId: null, classIds: [A, B] })] })).toEqual([
      [A, id(1), "open"],
      [B, id(1), "open"],
    ]);
  });

  it("is the first paper of each class when a shared one comes later", () => {
    expect(
      next({
        dueNow: [
          paper(1, { classId: null, classIds: [A, B] }),
          paper(2, { classId: A, classIds: [A], closesAt: "2026-09-03T01:00:00Z" }),
        ],
      }),
    ).toEqual([
      [A, id(2), "open"],
      [B, id(1), "open"],
    ]);
  });

  it("still reads classId from a server that sends no classIds", () => {
    expect(next({ dueNow: [paper(1, { classId: A })] })).toEqual([[A, id(1), "open"]]);
  });
});
