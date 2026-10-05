import { describe, expect, it } from "vitest";
import {
  CircleCheck,
  ClipboardList,
  Clock,
  FileCheck,
  Flag,
  RotateCw,
  SquarePen,
  UserPlus,
} from "lucide-react";
import i18n from "@/lib/i18n";
import { loadSpec } from "@tests/support/openapi";
import {
  KIND_LOOK,
  TEACHER_EVENTS,
  STUDENT_EVENTS,
  audienceOf,
  describeNotification,
  notificationPath,
} from "@/features/notifications/kinds";
import type {
  Notification,
  NotificationParams,
  NotificationTarget,
} from "@/features/notifications/api";
import { assignmentId, attemptId, notices, title, listResponse } from "./support";

const expected = [
  [
    "6 bài nộp của Mid-term Reading Mock đang chờ chấm",
    "6 papers in Mid-term Reading Mock need grading",
    SquarePen,
    "warning",
  ],
  [
    "Lê Hoàng Nam đã rời trang làm bài 3 lần",
    "Lê Hoàng Nam left the test 3 times",
    Flag,
    "danger",
  ],
  [
    "Mid-term Reading Mock sắp đóng, còn 4 học viên chưa nộp",
    "Mid-term Reading Mock closes soon and4 students have not handed in".replace(
      "and4",
      "and 4",
    ),
    Clock,
    "warning",
  ],
  [
    "Nguyễn Gia Bảo đã vào lớp IELTS Foundation A",
    "Nguyễn Gia Bảo joined IELTS Foundation A",
    UserPlus,
    "neutral",
  ],
  [
    "Mã tham gia của 7 lớp đã được đổi mới: IELTS Foundation A, IELTS 6.5 Evening, IELTS 7.0 Morning, Reading A, Listening B…",
    "The join codes of 7 classes were replaced: IELTS Foundation A, IELTS 6.5 Evening, IELTS 7.0 Morning, Reading A, Listening B…",
    RotateCw,
    "info",
  ],
  [
    "Mid-term Reading Mock đã mở, đóng lúc 06:30, 02/10/2026",
    "Mid-term Reading Mock is open and closes at06:30, 02/10/2026".replace(
      "at06",
      "at 06",
    ),
    ClipboardList,
    "info",
  ],
  [
    "Reading test đóng lúc 06:30, 02/10/2026",
    "Reading test closes at 06:30, 02/10/2026",
    Clock,
    "warning",
  ],
  [
    "Mid-term Reading Mock được gia hạn đến06:30, 02/10/2026".replace(
      "đến06",
      "đến 06",
    ),
    "Mid-term Reading Mock now closes at 06:30, 02/10/2026",
    Clock,
    "info",
  ],
  [
    "Unit 3 grammar check đã có kết quả",
    "Your result for Unit 3 grammar check is ready",
    CircleCheck,
    "success",
  ],
] as const;
const pluralCases = [
  {
    index: 0,
    field: "toGrade",
    enOne: "1 paper in Mid-term Reading Mock needs grading",
    enOther: "2 papers in Mid-term Reading Mock need grading",
    vi: "{{n}} bài nộp của Mid-term Reading Mock đang chờ chấm",
  },
  {
    index: 0,
    field: "count",
    enOne: "1 paper handed in for Mid-term Reading Mock",
    enOther: "2 papers handed in for Mid-term Reading Mock",
    vi: "{{n}} bài đã nộp cho Mid-term Reading Mock",
  },
  {
    index: 1,
    field: "focusLost",
    enOne: "Lê Hoàng Nam left the test 1 time",
    enOther: "Lê Hoàng Nam left the test 2 times",
    vi: "Lê Hoàng Nam đã rời trang làm bài {{n}} lần",
  },
  {
    index: 2,
    field: "notSubmitted",
    enOne: "Mid-term Reading Mock closes soon and1 student has not handed in".replace(
      "and1",
      "and 1",
    ),
    enOther:
      "Mid-term Reading Mock closes soon and2 students have not handed in".replace(
        "and2",
        "and 2",
      ),
    vi: "Mid-term Reading Mock sắp đóng, còn {{n}} học viên chưa nộp",
  },
  {
    index: 4,
    field: "count",
    enOne: "The join code of 1 class was replaced: Lớp A",
    enOther: "The join codes of 2 classes were replaced: Lớp A…",
    vi: "Mã tham gia của {{n}} lớp đã được đổi mới: Lớp A",
  },
] as const;

