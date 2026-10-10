import { describe, expect, it, beforeEach } from "vitest";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { draftBody, draftOf, emptyDraft } from "@/features/assignments/draft";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BASE,
  CHI,
  CLASS_A,
  CLASS_B,
  NOW,
  OTHER_TEST_ID,
  TEST_ID,
  VERSION_ID,
  VERSIONS,
  installWizardServer,
  renderWizard,
  requests,
  stored,
  version,
} from "@tests/support/assignmentWizard";
import "@/lib/i18n";

beforeEach(installWizardServer);

describe("the new assignment wizard", () => {
  it("opens on the Test step with the deck's header, and any step can be opened", async () => {
    const router = renderWizard();
    expect(
      screen.getByRole("heading", { level: 1, name: "Giao bài mới" }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Chưa có gì được gửi đi cho tới khi bạn bấm Giao bài. Mặc định là lựa chọn an toàn.",
      ),
    ).toBeVisible();
    const steps = screen.getByRole("list", { name: "Các bước giao bài" });
    const buttons = within(steps).getAllByRole("button");
    expect(buttons).toHaveLength(4);
    expect(buttons[0]).toHaveAttribute("aria-current", "step");
    for (const button of buttons) expect(button).toBeEnabled();
    expect(buttons[0]).toHaveTextContent("Chưa chọn đề");
    expect(buttons[3]).toHaveTextContent("Chặn sao chép/dán · Hiện điểm");
    const row = await screen.findByRole("radio", { name: /Unit 5 Reading/ });
    await waitFor(() =>
      expect(row).toHaveTextContent("Nghe · 24 câu · 2 câu cần chấm tay"),
    );
    expect(row, "the draft's skills do not describe the version").not.toHaveTextContent(
      "Đọc",
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Tiếp tục" }));
    expect(router.state.location.search).toBe("?step=2");
    await user.click(buttons[3]!);
    expect(router.state.location.search).toBe("?step=4");
    expect(screen.getByRole("heading", { level: 2, name: "Quy định" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Tiếp tục" })).toBeNull();
    expect(screen.getByRole("button", { name: "Giao bài" })).toBeEnabled();
  });

  it("preselects the test and the class named in the URL without counting it as a change", async () => {
    const user = userEvent.setup();
    const router = renderWizard(
      `/teacher/assignments/new?test=${TEST_ID}&class=${CLASS_A}&step=2`,
    );
    expect(
      await screen.findByRole("checkbox", { name: /IELTS Foundation/ }),
    ).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("checkbox", { name: /TOEIC 600/ })).toHaveAttribute(
      "aria-checked",
      "false",
    );
    expect(await screen.findByText("2 học viên sẽ nhận bài này")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Quay lại" }));
    expect(await screen.findByRole("radio", { name: /Unit 5 Reading/ })).toBeChecked();
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/assignments"),
    );
  });

  it("counts a student once across classes and individual picks, and names the class that includes them", async () => {
    const user = userEvent.setup();
    renderWizard("/teacher/assignments/new?step=2");
    await user.click(await screen.findByRole("checkbox", { name: /IELTS Foundation/ }));
    await user.click(screen.getByRole("checkbox", { name: /TOEIC 600/ }));
    expect(await screen.findByText("2 học viên sẽ nhận bài này")).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Thêm từng học viên" }));
    const dialog = await screen.findByRole("dialog");
    const an = await within(dialog).findByRole("checkbox", { name: /Nguyễn An/ });
    expect(an).toHaveTextContent(
      "IELTS Foundation, TOEIC 600 · đã có trong lớp đã chọn",
    );
    expect(within(dialog).getByRole("checkbox", { name: /Lê Chi/ })).toHaveTextContent(
      "Lê Chi",
    );
    await user.click(an);
    await user.click(within(dialog).getByRole("checkbox", { name: /Lê Chi/ }));
    await user.click(within(dialog).getByRole("button", { name: "Thêm 2" }));

    const panel = await screen.findByRole("region", { name: /Học viên thêm lẻ/ });
    expect(within(panel).getByText("Đã có trong lớp IELTS Foundation")).toBeVisible();
    expect(within(panel).getByText("Chưa vào lớp nào")).toBeVisible();
    expect(
      await screen.findByText("3 học viên sẽ nhận bài này · 2 từ các lớp, 1 thêm lẻ"),
    ).toBeVisible();

    await user.click(within(panel).getByRole("button", { name: "Bỏ tất cả" }));
    expect(screen.queryByRole("region", { name: /Học viên thêm lẻ/ })).toBeNull();
    expect(await screen.findByText("2 học viên sẽ nhận bài này")).toBeVisible();
  });

  it("saves a draft with every field and opens the Drafts tab", async () => {
    const user = userEvent.setup();
    const router = renderWizard();
    await user.click(screen.getByRole("button", { name: "Lưu nháp" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Chọn một đề trước khi lưu nháp.",
    );
    expect(requests.posted).toEqual([]);

    await user.click(await screen.findByRole("radio", { name: /Unit 5 Reading/ }));
    await user.click(screen.getByRole("button", { name: "Tiếp tục" }));
    await user.click(await screen.findByRole("checkbox", { name: /TOEIC 600/ }));
    await user.click(screen.getByRole("button", { name: "Lưu nháp" }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/assignments"),
    );
    expect(router.state.location.search).toBe("?status=draft");
    expect(
      await screen.findByText("Đã lưu nháp. Học viên chưa thấy bài này."),
    ).toBeVisible();
    expect(requests.posted).toHaveLength(1);
    expect(requests.posted[0]).toMatchObject({
      draft: true,
      testVersionId: VERSION_ID,
      targets: { classIds: [CLASS_B], studentIds: [] },
      durationMinutes: 45,
      maxAttempts: 1,
      shuffleQuestions: false,
      shuffleOptions: false,
      review: {
        showScore: true,
        showCorrectAnswers: false,
        showExplanations: false,
        release: "on_submit",
        showClassAverage: false,
      },
      integrity: {
        requireFullscreen: false,
        blockCopyPaste: true,
        maxFocusLoss: 0,
        onLimitExceeded: "flag",
        minAwayMs: 3000,
      },
      studentNote: null,
    });
    expect(
      Object.keys(requests.posted[0] ?? {}).sort((a, b) => a.localeCompare(b)),
    ).toEqual([
      "draft",
      "durationMinutes",
      "integrity",
      "maxAttempts",
      "review",
      "shuffleOptions",
      "shuffleQuestions",
      "studentNote",
      "targets",
      "testVersionId",
      "window",
    ]);
  });

  it("posts one draft when Save draft is clicked twice in the same tick", async () => {
    const router = renderWizard(`/teacher/assignments/new?test=${TEST_ID}`);
    expect(await screen.findByRole("radio", { name: /Unit 5 Reading/ })).toBeChecked();
    const save = screen.getByRole("button", { name: "Lưu nháp" });
    act(() => {
      fireEvent.click(save);
      fireEvent.click(save);
    });
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/assignments"),
    );
    expect(requests.posted).toHaveLength(1);
  });

  it("says when a test's versions cannot load, and retries", async () => {
    let fail = true;
    server.use(
      http.get(`${BASE}/teacher/tests/${OTHER_TEST_ID}/versions`, () =>
        fail
          ? HttpResponse.json(
              { error: { code: "INTERNAL", message: "boom", requestId: "r1" } },
              { status: 500 },
            )
          : contractJson("/teacher/tests/{id}/versions", "get", 200, {
              items: [VERSIONS[OTHER_TEST_ID]],
            }),
      ),
    );
    const user = userEvent.setup();
    renderWizard();
    expect(
      await screen.findByText("Không tải được phiên bản hiện tại của đề này."),
    ).toBeVisible();
    expect(screen.queryByRole("radio", { name: /Mock B/ })).toBeNull();
    await waitFor(() =>
      expect(screen.getByRole("radio", { name: /Unit 5 Reading/ })).toBeEnabled(),
    );
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("radio", { name: /Mock B/ })).toBeEnabled();
  });

  it("asks before leaving with unsaved choices, and stays when told to", async () => {
    const user = userEvent.setup();
    const router = renderWizard();
    await user.click(await screen.findByRole("radio", { name: /Mock B/ }));
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Rời trang mà không lưu?",
    });
    await user.click(within(dialog).getByRole("button", { name: "Ở lại" }));
    expect(router.state.location.pathname).toBe("/teacher/assignments/new");
    expect(screen.getByRole("radio", { name: /Mock B/ })).toBeChecked();

    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Rời trang",
      }),
    );
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/assignments"),
    );
  });
});

describe("the assignment draft", () => {
  it("round-trips every field through the request body and the stored assignment", () => {
    const draft = {
      ...emptyDraft(new Date("2026-09-01T03:20:00Z")),
      picked: {
        testId: TEST_ID,
        testTitle: "Unit 5 Reading",
        version: { ...version(VERSION_ID, 2), testUpdatedAt: NOW },
      },
      classes: [{ id: CLASS_A, label: "IELTS Foundation", hint: "2" }],
      students: [{ id: CHI, label: "Lê Chi" }],
      durationMinutes: 65,
      maxAttempts: 2,
      shuffleOptions: true,
      integrity: {
        requireFullscreen: true,
        blockCopyPaste: false,
        maxFocusLoss: -1,
        onLimitExceeded: "auto_submit" as const,
        minAwayMs: 5000,
      },
    };
    const back = draftOf(stored({ ...draftBody(draft), draft: true }), [
      draft.picked.version,
    ]);
    expect(back).toEqual({
      ...draft,
      review: { ...draft.review, release: "on_submit", showClassAverage: false },
    });
    expect(draftBody(back)).toEqual({
      ...draftBody(draft),
      review: { ...draft.review, release: "on_submit", showClassAverage: false },
    });
  });
});
