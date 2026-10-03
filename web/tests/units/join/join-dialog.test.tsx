import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  onlineManager,
  QueryClient,
  QueryClientProvider,
  useQuery,
} from "@tanstack/react-query";
import { delay, http } from "msw";
import { DeckScale } from "@/components/ui/deck-scale";
import { Toaster, toast } from "@/components/ui/sonner";
import { listMyAssignments } from "@/features/assignments/api";
import { myClassesQuery } from "@/features/classes/api";
import { JoinDialog } from "@/features/join/components/JoinDialog";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  adminWhoTakesTests,
  myClass,
  sampleClass,
  studentUser,
} from "@tests/support/fixtures";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CODE = "K7QM2PXA";
const OTHER = "W3RT8KDZ";
const CLASS_ID = "019535da-0000-7000-8000-0000000000aa";
const OTHER_ID = "019535da-0000-7000-8000-0000000000bb";
const CLASS_NAME = "TOEIC 600 Weekend";
const TEACHER = "Hoàng Thương";
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const NOT_FOUND = "Không có lớp nào dùng mã này. Hãy kiểm tra lại với giáo viên.";
const TOO_FAST = "Bạn thao tác quá nhanh. Vui lòng thử lại sau.";
const JOINED = { ...myClass, id: CLASS_ID, name: CLASS_NAME, teacherName: TEACHER };

let previews: string[] = [];
let joins: string[] = [];
let asked = { classes: 0, assignments: 0 };
let enrolled: unknown[] = [];

function refusal(status: number, code: string, message: string, headers = {}) {
  return new Response(
    JSON.stringify({ error: { code, message, requestId: REQUEST_ID } }),
    { status, headers: { "Content-Type": "application/json", ...headers } },
  );
}

function lists(classes: "load" | "fail" | "wait" = "load") {
  server.use(
    http.get(`${BASE}/app/classes`, async () => {
      asked.classes += 1;
      if (classes === "wait") await delay("infinite");
      if (classes === "fail") return new Response(null, { status: 500 });
      return contractJson("/app/classes", "get", 200, { items: enrolled });
    }),
    http.get(`${BASE}/app/assignments`, () => {
      asked.assignments += 1;
      return contractJson("/app/assignments", "get", 200, {
        dueNow: [],
        upcoming: [],
        completed: [],
      });
    }),
  );
}

function previewFinds(classId = CLASS_ID) {
  server.use(
    http.post(`${BASE}/join/preview`, async ({ request }) => {
      const { joinCode } = (await request.json()) as { joinCode: string };
      previews.push(joinCode);
      return contractJson("/join/preview", "post", 200, {
        classId,
        className: CLASS_NAME,
        teacherName: TEACHER,
      });
    }),
  );
}

function previewAnswers(answer: () => Response) {
  server.use(
    http.post(`${BASE}/join/preview`, async ({ request }) => {
      const { joinCode } = (await request.json()) as { joinCode: string };
      previews.push(joinCode);
      return answer();
    }),
  );
}

function joinSucceeds() {
  server.use(
    http.post(`${BASE}/app/classes/join`, async ({ request }) => {
      const { joinCode } = (await request.json()) as { joinCode: string };
      joins.push(joinCode);
      enrolled = [...enrolled, JOINED];
      return contractJson("/app/classes/join", "post", 200, {
        ...sampleClass,
        id: CLASS_ID,
        name: CLASS_NAME,
      });
    }),
  );
}

function joinAnswers(answer: () => Response | Promise<Response>) {
  server.use(
    http.post(`${BASE}/app/classes/join`, async ({ request }) => {
      const { joinCode } = (await request.json()) as { joinCode: string };
      joins.push(joinCode);
      return answer();
    }),
  );
}

function Harness({ vanishing }: Readonly<{ vanishing: boolean }>) {
  const [open, setOpen] = useState(false);
  const classes = useQuery(myClassesQuery);
  useQuery({
    queryKey: ["my-assignments"],
    queryFn: ({ signal }) => listMyAssignments(signal),
  });
  const gone = vanishing && (classes.data?.items.length ?? 0) > 0;
  return (
    <DeckScale>
      <main tabIndex={-1}>
        {!gone && (
          <button type="button" onClick={() => setOpen(true)}>
            Mở
          </button>
        )}
        <ul>
          {classes.data?.items.map((c) => (
            <li key={c.id}>{c.name}</li>
          ))}
        </ul>
        <JoinDialog open={open} onOpenChange={setOpen} />
      </main>
      <Toaster />
    </DeckScale>
  );
}

