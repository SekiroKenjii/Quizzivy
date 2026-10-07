import { createElement } from "react";
import { afterEach, beforeAll, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { useTranslation } from "react-i18next";
import i18n, { setLocale } from "@/lib/i18n";
import vi from "@/lib/i18n/locales/vi.json";
import en from "@/lib/i18n/locales/en.json";

const expected = {
  vi: {
    queueRow: "{{name}} · {{assignment}} · {{remaining}}",
    candidate: "Xem bài của {{name}}: {{title}}",
    pickFirstKeys: "Chọn điểm trước (phím 1–{{n}}).",
    title: "Chấm bài",
    remaining: "Còn {{answers}} câu trả lời từ {{students}} học viên.",
    mode: "Cách nhóm",
    byStudent: "Theo học viên",
    byQuestion: "Theo câu hỏi",
    studentsWaiting: "Học viên đang chờ",
    questionsWaiting: "Câu hỏi đang chờ",
    question: "Câu {{n}}",
    questionTile: "C{{n}}",
    left: "Còn {{count}}",
    savedSession: "Đã lưu trong phiên",
    answerCard: "Câu trả lời đang chấm",
    questionPoints: "Câu {{n}} · {{max}} điểm",
    prompt: "Câu hỏi",
    accepted: "Đáp án chấp nhận",
    score: "Chấm {{points}} điểm",
    numberScore: "Điểm (0–{{max}})",
    comment: "Nhận xét (không bắt buộc, học viên sẽ thấy)",
    commentPlaceholder: "Viết góp ý…",
    saveNext: "Lưu & câu tiếp theo",
    position: "Câu trả lời {{i}}/{{n}} của {{name}} trong cửa sổ này",
    moveKeys: "K / J để chuyển",
    toMove: "để chuyển",
    previousKey: "K",
    nextKey: "J",
    openReview: "Xem toàn bộ bài",
    unavailable: "Bạn không có quyền chấm bài tại đây.",
    saveFailed: "Không lưu được. Điểm đã xác nhận vẫn được giữ; hãy thử lại.",
    pickFirst: "Chọn điểm trước bằng nút hoặc phím số.",
    pendingUnknown:
      "Chưa xác nhận được số câu còn chờ chấm. Xem lại bài trước khi hoàn tất.",
    finishFailed: "Điểm đã lưu, nhưng chưa hoàn tất chấm bài.",
    retryFinish: "Thử hoàn tất lại",
    loadFailed: "Không tải được hàng đợi chấm.",
    prefix:
      "Đang hiển thị {{shown}}/{{total}} câu chờ chấm; câu tiếp theo sẽ được tải sau khi lưu.",
    windowProgress: "Tiến độ lưu trong cửa sổ hiện tại",
    empty: "Không có câu trả lời chờ chấm",
    emptyHint: "Bài đã lưu điểm vẫn cần được hoàn tất bằng thao tác rõ ràng bên dưới.",
    assignment: "Bài giao",
    student: "Học viên",
    all: "Tất cả",
    appliedAssignment: "Bài giao đang lọc",
    appliedStudent: "Học viên đang lọc",
    recoveryTitle: "Bài sẵn sàng để xem lại hoặc hoàn tất",
    recoveryHint:
      "Danh sách này gồm cả bài chỉ có câu tự động hoặc bỏ trống. Không bài nào được tự hoàn tất. Dữ liệu có thể thay đổi khi đang quét; hãy quét lại nếu cần.",
    rescan: "Quét lại",
    scanFailed: "Chưa quét đủ các bài. Không thể kết luận đã hoàn tất.",
    reviewFailed: "Không tải được bài để xác nhận.",
    ready: "Hiện không còn câu chờ chấm. Bạn có thể hoàn tất bài này.",
    snippets: {
      short: {
        "0": "Kiểm tra chính tả",
        "1": "Vượt giới hạn từ",
        "2": "Gần đúng — sai dạng từ",
        "3": "Diễn đạt lại tốt",
      },
      essay: {
        "0": "Tổng quan rõ ràng",
        "1": "Thêm so sánh số liệu",
        "2": "Chú ý nhất quán về thì",
        "3": "Vốn từ phong phú",
      },
    },
  },
  en: {
    queueRow: "{{name}} · {{assignment}} · {{remaining}}",
    candidate: "Review {{name}}: {{title}}",
    pickFirstKeys: "Pick a score first (keys 1–{{n}}).",
    title: "Grading",
    remaining: "{{answers}} answers left from {{students}} students.",
    mode: "Grouping",
    byStudent: "By student",
    byQuestion: "By question",
    studentsWaiting: "Students waiting",
    questionsWaiting: "Questions waiting",
    question: "Question {{n}}",
    questionTile: "Q{{n}}",
    left: "{{count}} left",
    savedSession: "Saved this session",
    answerCard: "Answer being graded",
    questionPoints: "Question {{n}} · {{max}} pt",
    prompt: "Question",
    accepted: "Accepted",
    score: "Give {{points}} points",
    numberScore: "Score (0–{{max}})",
    comment: "Comment (optional, student sees it)",
    commentPlaceholder: "Write feedback…",
    saveNext: "Save & next",
    position: "Answer {{i}} of {{n}} for {{name}} in this window",
    moveKeys: "K / J to move",
    toMove: "to move",
    previousKey: "K",
    nextKey: "J",
    openReview: "Full paper review",
    unavailable: "You do not have permission to grade here.",
    saveFailed: "Could not save. Confirmed marks are retained; please retry.",
    pickFirst: "Pick a score first using a button or number key.",
    pendingUnknown:
      "The remaining manual count is not confirmed. Review the paper before finishing.",
    finishFailed: "The mark was saved, but grading has not been finished.",
    retryFinish: "Retry Finish",
    loadFailed: "Could not load the grading queue.",
    prefix:
      "Showing {{shown}} of {{total}} pending answers; saving refills the window.",
    windowProgress: "Saved progress in the current window",
    empty: "Nothing to grade",
    emptyHint: "Papers with saved marks still require an explicit Finish action below.",
    assignment: "Assignment",
    student: "Student",
    all: "All",
    appliedAssignment: "Filtered assignment",
    appliedStudent: "Filtered student",
    recoveryTitle: "Papers ready for review or Finish",
    recoveryHint:
      "Candidates include auto-only and unanswered papers. None is finished automatically. Records can move during a scan; rescan when needed.",
    rescan: "Rescan",
    scanFailed: "The scan is incomplete. It does not prove completion.",
    reviewFailed: "Could not load the paper for confirmation.",
    ready: "No manual answers currently remain. You may finish this paper.",
    snippets: {
      short: {
        "0": "Check spelling",
        "1": "Word limit exceeded",
        "2": "Close — wrong word form",
        "3": "Good paraphrase",
      },
      essay: {
        "0": "Clear overview",
        "1": "Add more data comparisons",
        "2": "Watch tense consistency",
        "3": "Good range of vocabulary",
      },
    },
  },
};
const eager = { vi: structuredClone(vi), en: structuredClone(en) };
const before = {
  vi: JSON.stringify(i18n.getResourceBundle("vi", "translation")),
  en: JSON.stringify(i18n.getResourceBundle("en", "translation")),
};

beforeAll(async () => {
  await import("@/features/attempts/pages/teacher/GradingPage");
});

afterEach(async () => {
  await act(async () => setLocale("vi"));
});

function leaves(value: unknown, prefix = ""): Record<string, string> {
  if (typeof value === "string") return { [prefix]: value };
  if (typeof value !== "object" || value === null) return {};
  return Object.fromEntries(
    Object.entries(value).flatMap(([key, item]) =>
      Object.entries(leaves(item, prefix ? `${prefix}.${key}` : key)),
    ),
  );
}

function Labels() {
  const { t } = useTranslation();
  return createElement(
    "section",
    null,
    createElement("h1", null, t("grading.title")),
    createElement("button", null, t("grading.retryFinish")),
  );
}

it("keeps grading copy out of eager reader locales and preserves every unrelated resource", () => {
  expect(eager.vi).not.toHaveProperty("grading");
  expect(eager.en).not.toHaveProperty("grading");
  for (const locale of ["vi", "en"] as const) {
    const unrelated: Record<string, unknown> = {
      ...i18n.getResourceBundle(locale, "translation"),
    };
    delete unrelated.grading;
    expect(JSON.stringify(unrelated)).toBe(before[locale]);
  }
});

it("registers the exact original bilingual grading leaves without empty strings or parity drift", () => {
  expect(Object.keys(leaves(expected.vi)).sort((a, b) => a.localeCompare(b))).toEqual(
    Object.keys(leaves(expected.en)).sort((a, b) => a.localeCompare(b)),
  );
  for (const locale of ["vi", "en"] as const) {
    const registered: unknown = i18n.getResource(locale, "translation", "grading");
    expect(registered).toEqual(expected[locale]);
    expect(
      Object.values(leaves(registered)).every((value) => value.trim().length > 0),
    ).toBe(true);
  }
});

it.each(["vi", "en"] as const)(
  "renders %s grading labels on the first render after route import",
  async (locale) => {
    await act(async () => setLocale(locale));
    render(createElement(Labels));
    expect(
      screen.getByRole("heading", { name: expected[locale].title }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: expected[locale].retryFinish }),
    ).toBeInTheDocument();
  },
);

it("updates mounted grading labels on a live language switch without replacing the DOM", async () => {
  render(createElement(Labels));
  const button = screen.getByRole("button", { name: expected.vi.retryFinish });
  await act(async () => setLocale("en"));
  expect(screen.getByRole("heading", { name: expected.en.title })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: expected.en.retryFinish })).toBe(button);
  await act(async () => setLocale("vi"));
  expect(screen.getByRole("button", { name: expected.vi.retryFinish })).toBe(button);
});
