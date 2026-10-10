import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import {
  DETAIL_CLASS_ID,
  detailAssignment,
  detailRow,
  renderDetail,
  serveDetail,
} from "@tests/support/assignmentDetail";
import i18n from "@/lib/i18n";

const AN = "018f0000-0000-7000-8000-0000000000e1";
const BINH = "018f0000-0000-7000-8000-0000000000e2";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

beforeEach(() => useAuthStore.getState().setSession("token", teacherUser));
afterEach(() => void i18n.changeLanguage("vi"));

function card(name: string) {
  return screen.getByRole("region", { name });
}

describe("the Settings tab", () => {
  it("locks Test & timing while live, with the reason as its tooltip, and leaves the rest editable", async () => {
    serveDetail(detailAssignment());
    renderDetail("settings");
    const timing = await screen.findByRole("region", { name: "Đề và thời gian" });
    const pill = within(timing).getByText("Đã khóa");
    expect(pill.closest("[title]")).toHaveAttribute(
      "title",
      "Bị khóa khi học viên đang làm bài",
    );
    expect(within(timing).queryByRole("button", { name: /^Sửa/ })).toBeNull();
    for (const name of [
      "Thời gian mở",
      "Tính toàn vẹn",
      "Kết quả",
      "Lời nhắn cho học viên",
    ])
      expect(
        within(card(name)).getByRole("button", { name: `Sửa ${name}` }),
      ).toBeEnabled();
  });

  it("lets Test & timing be edited while the assignment is not live", async () => {
    serveDetail(
      detailAssignment({
        status: "scheduled",
        window: {
          opensAt: "2099-09-07T01:00:00Z",
          closesAt: "2099-09-09T14:00:00Z",
          closedAt: null,
        },
      }),
    );
    renderDetail("settings");
    const timing = await screen.findByRole("region", { name: "Đề và thời gian" });
    expect(within(timing).queryByText("Đã khóa")).toBeNull();
    expect(
      within(timing).getByRole("button", { name: "Sửa Đề và thời gian" }),
    ).toBeEnabled();
  });

  it("saves one group, keeps every other value as stored, and says students see it", async () => {
    const a = detailAssignment();
    const calls = serveDetail(a);
    const user = renderDetail("settings");
    await user.click(await screen.findByRole("button", { name: "Sửa Kết quả" }));
    const dialog = await screen.findByRole("dialog", { name: "Sửa Kết quả" });
    await user.click(within(dialog).getByRole("switch", { name: /Đáp án đúng/ }));
    await user.click(within(dialog).getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() => expect(calls.patches).toHaveLength(1), { timeout: 5_000 });
    expect(calls.patches[0]).toEqual({
      testVersionId: a.testVersionId,
      targets: { classIds: [DETAIL_CLASS_ID], studentIds: [] },
      window: { opensAt: a.window.opensAt, closesAt: a.window.closesAt },
      durationMinutes: 45,
      maxAttempts: 1,
      shuffleQuestions: true,
      shuffleOptions: true,
      review: { ...a.review, showCorrectAnswers: true },
      integrity: a.integrity,
      studentNote: null,
      draft: false,
    });
    expect(
      await screen.findByText("Đã lưu Kết quả. Học viên thấy thay đổi ngay."),
    ).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("keeps the dialog open with the server's message on a 409", async () => {
    serveDetail(detailAssignment(), { patchStatus: 409 });
    const user = renderDetail("settings");
    await user.click(await screen.findByRole("button", { name: "Sửa Tính toàn vẹn" }));
    const dialog = await screen.findByRole("dialog", { name: "Sửa Tính toàn vẹn" });
    await user.click(within(dialog).getByRole("button", { name: "Lưu thay đổi" }));
    expect(
      await within(dialog).findByText("Bài đang mở nên không đổi được thời gian làm."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("dialog", { name: "Sửa Tính toàn vẹn" }),
    ).toBeInTheDocument();
  });

  it("keeps the honest limits in the Integrity dialog", async () => {
    serveDetail(detailAssignment());
    const user = renderDetail("settings");
    await user.click(await screen.findByRole("button", { name: "Sửa Tính toàn vẹn" }));
    const dialog = await screen.findByRole("dialog", { name: "Sửa Tính toàn vẹn" });
    expect(
      within(dialog).getByText("Hệ thống thấy được gì và không thấy gì"),
    ).toBeInTheDocument();
  });

  it("edits the window with the wizard's fields and the class chips", async () => {
    serveDetail(detailAssignment());
    const user = renderDetail("settings");
    await user.click(await screen.findByRole("button", { name: "Sửa Thời gian mở" }));
    const dialog = await screen.findByRole("dialog", { name: "Sửa Thời gian mở" });
    expect(within(dialog).getByRole("button", { name: "Lưu thay đổi" })).toBeEnabled();
    expect(within(dialog).getByRole("group", { name: "Giao cho" })).toBeInTheDocument();
  });
});

describe("the Questions tab", () => {
  it("lists questions hardest first with the deck's three tones, and keeps the version facts", async () => {
    serveDetail(detailAssignment(), {
      analysis: {
        handedIn: 12,
        items: [
          {
            questionId: "018f0000-0000-7000-8000-000000000101",
            number: 27,
            type: "short_answer",
            promptExcerpt: "Term for streets closed to cars",
            answered: 12,
            correctRate: 0.22,
          },
          {
            questionId: "018f0000-0000-7000-8000-000000000102",
            number: 8,
            type: "single_choice",
            promptExcerpt: "Match headings",
            answered: 12,
            correctRate: 0.61,
          },
          {
            questionId: "018f0000-0000-7000-8000-000000000103",
            number: 3,
            type: "true_false",
            promptExcerpt: "Main purpose",
            answered: 12,
            correctRate: 0.83,
          },
          {
            questionId: "018f0000-0000-7000-8000-000000000104",
            number: 5,
            type: "fill_blank",
            promptExcerpt: "Nobody marked",
            answered: 0,
            correctRate: null,
          },
        ],
      },
    });
    renderDetail("questions");
    expect(
      await screen.findByText(
        "Các câu hỏi xếp theo mức độ học viên làm sai nhiều nhất.",
      ),
    ).toBeInTheDocument();
    const bars = await screen.findAllByRole("progressbar");
    expect(bars.map((bar) => bar.getAttribute("aria-label"))).toEqual([
      "22% đúng",
      "61% đúng",
      "83% đúng",
      "Chưa có bài chấm cho câu này",
    ]);
    expect(bars.map((bar) => bar.firstElementChild?.className)).toEqual([
      expect.stringContaining("bg-danger"),
      expect.stringContaining("bg-warning"),
      expect.stringContaining("bg-success"),
      expect.any(String),
    ]);
    expect(screen.getByText("C27")).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Xem đề" })).toBeInTheDocument();
  });

  it("says when nothing has been handed in", async () => {
    serveDetail(detailAssignment(), {
      analysis: {
        handedIn: 0,
        items: [
          {
            questionId: "018f0000-0000-7000-8000-000000000101",
            number: 1,
            type: "short_answer",
            promptExcerpt: "Anything",
            answered: 0,
            correctRate: null,
          },
        ],
      },
    });
    renderDetail("questions");
    expect(await screen.findByText("Chưa có bài nộp nào.")).toBeInTheDocument();
  });

  it("shows the grading permission's line without reading the analysis", async () => {
    useAuthStore.getState().setSession("token", {
      ...teacherUser,
      permissions: teacherUser.permissions.filter((key) => key !== "teaching.grading"),
    });
    serveDetail(detailAssignment());
    renderDetail("questions");
    expect(
      await screen.findByText("Cần quyền chấm bài để xem học viên làm sai câu nào."),
    ).toBeInTheDocument();
  });
});

describe("the Students tab", () => {
  it("says how long a student's own extension runs", async () => {
    serveDetail(detailAssignment(), {
      rows: [detailRow(AN, "Lê Văn An", { extendedTo: "2099-09-10T14:00:00Z" })],
    });
    renderDetail();
    expect(await screen.findByText(/^Gia hạn đến /)).toBeInTheDocument();
  });
});

describe("Close early", () => {
  it.each([
    [1, "1 học viên được gia hạn riêng vẫn giữ thời gian của mình."],
    [0, null],
  ])("names %i students who keep their own extension", async (count, sentence) => {
    serveDetail(detailAssignment(), {
      rows: [
        detailRow(
          AN,
          "Lê Văn An",
          count > 0 ? { extendedTo: "2099-09-10T14:00:00Z" } : {},
        ),
        detailRow(BINH, "Trần Bình"),
      ],
    });
    const user = renderDetail();
    await screen.findByText("Lê Văn An");
    await user.click(screen.getByRole("button", { name: "Thao tác khác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Đóng sớm" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Đóng Unit 5 — Present perfect & listening ngay?",
    });
    expect(
      within(dialog).getByText(/Học viên đang làm vẫn được làm hết 45 phút/),
    ).toBeInTheDocument();
    if (sentence === null)
      expect(within(dialog).queryByText(/gia hạn riêng/)).toBeNull();
    else expect(within(dialog).getByText(new RegExp(sentence))).toBeInTheDocument();
  });
});

describe("Reopen for a student", () => {
  async function open(rows: Parameters<typeof serveDetail>[1]) {
    const calls = serveDetail(detailAssignment({ maxAttempts: 1 }), rows);
    const user = renderDetail();
    await screen.findByText("Lê Văn An");
    await user.click(screen.getByRole("button", { name: "Thao tác khác" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Mở lại cho một học viên" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Mở lại cho một học viên",
    });
    return { calls, user, dialog };
  }

  it("adds one attempt for a student who has none left, and says so", async () => {
    const { calls, user, dialog } = await open({
      rows: [detailRow(AN, "Lê Văn An", { state: "submitted", attemptNo: 1 })],
    });
    expect(
      within(dialog).getByText(
        "Lê Văn An đã dùng hết lượt làm, nên sẽ được thêm một lượt.",
      ),
    ).toBeInTheDocument();
    await user.type(within(dialog).getByRole("textbox", { name: /Lý do/ }), "Mất điện");
    await user.click(within(dialog).getByRole("button", { name: "Mở lại" }));
    await waitFor(() => expect(calls.overrides).toHaveLength(1), { timeout: 5_000 });
    expect(calls.overrides[0]).toMatchObject({
      studentIds: [AN],
      durationMinutes: 60,
      reason: "Mất điện",
      extraAttempts: 1,
    });
  });

  it("adds no attempt for a student who can still continue", async () => {
    const { calls, user, dialog } = await open({
      rows: [detailRow(AN, "Lê Văn An", { state: "in_progress", attemptNo: 1 })],
    });
    expect(
      within(dialog).getByText(
        "Lê Văn An vẫn còn lượt làm, nên có thể tiếp tục mà không cần thêm lượt.",
      ),
    ).toBeInTheDocument();
    await user.type(within(dialog).getByRole("textbox", { name: /Lý do/ }), "Hỏng máy");
    await user.click(within(dialog).getByRole("button", { name: "Mở lại" }));
    await waitFor(() => expect(calls.overrides).toHaveLength(1), { timeout: 5_000 });
    expect(calls.overrides[0]).not.toHaveProperty("extraAttempts");
  });

  it("requires a reason", async () => {
    const { calls, user, dialog } = await open({
      rows: [detailRow(AN, "Lê Văn An")],
    });
    await user.click(within(dialog).getByRole("button", { name: "Mở lại" }));
    expect(calls.overrides).toEqual([]);
    expect(within(dialog).getByRole("textbox", { name: /Lý do/ })).toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });
});

describe("Extend deadline", () => {
  const ROWS = [
    detailRow(AN, "Lê Văn An", { state: "in_progress", attemptNo: 1 }),
    detailRow(BINH, "Trần Bình"),
  ];

  async function open() {
    const calls = serveDetail(detailAssignment(), { rows: ROWS });
    const user = renderDetail();
    await screen.findByText("Lê Văn An");
    await user.click(screen.getByRole("button", { name: "Gia hạn" }));
    const dialog = await screen.findByRole("dialog", { name: "Gia hạn" });
    return { calls, user, dialog };
  }

  it("moves the close for everyone through extend, with no reason", async () => {
    const { calls, user, dialog } = await open();
    expect(
      within(within(dialog).getByRole("group", { name: "Áp dụng cho" })).getByRole(
        "button",
        { name: "Mọi người" },
      ),
    ).toHaveAttribute("aria-pressed", "true");
    expect(within(dialog).queryByRole("textbox", { name: /Lý do/ })).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Gia hạn" }));
    await waitFor(() => expect(calls.extensions).toHaveLength(1), { timeout: 5_000 });
    expect(calls.extensions[0]).toEqual({ minutes: 30, notify: true });
    expect(calls.overrides).toEqual([]);
  });

  it("gives chosen students their own later close, with the reason", async () => {
    const { calls, user, dialog } = await open();
    await user.click(
      within(dialog).getByRole("button", { name: "Học viên được chọn" }),
    );
    const students = within(dialog).getByRole("group", { name: "Học viên" });
    expect(within(students).getByText("Đang làm")).toBeInTheDocument();
    expect(within(students).getByText("Chưa bắt đầu")).toBeInTheDocument();
    await user.click(within(students).getByRole("checkbox", { name: /Trần Bình/ }));
    await user.click(within(dialog).getByRole("button", { name: "1 giờ" }));
    await user.type(within(dialog).getByRole("textbox", { name: /Lý do/ }), "Ốm");
    await user.click(within(dialog).getByRole("button", { name: "Gia hạn" }));
    await waitFor(() => expect(calls.overrides).toHaveLength(1), { timeout: 5_000 });
    expect(calls.overrides[0]).toEqual({
      studentIds: [BINH],
      extendBy: 60,
      reason: "Ốm",
      notify: true,
    });
    expect(calls.extensions).toEqual([]);
  });

  it("asks for a student and a reason before extending for chosen students", async () => {
    const { calls, user, dialog } = await open();
    await user.click(
      within(dialog).getByRole("button", { name: "Học viên được chọn" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Gia hạn" }));
    expect(within(dialog).getByText("Chọn ít nhất một học viên.")).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: /Lý do/ })).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(calls.overrides).toEqual([]);
    expect(calls.extensions).toEqual([]);
  });

  it("offers only everyone to a teacher who may not intervene", async () => {
    useAuthStore.getState().setSession("token", {
      ...teacherUser,
      permissions: teacherUser.permissions.filter(
        (key) => key !== "teaching.attempts.intervene",
      ),
    });
    const { dialog } = await open();
    expect(within(dialog).queryByRole("group", { name: "Áp dụng cho" })).toBeNull();
  });

  it("is not offered on a closed assignment, which is reopened instead", async () => {
    serveDetail(
      detailAssignment({
        status: "closed",
        window: {
          opensAt: "2020-09-07T01:00:00Z",
          closesAt: "2020-09-09T14:00:00Z",
          closedAt: null,
        },
      }),
    );
    renderDetail();
    expect(
      await screen.findByRole("button", { name: "Gia hạn cho tất cả" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Gia hạn" })).toBeNull();
  });
});

describe("Duplicate", () => {
  it("copies the assignment as a draft for its classes and offers to open it", async () => {
    const calls = serveDetail(detailAssignment());
    const user = renderDetail();
    await user.click(await screen.findByRole("button", { name: "Thao tác khác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Nhân bản" }));
    const dialog = await screen.findByRole("dialog", { name: "Nhân bản bài giao" });
    await user.click(within(dialog).getByRole("button", { name: "Tạo bản nháp" }));
    await waitFor(() => expect(calls.duplicates).toHaveLength(1), { timeout: 5_000 });
    expect(calls.duplicates[0]).toEqual({ classIds: [DETAIL_CLASS_ID] });
    expect(await screen.findByText("Đã tạo bản nháp.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mở" })).toBeInTheDocument();
  });
});