function mount(vanishing = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Harness vanishing={vanishing} />
    </QueryClientProvider>,
  );
}

async function opened(vanishing = false) {
  const user = userEvent.setup();
  mount(vanishing);
  await user.click(screen.getByRole("button", { name: "Mở" }));
  const dialog = await screen.findByRole("dialog", { name: "Tham gia lớp" });
  return { user, dialog, in: within(dialog) };
}

const field = () => screen.getByLabelText("Mã lớp");
const submit = () => screen.getByRole("button", { name: "Tham gia" });
const rest = () => new Promise((resolve) => setTimeout(resolve, 400));

beforeEach(() => {
  previews = [];
  joins = [];
  asked = { classes: 0, assignments: 0 };
  enrolled = [];
  useAuthStore.getState().setSession("token", studentUser);
});
afterEach(() => {
  act(() => {
    toast.dismiss();
  });
  useAuthStore.getState().clearSession();
});

describe("the frame and the field", () => {
  it("opens on the code field, with no close button", async () => {
    lists();
    const { in: dialog } = await opened();
    expect(
      dialog.getByText("Nhập mã 8 ký tự giáo viên gửi cho bạn."),
    ).toBeInTheDocument();
    expect(dialog.queryByRole("button", { name: "Đóng" })).toBeNull();
    expect(dialog.getAllByRole("button").map((b) => b.textContent)).toEqual([
      "Huỷ",
      "Tham gia",
    ]);
    await waitFor(() => expect(field()).toHaveFocus());
  });

  it("is a deck surface with the dialog's 52px field", async () => {
    lists();
    const { dialog } = await opened();
    expect(dialog.dataset["scale"]).toBe("deck");
    expect(dialog).toHaveClass("top-[12%]", "min-[768px]:top-[50%]");
    expect(field()).toHaveClass("h-13", "text-xl", "rounded-[11px]");
    expect(field()).not.toHaveClass("h-[58px]");
  });

  it("groups what is typed and keeps a pasted code whole", async () => {
    lists();
    previewFinds();
    const { user } = await opened();
    expect(field()).toHaveAttribute("placeholder", "K7QM-2PXA");
    expect(field()).toHaveAttribute("maxlength", "9");
    await user.type(field(), "k7qm 2p");
    expect(field()).toHaveValue("K7QM-2P");
    await user.clear(field());
    await user.paste("k7qm - 2pxa");
    expect(field()).toHaveValue("K7QM-2PXA");
  });

  it("does not join an incomplete code", async () => {
    lists();
    joinSucceeds();
    const { user } = await opened();
    await user.type(field(), "K7QM");
    expect(submit()).toHaveAttribute("aria-disabled", "true");
    expect(submit()).toBeEnabled();
    await user.tab();
    await user.tab();
    expect(submit()).toHaveFocus();
    await user.click(submit());
    await user.type(field(), "{Enter}");
    await rest();
    expect(joins).toEqual([]);
    expect(previews).toEqual([]);
  });
});

