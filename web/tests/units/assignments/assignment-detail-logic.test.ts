import { describe, expect, it } from "vitest";
import {
  assignmentStats,
  detailTab,
  elapsedMinutes,
  orderedRoster,
  pendingAnswers,
  reviewMatches,
  rosterFilter,
  rosterMatches,
} from "@/features/assignments/pages/teacher/assignmentDetail";
import { assignmentDetailLocation } from "@/features/assignments/pages/teacher/assignmentDetailUrl";
import { assignmentStudentsLocation } from "@/app/legacyTeacherPath";
import { defaultRows, review } from "../attempts/fixtures";

const rows = defaultRows();

describe("assignment detail pure contracts", () => {
  it("counts handed-in and real flags independently of filtering and excludes pending scores from its graded mean", () => {
    const all = [
      ...rows,
      {
        ...rows[1]!,
        studentId: "graded",
        state: "graded" as const,
        score: { earned: 0, total: 30, pendingManual: 0 },
      },
      {
        ...rows[1]!,
        studentId: "graded2",
        state: "graded" as const,
        score: { earned: 9, total: 10, pendingManual: 0 },
      },
      { ...rows[1]!, studentId: "timeout", state: "timed_out" as const },
      {
        ...rows[1]!,
        studentId: "void",
        state: "voided" as const,
        flagged: true,
        score: { earned: 20, total: 30, pendingManual: 4 },
      },
    ];
    expect(assignmentStats(all)).toEqual({
      total: 7,
      submitted: 4,
      inProgress: 1,
      flagged: 2,
      pending: 4,
      average: 45,
    });
    expect(all.filter((row) => rosterMatches(row, "pending", ""))).toHaveLength(2);
    expect(assignmentStats(all).total).toBe(7);
  });

  it("leaves an average absent for null or nonpositive denominators and preserves zero scores", () => {
    expect(assignmentStats([]).average).toBeNull();
    expect(
      assignmentStats([
        { ...rows[0]!, state: "graded", score: null },
        {
          ...rows[1]!,
          state: "graded",
          score: { earned: 0, total: 0, pendingManual: 0 },
        },
      ]).average,
    ).toBeNull();
    expect(
      assignmentStats([
        {
          ...rows[1]!,
          state: "graded",
          score: { earned: 0, total: 30, pendingManual: 0 },
        },
      ]).average,
    ).toBe(0);
  });

  it("never grades in-progress or voided papers even if a score reports pending answers", () => {
    expect(pendingAnswers({ ...rows[1]!, state: "in_progress" })).toBe(0);
    expect(pendingAnswers({ ...rows[1]!, state: "voided" })).toBe(0);
    expect(pendingAnswers({ ...rows[1]!, state: "timed_out" })).toBe(2);
  });

  it("preserves paper filters, Vietnamese search and deterministic identity tie-breaks", () => {
    expect(rosterMatches(rows[0]!, "flagged", "pham gia han")).toBe(true);
    expect(rosterMatches({ ...rows[0]!, state: "voided" }, "flagged", "")).toBe(false);
    expect(rosterMatches(rows[2]!, "notStarted", "")).toBe(true);
    expect(rosterMatches(rows[0]!, "submitted", "")).toBe(false);
    expect(
      orderedRoster([
        { ...rows[0]!, studentId: "b" },
        { ...rows[0]!, studentId: "a" },
      ]).map((row) => row.studentId),
    ).toEqual(["a", "b"]);
    expect(rows[0]!.studentId).not.toBe("a");
  });

  it("uses actual elapsed timestamps without inventing a zero or focus duration", () => {
    expect(elapsedMinutes({ startedAt: null, submittedAt: null })).toBeNull();
    expect(
      elapsedMinutes({
        startedAt: "2026-09-04T00:00:00Z",
        submittedAt: "2026-09-04T00:00:20Z",
      }),
    ).toBe(0);
    expect(
      elapsedMinutes({
        startedAt: "2026-09-04T00:00:00Z",
        submittedAt: "2026-09-04T00:41:00Z",
      }),
    ).toBe(41);
  });

  it("requires exact assignment, attempt and student identity before accepting private review data", () => {
    const data = review();
    expect(reviewMatches(data, data.attempt.assignmentId, data.attempt.id)).toBe(true);
    expect(reviewMatches(data, "wrong", data.attempt.id)).toBe(false);
    expect(reviewMatches(data, data.attempt.assignmentId, "wrong")).toBe(false);
    expect(
      reviewMatches(
        { ...data, student: { ...data.student, id: "wrong" } },
        data.attempt.assignmentId,
        data.attempt.id,
      ),
    ).toBe(false);
    expect(reviewMatches(undefined, data.attempt.assignmentId, data.attempt.id)).toBe(
      false,
    );
  });

  it("separates tabs from roster filters and falls back safely for unsupported values", () => {
    expect(detailTab("pending")).toBe("students");
    expect(detailTab("questions")).toBe("questions");
    expect(detailTab("settings")).toBe("settings");
    expect(rosterFilter("flagged")).toBe("flagged");
    expect(rosterFilter("settings")).toBe("all");
  });

  it("canonicalizes both papers paths and retains filters, duplicates, attempt, paging and hash", () => {
    for (const pathname of [
      "/teacher/assignments/id/attempts",
      "/AdMiN/AsSiGnMeNtS/id/AtTeMpTs",
    ]) {
      const next = assignmentStudentsLocation({
        pathname,
        search: "?tab=pending&q=H%C3%A2n&page=3&size=20&attempt=A&other=one&other=two",
        hash: "#kept",
      });
      expect(next?.pathname).toBe("/teacher/assignments/id");
      expect(next?.hash).toBe("#kept");
      const params = new URLSearchParams(next?.search);
      expect(params.get("tab")).toBe("students");
      expect(params.get("roster")).toBe("pending");
      expect(params.get("attempt")).toBe("A");
      expect(params.get("q")).toBe("Hân");
      expect(params.get("page")).toBe("3");
      expect(params.get("size")).toBe("20");
      expect(params.getAll("other")).toEqual(["one", "two"]);
    }
    expect(
      assignmentStudentsLocation({
        pathname: "/teacher/tests/id",
        search: "",
        hash: "",
      }),
    ).toBeNull();
  });

  it("ordinary query changes preserve unrelated values and remove only attempt when closing", () => {
    const location = {
      pathname: "/teacher/assignments/id",
      search:
        "?tab=settings&attempt=A&q=H%C3%A2n&roster=flagged&page=2&size=20&x=1&x=2",
      hash: "#kept",
    };
    const next = assignmentDetailLocation(location, { attempt: null });
    expect(next.pathname).toBe(location.pathname);
    expect(next.hash).toBe("#kept");
    expect(new URLSearchParams(next.search).has("attempt")).toBe(false);
    expect(new URLSearchParams(next.search).getAll("x")).toEqual(["1", "2"]);
    expect(new URLSearchParams(next.search).get("page")).toBe("2");
    expect(new URLSearchParams(next.search).get("tab")).toBe("settings");
  });
});