describe("notification kinds and sentences", () => {
  it("matches both contract enums exhaustively and validates all nine fixtures", () => {
    const schemas = loadSpec().components.schemas;
    expect(Object.keys(KIND_LOOK)).toEqual(schemas.NotificationKind.enum);
    expect([...TEACHER_EVENTS, ...STUDENT_EVENTS]).toEqual(
      schemas.NotificationEvent.enum,
    );
    expect(TEACHER_EVENTS).toEqual(schemas.NotificationEvent.enum.slice(0, 3));
    expect(STUDENT_EVENTS).toEqual(schemas.NotificationEvent.enum.slice(3));
    expect(
      TEACHER_EVENTS.filter((event) =>
        (STUDENT_EVENTS as readonly string[]).includes(event),
      ),
    ).toEqual([]);
    expect(() => listResponse({ items: notices, nextBefore: null })).not.toThrow();
  });
  it.each(notices.map((notice, index) => ({ notice, index })))(
    "formats $notice.kind with the exact sentence, icon and tone in both languages",
    ({ notice, index }) => {
      for (const locale of ["vi", "en"] as const) {
        const view = describeNotification(notice, i18n.getFixedT(locale), locale);
        expect(view?.text).toBe(expected[index]![locale === "vi" ? 0 : 1]);
        expect(view?.icon).toBe(expected[index]![2]);
        expect(view?.tone).toBe(expected[index]![3]);
      }
      expect(audienceOf(notice.kind)).toBe(index < 5 ? "teacher" : "student");
    },
  );
  it.each(pluralCases)(
    "formats one and many from $field for fixture$index in both languages",
    (entry) => {
      for (const count of [1, 2]) {
        const notice = notices[entry.index]!;
        const params = { ...notice.params, [entry.field]: count };
        if (entry.index === 0 && entry.field === "count") params.toGrade = 0;
        if (entry.index === 4) params.classNames = ["Lớp A"];
        const n = { ...notice, params };
        expect(describeNotification(n, i18n.getFixedT("en"), "en")?.text).toBe(
          count === 1 ? entry.enOne : entry.enOther,
        );
        const ellipsis = entry.index === 4 && count === 2 ? "…" : "";
        expect(describeNotification(n, i18n.getFixedT("vi"), "vi")?.text).toBe(
          entry.vi.replace("{{n}}", String(count)) + ellipsis,
        );
      }
    },
  );
  it("uses neutral submitted look/count at zero toGrade, and preserves schema-valid zero counts", () => {
    const notice = { ...notices[0]!, params: { title, count: 0, toGrade: 0 } };
    expect(describeNotification(notice, i18n.getFixedT("en"), "en")).toEqual({
      icon: FileCheck,
      tone: "neutral",
      text: "0 papers handed in for Mid-term Reading Mock",
      to: `/teacher/assignments/${assignmentId}`,
    });
    for (const [index, field] of [
      [1, "focusLost"],
      [2, "notSubmitted"],
    ] as const)
      expect(
        describeNotification(
          { ...notices[index]!, params: { ...notices[index]!.params, [field]: 0 } },
          i18n.getFixedT("vi"),
          "vi",
        ),
      ).not.toBeNull();
    expect(
      describeNotification(
        { ...notices[4]!, params: { count: 0, classNames: [] } },
        i18n.getFixedT("en"),
        "en",
      )?.text,
    ).toBe("The join codes of 0 classes were replaced: ");
  });
  it("adds an ellipsis only when there are more classes than supplied names", () => {
    const n = { ...notices[4]!, params: { count: 3, classNames: ["A", "B", "C"] } };
    expect(describeNotification(n, i18n.getFixedT("en"), "en")?.text).toBe(
      "The join codes of 3 classes were replaced: A, B, C",
    );
    n.params.count = 7;
    expect(describeNotification(n, i18n.getFixedT("en"), "en")?.text).toMatch(
      /A, B, C…$/,
    );
  });
  it("leaves names containing markup as plain text and dates in the app zone", () => {
    const n = { ...notices[3]!, params: { studentName: "<b>Nam</b>", className: "A" } };
    expect(describeNotification(n, i18n.getFixedT("en"), "en")?.text).toBe(
      "<b>Nam</b> joined A",
    );
    expect(
      describeNotification(notices[5]!, i18n.getFixedT("en"), "en")?.text,
    ).toContain("06:30, 02/10/2026");
  });
  it.each([
    [0, "toGrade"],
    [1, "studentName"],
    [2, "notSubmitted"],
    [3, "className"],
    [4, "classNames"],
    [5, "closesAt"],
    [6, "title"],
    [7, "closesAt"],
    [8, "title"],
  ] as const)("skips fixture%s when required %s is missing", (index, field) => {
    const params: NotificationParams = { ...notices[index]!.params };
    delete params[field];
    expect(
      describeNotification({ ...notices[index]!, params }, i18n.getFixedT("vi"), "vi"),
    ).toBeNull();
  });
  it("skips unknown and prototype kind names, invalid dates and invalid counts rather than filling holes", () => {
    for (const kind of ["future.kind", "__proto__"])
      expect(
        describeNotification(
          { ...notices[0]!, kind } as Notification,
          i18n.getFixedT("en"),
          "en",
        ),
      ).toBeNull();
    expect(
      describeNotification(
        { ...notices[5]!, params: { title, closesAt: "bad" } },
        i18n.getFixedT("en"),
        "en",
      ),
    ).toBeNull();
    expect(
      describeNotification(
        { ...notices[0]!, params: { title, toGrade: 0 } },
        i18n.getFixedT("en"),
        "en",
      ),
    ).toBeNull();
    for (const toGrade of [-1, 0.5, Number.NaN])
      expect(
        describeNotification(
          { ...notices[0]!, params: { title, count: 2, toGrade } },
          i18n.getFixedT("en"),
          "en",
        ),
      ).toBeNull();
  });
  it("overrides a merged submitted target for grading only when permitted, with identical text and look", () => {
    const n = { ...notices[0]!, params: { title, count: 8, toGrade: 3 } };
    const normal = describeNotification(n, i18n.getFixedT("en"), "en")!;
    expect(normal.to).toBe("/teacher/grading");
    expect(
      describeNotification(n, i18n.getFixedT("en"), "en", { canGrade: true }),
    ).toEqual(normal);
    expect(
      describeNotification(n, i18n.getFixedT("en"), "en", { canGrade: false }),
    ).toEqual({ ...normal, to: `/teacher/assignments/${assignmentId}` });
    expect(
      describeNotification(
        { ...n, params: { title, count: 8, toGrade: 0 } },
        i18n.getFixedT("en"),
        "en",
      )?.to,
    ).toBe(`/teacher/assignments/${assignmentId}`);
    for (const notice of notices.slice(1))
      expect(
        describeNotification(notice, i18n.getFixedT("en"), "en", { canGrade: false }),
      ).toEqual(describeNotification(notice, i18n.getFixedT("en"), "en"));
  });
  it.each([
    [{ route: "assignment", assignmentId }, `/teacher/assignments/${assignmentId}`],
    [{ route: "attempt", attemptId }, `/teacher/attempts/${attemptId}`],
    [{ route: "grading" }, "/teacher/grading"],
    [{ route: "classes" }, "/teacher/classes"],
    [{ route: "result", attemptId }, `/app/attempts/${attemptId}/result`],
    [{ route: "studentAssignment", assignmentId }, `/app/assignments/${assignmentId}`],
  ] as const)("resolves a local %j target", (target, path) => {
    expect(notificationPath(target)).toBe(path);
  });
  it("refuses null, unknown routes and missing or malformed IDs", () => {
    expect(notificationPath(null)).toBeNull();
    expect(
      notificationPath({
        route: "https://evil.example",
      } as unknown as NotificationTarget),
    ).toBeNull();
    for (const route of [
      "assignment",
      "attempt",
      "result",
      "studentAssignment",
    ] as const) {
      expect(notificationPath({ route })).toBeNull();
      for (const value of ["", "..", "a/b", "https://evil.example", "id?redirect=x"])
        expect(
          notificationPath({ route, assignmentId: value, attemptId: value }),
        ).toBeNull();
    }
  });
});