describe("the lookup", () => {
  it("sends nothing while typing and one preview once the code rests", async () => {
    lists();
    previewFinds();
    const { user } = await opened();
    await user.type(field(), CODE);
    expect(previews).toEqual([]);
    await waitFor(() => expect(previews).toEqual([CODE]));
  });

  it("asks once per distinct code, found or refused", async () => {
    lists();
    previewFinds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    previewAnswers(() => refusal(404, "JOIN_CODE_INVALID", "Mã lớp không đúng."));
    await user.clear(field());
    await user.type(field(), OTHER);
    await screen.findByText(NOT_FOUND);
    await user.clear(field());
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.clear(field());
    await user.type(field(), OTHER);
    await screen.findByText(NOT_FOUND);
    await rest();
    expect(previews).toEqual([CODE, OTHER]);
  });

  it("costs nothing more when the dialog is closed and the code typed again", async () => {
    lists();
    previewFinds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await user.click(screen.getByRole("button", { name: "Mở" }));
    expect(field()).toHaveValue("");
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await rest();
    expect(previews).toEqual([CODE]);
  });

  it("asks once about a code retyped while its lookup is out", async () => {
    lists();
    server.use(
      http.post(`${BASE}/join/preview`, async ({ request }) => {
        const { joinCode } = (await request.json()) as { joinCode: string };
        previews.push(joinCode);
        await delay(300);
        return contractJson("/join/preview", "post", 200, {
          classId: CLASS_ID,
          className: CLASS_NAME,
          teacherName: TEACHER,
        });
      }),
    );
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    await waitFor(() => expect(previews).toEqual([CODE]));
    expect(dialog.getByRole("status")).toHaveTextContent("Đang tìm lớp…");
    await user.type(field(), "{Backspace}");
    await user.type(field(), "A");
    await screen.findByText(CLASS_NAME);
    await rest();
    expect(previews).toEqual([CODE]);
  });

  it("sends a lookup that breaks once, whatever the client retries by default", async () => {
    lists();
    previewAnswers(() => refusal(500, "INTERNAL", "Lỗi máy chủ."));
    const user = userEvent.setup();
    render(
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: 2, retryDelay: 0 } } })
        }
      >
        <Harness vanishing={false} />
      </QueryClientProvider>,
    );
    await user.click(screen.getByRole("button", { name: "Mở" }));
    await user.type(field(), CODE);
    await screen.findByRole("alert");
    await rest();
    expect(previews).toEqual([CODE]);
  });

  it("does not ask again by itself when the connection comes back", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      lists();
      previewFinds();
      const { user } = await opened();
      await user.type(field(), CODE);
      await screen.findByText(CLASS_NAME);
      await act(() => vi.advanceTimersByTimeAsync(31_000));
      act(() => onlineManager.setOnline(false));
      act(() => onlineManager.setOnline(true));
      await act(() => vi.advanceTimersByTimeAsync(500));
      expect(previews).toEqual([CODE]);
    } finally {
      onlineManager.setOnline(true);
      vi.useRealTimers();
    }
  });

  it("asks again about a code once its answer is 30 seconds old", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      lists();
      previewFinds();
      const { user } = await opened();
      await user.type(field(), CODE);
      await screen.findByText(CLASS_NAME);
      await user.clear(field());
      await act(() => vi.advanceTimersByTimeAsync(31_000));
      await user.type(field(), CODE);
      await waitFor(() => expect(previews).toEqual([CODE, CODE]));
    } finally {
      vi.useRealTimers();
    }
  });

  it("says a code never uses 0, O, 1 or I, and sends nothing", async () => {
    lists();
    previewFinds();
    const { user } = await opened();
    await user.type(field(), "K7Q0-2PXA");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Mã lớp không bao giờ có 0, O, 1 hoặc I.",
    );
    expect(field()).toHaveAttribute("aria-invalid", "true");
    await rest();
    expect(previews).toEqual([]);
  });
});

