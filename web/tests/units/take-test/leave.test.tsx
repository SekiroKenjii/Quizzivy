import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  saveAnswers,
  type StudentQuestion,
} from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { pending } from "@/features/integrity/buffer";
import { ApiError } from "@/lib/api/errors";
import { session, viewport } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
}));

const ASK = "Thoát khỏi bài làm?";
const SAVED_BODY =
  "Câu trả lời của bạn đã được lưu, nhưng đồng hồ vẫn chạy. Hãy quay lại trước khi hết giờ.";
const PENDING_BODY =
  "Câu trả lời mới nhất của bạn sẽ được lưu trước khi thoát. Đồng hồ vẫn chạy, hãy quay lại trước khi hết giờ.";
const UNSAVED = "Câu trả lời chưa được lưu";
const UNSAVED_BODY =
  "Kết nối chưa ổn định. Hãy thử lưu lại trước khi thoát. Đồng hồ vẫn tiếp tục chạy.";
const DRAFT = "quizzivy.answer-draft.att-1";

const questions: StudentQuestion[] = [
  {
    id: "q1",
    sectionId: "s1",
    type: "short_answer",
    prompt: "What should cities prioritise?",
    points: 1,
  },
];

const store = () => useTakeTestStore.getState();
const superseded = () =>
  new ApiError({ status: 409, code: "SESSION_SUPERSEDED", message: "elsewhere" });

function reply() {
  const now = new Date().toISOString();
  return {
    serverTime: now,
    savedAt: now,
    deadlineAt: new Date(store().deadlineAt).toISOString(),
  };
}

function heldSave() {
  const held: { resolve?: () => void; reject?: (cause: unknown) => void } = {};
  vi.mocked(saveAnswers).mockImplementationOnce(
    () =>
      new Promise((resolve, reject) => {
        held.resolve = () => resolve(reply());
        held.reject = reject;
      }),
  );
  return {
    succeed: () => act(async () => held.resolve?.()),
    fail: () => act(async () => held.reject?.(new Error("offline"))),
  };
}

async function open() {
  const router = createMemoryRouter(
    [
      { path: "/app", element: <p>home</p> },
      { path: "/app/classes", element: <p>classes</p> },
      { path: "/app/attempts/:attemptId", element: <TakeTestPage /> },
    ],
    { initialEntries: ["/app/classes", "/app/attempts/att-1"], initialIndex: 1 },
  );
  render(<RouterProvider router={router} />);
  await screen.findByText("What should cities prioritise?");
  return router;
}

const leaveButton = () => screen.getByRole("button", { name: "Thoát khỏi bài làm" });
const dialog = () => screen.getByRole("dialog");
const inDialog = () => within(dialog());
const path = (router: { state: { location: { pathname: string } } }) =>
  router.state.location.pathname;

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  const now = new Date();
  vi.mocked(getAttempt)
    .mockReset()
    .mockResolvedValue({
      ...session({
        serverTime: now.toISOString(),
        deadlineAt: new Date(now.getTime() + 3_600_000).toISOString(),
      }),
      questions,
    });
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () => reply());
});