describe("what a code comes to", () => {
  it("shows the class and its teacher, and nothing the preview must not say", async () => {
    lists();
    previewFinds();
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    expect(await dialog.findByText(CLASS_NAME)).toBeInTheDocument();
    expect(dialog.getByText(TEACHER)).toBeInTheDocument();
    expect(dialog.getByText("Đã tìm thấy lớp")).toHaveClass("sr-only");
    expect(dialog.getByRole("status")).toHaveTextContent(
      `Đã tìm thấy lớp: ${CLASS_NAME}, ${TEACHER}`,
    );
    expect(dialog.queryByText(/học viên/)).toBeNull();
    expect(field()).toHaveClass("border-success");
    expect(submit()).not.toHaveAttribute("aria-disabled");
  });

  it("tells a member so, shows no class and joins nothing", async () => {
    enrolled = [JOINED];
    lists();
    previewFinds();
    joinSucceeds();
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    expect(await dialog.findByRole("alert")).toHaveTextContent(
      "Bạn đã ở trong lớp này rồi.",
    );
    expect(dialog.queryByText(CLASS_NAME)).toBeNull();
    expect(dialog.getByRole("status")).toBeEmptyDOMElement();
    expect(field()).toHaveClass("border-danger");
    expect(submit()).toHaveAttribute("aria-disabled", "true");
    await user.click(submit());
    await user.type(field(), "{Enter}");
    await rest();
    expect(joins).toEqual([]);
  });

  it("waits for the student's classes before saying which it is", async () => {
    lists("wait");
    previewFinds();
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    await waitFor(() => expect(previews).toEqual([CODE]));
    await rest();
    expect(dialog.queryByText(CLASS_NAME)).toBeNull();
    expect(dialog.queryByRole("alert")).toBeNull();
    expect(dialog.getByRole("status")).toHaveTextContent("Đang tìm lớp…");
    expect(submit()).toHaveAttribute("aria-disabled", "true");
  });

  it("lets a student join when the classes could not be read", async () => {
    lists("fail");
    previewFinds();
    joinSucceeds();
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    expect(await dialog.findByText(CLASS_NAME)).toBeInTheDocument();
    await user.click(submit());
    await waitFor(() => expect(joins).toEqual([CODE]));
  });

  it.each([
    ["JOIN_CODE_INVALID", "Mã lớp không đúng."],
    ["JOIN_CODE_EXPIRED", "Mã lớp đã hết hạn."],
    ["JOIN_CODE_EXHAUSTED", "Mã lớp đã dùng hết lượt."],
    ["JOIN_CODE_REVOKED", "Mã lớp đã bị thu hồi."],
  ])("answers %s with the one line that names no class", async (code, message) => {
    lists();
    previewAnswers(() => refusal(404, code, message));
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    expect(await dialog.findByRole("alert")).toHaveTextContent(NOT_FOUND);
    expect(dialog.queryByText(message)).toBeNull();
    expect(dialog.queryByText(CLASS_NAME)).toBeNull();
    expect(submit()).toHaveAttribute("aria-disabled", "true");
  });

  it("says how long to wait when the server says slow down", async () => {
    lists();
    previewAnswers(() =>
      refusal(429, "RATE_LIMITED", TOO_FAST, { "Retry-After": "6" }),
    );
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    expect(await dialog.findByRole("alert")).toHaveTextContent(
      `${TOO_FAST} Thử lại sau 6 giây.`,
    );
    await rest();
    expect(previews).toEqual([CODE]);
  });

  it("gives the server's sentence alone when it names no wait", async () => {
    lists();
    previewAnswers(() => refusal(429, "RATE_LIMITED", TOO_FAST));
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    const alert = await dialog.findByRole("alert");
    expect(alert).toHaveTextContent(TOO_FAST);
    expect(alert).not.toHaveTextContent(/giây/);
  });

  it("says the code could not be checked when the lookup breaks", async () => {
    lists();
    previewAnswers(() => refusal(500, "INTERNAL", "Lỗi máy chủ."));
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    expect(await dialog.findByRole("alert")).toHaveTextContent(
      "Chưa kiểm tra được mã. Vui lòng thử lại.",
    );
  });

  it("shows the class to an account that is not only a student, and joins nothing", async () => {
    useAuthStore.getState().setSession("token", adminWhoTakesTests);
    lists();
    previewFinds();
    joinSucceeds();
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    expect(await dialog.findByText(CLASS_NAME)).toBeInTheDocument();
    expect(
      dialog.getByText("Chỉ tài khoản học viên mới tham gia lớp được."),
    ).toBeInTheDocument();
    expect(submit()).toHaveAttribute("aria-disabled", "true");
    await user.click(submit());
    await rest();
    expect(joins).toEqual([]);
  });
});