afterEach(() => {
  store().reset();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe.each(["desktop", "phone"] as const)("leaving the test on a %s", (width) => {
  beforeEach(() => {
    viewport(width);
  });

  it("asks first, and Stay keeps the student on the paper", async () => {
    const user = userEvent.setup();
    const router = await open();

    await user.click(leaveButton());
    expect(dialog()).toHaveAccessibleName(ASK);
    expect(dialog()).toHaveAccessibleDescription(SAVED_BODY);
    expect(
      inDialog()
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Ở lại", "Thoát"]);

    await user.click(inDialog().getByRole("button", { name: "Ở lại" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(path(router)).toBe("/app/attempts/att-1");
    expect(leaveButton()).toHaveFocus();
    expect(saveAnswers).not.toHaveBeenCalled();
  });

  it("takes Esc as Stay", async () => {
    const user = userEvent.setup();
    const router = await open();
    await user.click(leaveButton());
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(path(router)).toBe("/app/attempts/att-1");
  });

  it("goes home at once when nothing is pending", async () => {
    const user = userEvent.setup();
    const router = await open();
    await user.click(leaveButton());
    await user.click(inDialog().getByRole("button", { name: "Thoát" }));

    await waitFor(() => expect(path(router)).toBe("/app"));
    expect(saveAnswers).not.toHaveBeenCalled();
    expect(router.state.historyAction).toBe("PUSH");
  });

  it("goes home at once with every answer saved, whatever the monitor has recorded", async () => {
    const user = userEvent.setup();
    const router = await open();
    act(() => {
      window.dispatchEvent(new Event("blur"));
      window.dispatchEvent(new Event("focus"));
    });
    expect(pending().map((event) => event.kind)).toEqual([
      "window_blur",
      "window_focus",
    ]);
    vi.mocked(saveAnswers).mockImplementation(() => new Promise(() => {}));
    await user.click(leaveButton());
    await user.click(inDialog().getByRole("button", { name: "Thoát" }));

    await waitFor(() => expect(path(router)).toBe("/app"));
    expect(saveAnswers).not.toHaveBeenCalled();
  });

  it("saves a pending answer before it goes home", async () => {
    const user = userEvent.setup();
    const router = await open();
    const save = heldSave();
    await user.type(screen.getByRole("textbox"), "public green space");

    await user.click(leaveButton());
    expect(dialog()).toHaveAccessibleName(ASK);
    expect(dialog()).toHaveAccessibleDescription(PENDING_BODY);
    expect(saveAnswers).not.toHaveBeenCalled();

    await user.click(inDialog().getByRole("button", { name: "Thoát" }));
    expect(saveAnswers).toHaveBeenCalledTimes(1);
    expect(vi.mocked(saveAnswers).mock.calls[0]?.[1].answers).toEqual({
      q1: { type: "text", value: "public green space" },
    });
    const leaving = inDialog().getByRole("button", { name: "Thoát" });
    expect(leaving).toHaveAttribute("aria-busy", "true");
    expect(leaving).toHaveAttribute("aria-disabled", "true");
    expect(path(router)).toBe("/app/attempts/att-1");

    await user.click(leaving);
    expect(saveAnswers).toHaveBeenCalledTimes(1);

    await save.succeed();
    await waitFor(() => expect(path(router)).toBe("/app"));
    expect(localStorage.getItem(DRAFT)).toBeNull();
  });

  it("sends an answer edited while an earlier save was out before it goes home", async () => {
    const user = userEvent.setup();
    const router = await open();
    const earlier = heldSave();
    await user.type(screen.getByRole("textbox"), "parks");
    act(() => {
      void store().flush();
    });
    await user.type(screen.getByRole("textbox"), " and trees");

    await user.click(leaveButton());
    await user.click(inDialog().getByRole("button", { name: "Thoát" }));
    expect(saveAnswers).toHaveBeenCalledTimes(1);
    await earlier.succeed();

    await waitFor(() => expect(path(router)).toBe("/app"));
    expect(saveAnswers).toHaveBeenCalledTimes(2);
    expect(vi.mocked(saveAnswers).mock.lastCall?.[1].answers).toEqual({
      q1: { type: "text", value: "parks and trees" },
    });
  });

  it("does not leave when the save fails, and says the answers are not saved", async () => {
    const user = userEvent.setup();
    const router = await open();
    const save = heldSave();
    vi.mocked(saveAnswers).mockRejectedValue(new Error("offline"));
    await user.type(screen.getByRole("textbox"), "public green space");
    await user.click(leaveButton());
    await user.click(inDialog().getByRole("button", { name: "Thoát" }));
    await save.fail();

    expect(dialog()).toHaveAccessibleName(UNSAVED);
    expect(dialog()).toHaveAccessibleDescription(UNSAVED_BODY);
    expect(
      inDialog()
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Ở lại", "Thử lưu lại"]);
    expect(path(router)).toBe("/app/attempts/att-1");
    expect(JSON.parse(localStorage.getItem(DRAFT) ?? "{}").answers).toEqual({
      q1: { type: "text", value: "public green space" },
    });

    await user.click(inDialog().getByRole("button", { name: "Ở lại" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("textbox")).toHaveValue("public green space");
    expect(path(router)).toBe("/app/attempts/att-1");
  });

  it("leaves once a retry from the dialog saves the answer", async () => {
    const user = userEvent.setup();
    const router = await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(new Error("offline"));
    await user.type(screen.getByRole("textbox"), "parks");
    await user.click(leaveButton());
    await user.click(inDialog().getByRole("button", { name: "Thoát" }));
    await screen.findByRole("dialog", { name: UNSAVED });

    const retry = heldSave();
    await user.click(inDialog().getByRole("button", { name: "Thử lưu lại" }));
    expect(inDialog().getByRole("button", { name: "Thoát" })).toHaveAttribute(
      "aria-busy",
      "true",
    );
    expect(path(router)).toBe("/app/attempts/att-1");

    await retry.succeed();
    await waitFor(() => expect(path(router)).toBe("/app"));
    expect(saveAnswers).toHaveBeenCalledTimes(2);
  });

  it("goes back to asking when the store's own retry saves the answer first", async () => {
    const user = userEvent.setup();
    const router = await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(new Error("offline"));
    await user.type(screen.getByRole("textbox"), "parks");
    await user.click(leaveButton());
    await user.click(inDialog().getByRole("button", { name: "Thoát" }));
    await screen.findByRole("dialog", { name: UNSAVED });

    await act(() => store().flush());
    expect(dialog()).toHaveAccessibleName(ASK);
    expect(dialog()).toHaveAccessibleDescription(SAVED_BODY);
    expect(path(router)).toBe("/app/attempts/att-1");

    await user.click(inDialog().getByRole("button", { name: "Thoát" }));
    await waitFor(() => expect(path(router)).toBe("/app"));
  });

  it("does not leave after all when the student stays while the save is out", async () => {
    const user = userEvent.setup();
    const router = await open();
    const save = heldSave();
    await user.type(screen.getByRole("textbox"), "parks");
    await user.click(leaveButton());
    await user.click(inDialog().getByRole("button", { name: "Thoát" }));

    await user.click(inDialog().getByRole("button", { name: "Ở lại" }));
    await save.succeed();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(path(router)).toBe("/app/attempts/att-1");
    expect(store().dirty.size).toBe(0);
  });

  it("holds the back button until a pending answer is saved, then goes where it was going", async () => {
    const user = userEvent.setup();
    const router = await open();
    const save = heldSave();
    await user.type(screen.getByRole("textbox"), "parks");

    await act(() => router.navigate(-1));
    expect(path(router)).toBe("/app/attempts/att-1");
    expect(saveAnswers).toHaveBeenCalledTimes(1);
    expect(dialog()).toHaveAccessibleName(ASK);
    expect(inDialog().getByRole("button", { name: "Thoát" })).toHaveAttribute(
      "aria-busy",
      "true",
    );

    await save.succeed();
    await waitFor(() => expect(path(router)).toBe("/app/classes"));
  });

  it("keeps the student on the paper when the back button's save fails, until they stay or it saves", async () => {
    const user = userEvent.setup();
    const router = await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(new Error("offline"));
    await user.type(screen.getByRole("textbox"), "parks");

    await act(() => router.navigate(-1));
    await screen.findByRole("dialog", { name: UNSAVED });
    expect(path(router)).toBe("/app/attempts/att-1");

    await user.click(inDialog().getByRole("button", { name: "Ở lại" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(router.state.blockers.size).toBe(1);
    expect([...router.state.blockers.values()][0]?.state).toBe("unblocked");
    await user.type(screen.getByRole("textbox"), " and trees");
    expect(screen.getByRole("textbox")).toHaveValue("parks and trees");

    await act(() => router.navigate(-1));
    await waitFor(() => expect(path(router)).toBe("/app/classes"));
    expect(vi.mocked(saveAnswers).mock.lastCall?.[1].answers).toEqual({
      q1: { type: "text", value: "parks and trees" },
    });
  });

  it("goes where the back button was going when a retry from the dialog saves", async () => {
    const user = userEvent.setup();
    const router = await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(new Error("offline"));
    await user.type(screen.getByRole("textbox"), "parks");
    await act(() => router.navigate(-1));
    await screen.findByRole("dialog", { name: UNSAVED });

    await user.click(inDialog().getByRole("button", { name: "Thử lưu lại" }));
    await waitFor(() => expect(path(router)).toBe("/app/classes"));
    expect(router.state.historyAction).toBe("POP");
  });

  it("lets the back button through without asking when nothing is pending", async () => {
    const router = await open();
    await act(() => router.navigate(-1));
    await waitFor(() => expect(path(router)).toBe("/app/classes"));
    expect(saveAnswers).not.toHaveBeenCalled();
  });

  it("raises the browser's prompt on closing the tab only while an answer is unsaved", async () => {
    const user = userEvent.setup();
    await open();
    const closing = () => {
      const event = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(event);
      return event.defaultPrevented;
    };
    expect(closing()).toBe(false);

    const save = heldSave();
    await user.type(screen.getByRole("textbox"), "parks");
    expect(closing()).toBe(true);

    let flushed: Promise<boolean> | undefined;
    act(() => {
      flushed = store().flush();
    });
    await save.succeed();
    await act(async () => {
      await flushed;
    });
    expect(store().dirty.size).toBe(0);
    expect(closing()).toBe(false);
  });

  it.each(["closed", "deadline"] as const)(
    "leaves without asking once the paper is %s, when no timer is left to warn about",
    async (lock) => {
      const user = userEvent.setup();
      const router = await open();
      act(() => store().lockNow(lock));

      await user.click(leaveButton());
      await waitFor(() => expect(path(router)).toBe("/app"));
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(saveAnswers).not.toHaveBeenCalled();
    },
  );

  it("leaves a paper another device took over without trying to save it", async () => {
    const user = userEvent.setup();
    const router = await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(superseded());
    await user.type(screen.getByRole("textbox"), "parks");
    await act(() => store().flush());
    expect(store().lock).toBe("superseded");
    expect(store().dirty.size).toBe(1);

    await user.click(leaveButton());
    await waitFor(() => expect(path(router)).toBe("/app"));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(saveAnswers).toHaveBeenCalledTimes(1);
  });

  it("closes the dialog, without saying the answers are saved, when another device takes the paper over", async () => {
    const user = userEvent.setup();
    const router = await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(superseded());
    await user.type(screen.getByRole("textbox"), "parks");
    await user.click(leaveButton());
    expect(dialog()).toHaveAccessibleDescription(PENDING_BODY);

    await act(() => store().flush());
    expect(store().lock).toBe("superseded");
    expect(store().dirty.size).toBe(1);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.queryByText(SAVED_BODY)).toBeNull();
    expect(path(router)).toBe("/app/attempts/att-1");

    await user.click(leaveButton());
    await waitFor(() => expect(path(router)).toBe("/app"));
  });

  it("closes the dialog over a held back button too when the store's retry finds the paper taken over", async () => {
    const user = userEvent.setup();
    const router = await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(new Error("offline"));
    await user.type(screen.getByRole("textbox"), "parks");
    await act(() => router.navigate(-1));
    await screen.findByRole("dialog", { name: UNSAVED });

    vi.mocked(saveAnswers).mockRejectedValueOnce(superseded());
    await act(() => store().flush());
    expect(store().lock).toBe("superseded");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.queryByText(SAVED_BODY)).toBeNull();
    expect(path(router)).toBe("/app/attempts/att-1");

    await user.click(leaveButton());
    await waitFor(() => expect(path(router)).toBe("/app"));
  });

  it("does not ask again by itself when the paper is taken back after a lock closed the dialog", async () => {
    const user = userEvent.setup();
    const router = await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(superseded());
    await user.type(screen.getByRole("textbox"), "parks");
    await user.click(leaveButton());
    await act(() => store().flush());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    const now = new Date();
    act(() =>
      store().hydrate({
        ...session({
          serverTime: now.toISOString(),
          deadlineAt: new Date(now.getTime() + 3_600_000).toISOString(),
        }),
        questions,
        sessionId: "ses-2",
      }),
    );
    expect(store().lock).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(path(router)).toBe("/app/attempts/att-1");

    await user.click(leaveButton());
    expect(dialog()).toHaveAccessibleName(ASK);
  });
});