describe("joining", () => {
  it("joins once, says so, refreshes both lists and closes", async () => {
    lists();
    previewFinds();
    joinSucceeds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    const before = { ...asked };
    expect(before.assignments).toBe(1);
    await user.dblClick(submit());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(joins).toEqual([CODE]);
    expect(await screen.findByText(`Bạn đã vào lớp ${CLASS_NAME}`)).toBeInTheDocument();
    expect(within(screen.getByRole("main")).getByRole("listitem")).toHaveTextContent(
      CLASS_NAME,
    );
    await waitFor(() =>
      expect(asked).toEqual({ classes: before.classes + 1, assignments: 2 }),
    );
  });

  it("waits for the class list before it says so and closes", async () => {
    lists();
    previewFinds();
    joinSucceeds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    let release = () => {};
    server.use(
      http.get(`${BASE}/app/classes`, async () => {
        await new Promise<void>((resolve) => {
          release = resolve;
        });
        return contractJson("/app/classes", "get", 200, { items: enrolled });
      }),
    );
    await user.click(submit());
    await waitFor(() => expect(joins).toEqual([CODE]));
    await rest();
    expect(submit()).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByText(`Bạn đã vào lớp ${CLASS_NAME}`)).toBeNull();
    release();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(await screen.findByText(`Bạn đã vào lớp ${CLASS_NAME}`)).toBeInTheDocument();
  });

  it("says so and closes when the class list does not come back", async () => {
    lists();
    previewFinds();
    joinSucceeds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    lists("wait");
    await user.click(submit());
    expect(
      await screen.findByText(`Bạn đã vào lớp ${CLASS_NAME}`, {}, { timeout: 4000 }),
    ).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("names the class as the join answered it", async () => {
    lists();
    previewFinds();
    joinAnswers(() =>
      contractJson("/app/classes/join", "post", 200, {
        ...sampleClass,
        id: CLASS_ID,
        name: "TOEIC 650 Weekend",
      }),
    );
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    expect(
      await screen.findByText("Bạn đã vào lớp TOEIC 650 Weekend"),
    ).toBeInTheDocument();
  });

  it("joins a second class without leaving the page", async () => {
    lists();
    previewFinds();
    joinSucceeds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    previewFinds(OTHER_ID);
    await user.click(screen.getByRole("button", { name: "Mở" }));
    await user.type(field(), OTHER);
    await within(await screen.findByRole("dialog")).findByText(CLASS_NAME);
    expect(submit()).not.toHaveAttribute("aria-busy");
    await user.click(submit());
    await waitFor(() => expect(joins).toEqual([CODE, OTHER]));
  });

  it("tells a student who retypes the code just joined that they are in the class", async () => {
    lists();
    previewFinds();
    joinSucceeds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await user.click(screen.getByRole("button", { name: "Mở" }));
    const dialog = within(await screen.findByRole("dialog"));
    await user.type(field(), CODE);
    expect(await dialog.findByRole("alert")).toHaveTextContent(
      "Bạn đã ở trong lớp này rồi.",
    );
    expect(dialog.queryByText(CLASS_NAME)).toBeNull();
    expect(submit()).toHaveAttribute("aria-disabled", "true");
    expect(submit()).not.toHaveAttribute("aria-busy");
    expect(joins).toEqual([CODE]);
  });

  it("does not carry a failed join over to another code", async () => {
    lists();
    previewFinds();
    joinAnswers(() => refusal(500, "INTERNAL", "Lỗi máy chủ."));
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    await dialog.findByRole("alert");
    await user.clear(field());
    await user.type(field(), OTHER);
    await waitFor(() => expect(previews).toEqual([CODE, OTHER]));
    await dialog.findByText(CLASS_NAME);
    expect(dialog.queryByRole("alert")).toBeNull();
  });

  it("gives the server's sentence alone when a refused join names no wait", async () => {
    lists();
    previewFinds();
    joinAnswers(() => refusal(429, "RATE_LIMITED", TOO_FAST));
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    const alert = await dialog.findByRole("alert");
    expect(alert).toHaveTextContent(TOO_FAST);
    expect(alert).not.toHaveTextContent(/giây/);
  });

  it("joins on Enter", async () => {
    lists();
    previewFinds();
    joinSucceeds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.type(field(), "{Enter}");
    await waitFor(() => expect(joins).toEqual([CODE]));
  });

  it("holds the button while the join is out and never says already a member", async () => {
    lists();
    previewFinds();
    let release = () => {};
    joinAnswers(async () => {
      await new Promise<void>((resolve) => {
        release = resolve;
      });
      enrolled = [JOINED];
      return contractJson("/app/classes/join", "post", 200, {
        ...sampleClass,
        id: CLASS_ID,
        name: CLASS_NAME,
      });
    });
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    await waitFor(() => expect(submit()).toHaveAttribute("aria-busy", "true"));
    await user.click(submit());
    const said: string[] = [];
    const watch = new MutationObserver(() => {
      const alert = dialog.queryByRole("alert");
      if (alert) said.push(alert.textContent);
    });
    watch.observe(document.body, {
      childList: true,
      subtree: true,
      characterData: true,
    });
    release();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    watch.disconnect();
    expect(said).toEqual([]);
    expect(joins).toEqual([CODE]);
  });

  it("answers a code that died after the preview with the one line, and stays open", async () => {
    lists();
    previewFinds();
    joinAnswers(() => refusal(404, "JOIN_CODE_REVOKED", "Mã lớp đã bị thu hồi."));
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    expect(await dialog.findByRole("alert")).toHaveTextContent(NOT_FOUND);
    expect(dialog.queryByText(CLASS_NAME)).toBeNull();
    expect(dialog.queryByText("Mã lớp đã bị thu hồi.")).toBeNull();
    expect(screen.queryByText(/Bạn đã vào lớp/)).toBeNull();
    expect(submit()).toHaveAttribute("aria-disabled", "true");
  });

  it("keeps the class and says how long to wait when the join is refused for speed", async () => {
    lists();
    previewFinds();
    joinAnswers(() => refusal(429, "RATE_LIMITED", TOO_FAST, { "Retry-After": "4" }));
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    expect(await dialog.findByRole("alert")).toHaveTextContent(
      `${TOO_FAST} Thử lại sau 4 giây.`,
    );
    expect(dialog.getByText(CLASS_NAME)).toBeInTheDocument();
    expect(submit()).not.toHaveAttribute("aria-busy");
  });

  it("says the join failed, keeps the class and lets the student try again", async () => {
    lists();
    previewFinds();
    joinAnswers(() => refusal(500, "INTERNAL", "Lỗi máy chủ."));
    const { user, in: dialog } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    expect(await dialog.findByRole("alert")).toHaveTextContent(
      "Không thể tham gia lớp. Vui lòng thử lại.",
    );
    expect(dialog.getByText(CLASS_NAME)).toBeInTheDocument();
    joinSucceeds();
    await user.click(submit());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(joins).toEqual([CODE, CODE]);
  });
});

describe("closing and focus", () => {
  it.each(["{Escape}", "Huỷ"])(
    "gives focus back to what opened it on %s",
    async (how) => {
      lists();
      const { user } = await opened();
      if (how === "Huỷ") await user.click(screen.getByRole("button", { name: "Huỷ" }));
      else await user.keyboard(how);
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      await waitFor(() =>
        expect(screen.getByRole("button", { name: "Mở" })).toHaveFocus(),
      );
    },
  );

  it.each(["{Escape}", "Huỷ"])(
    "lets a join that was left on %s finish without closing a dialog opened since",
    async (how) => {
      lists();
      previewFinds();
      let release = () => {};
      joinAnswers(async () => {
        await new Promise<void>((resolve) => {
          release = resolve;
        });
        enrolled = [JOINED];
        return contractJson("/app/classes/join", "post", 200, {
          ...sampleClass,
          id: CLASS_ID,
          name: CLASS_NAME,
        });
      });
      const { user } = await opened();
      await user.type(field(), CODE);
      await screen.findByText(CLASS_NAME);
      await user.click(submit());
      await waitFor(() => expect(joins).toEqual([CODE]));
      if (how === "Huỷ") await user.click(screen.getByRole("button", { name: "Huỷ" }));
      else await user.keyboard(how);
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      await user.click(screen.getByRole("button", { name: "Mở" }));
      await user.type(field(), "W3RT");
      release();
      expect(
        await screen.findByText(`Bạn đã vào lớp ${CLASS_NAME}`),
      ).toBeInTheDocument();
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      expect(field()).toHaveValue("W3RT");
    },
  );

  it("forgets a failed join when it is cancelled and reopened", async () => {
    lists();
    previewFinds();
    joinAnswers(() => refusal(500, "INTERNAL", "Lỗi máy chủ."));
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await user.click(screen.getByRole("button", { name: "Mở" }));
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(joins).toEqual([CODE]);
  });

  it("forgets a failed join when it is reopened", async () => {
    lists();
    previewFinds();
    joinAnswers(() => refusal(500, "INTERNAL", "Lỗi máy chủ."));
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    await screen.findByRole("alert");
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await user.click(screen.getByRole("button", { name: "Mở" }));
    expect(field()).toHaveValue("");
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("returns to the trigger after a join when it is still there", async () => {
    lists();
    previewFinds();
    joinSucceeds();
    const { user } = await opened();
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Mở" })).toHaveFocus(),
    );
  });

  it("lands on the page after a join when the trigger has gone", async () => {
    lists();
    previewFinds();
    joinSucceeds();
    const { user } = await opened(true);
    await user.type(field(), CODE);
    await screen.findByText(CLASS_NAME);
    await user.click(submit());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.queryByRole("button", { name: "Mở" })).toBeNull();
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });
});
