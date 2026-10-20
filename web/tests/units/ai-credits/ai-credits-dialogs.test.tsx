import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import { onlineManager } from "@tanstack/react-query";
import { delay, http, HttpResponse } from "msw";
import { toast } from "@/components/ui/sonner";
import i18n from "@/lib/i18n";
import { server } from "@tests/support/server";
import {
  BASE,
  bao,
  credits,
  giau,
  renderCreditsPage,
  serveCredits,
  type Credits,
  type Teacher,
} from "@tests/support/aiCredits";
import { assertContractRequest, contractJson } from "@tests/support/contractResponse";

const ALLOWANCE = "/admin/ai/teachers/{userId}/allowance";
const ADDITIONS = "/admin/ai/credits/additions";
const RULES = "/admin/ai/rules";

beforeEach(() => {
  Element.prototype.scrollTo = vi.fn();
});

afterEach(async () => {
  toast.dismiss();
  await i18n.changeLanguage("vi");
  vi.restoreAllMocks();
});

function twice(element: HTMLElement) {
  act(() => {
    element.click();
    element.click();
  });
}

function serveAllowance(seen: { view: Credits }) {
  const sent: { userId: string; body: unknown }[] = [];
  server.use(
    http.put(
      `${BASE}/admin/ai/teachers/:userId/allowance`,
      async ({ params, request }) => {
        const body = await request.json();
        assertContractRequest(ALLOWANCE, "put", body);
        const userId = String(params["userId"]);
        sent.push({ userId, body });
        const input = body as { custom?: boolean; credits?: number; paused?: boolean };
        const current = seen.view.teachers.find((row) => row.userId === userId)!;
        const custom = input.custom ?? current.custom;
        const granted = custom ? current.granted : 0;
        const next: Teacher = {
          ...current,
          custom,
          paused: input.paused ?? current.paused,
          allowance: custom
            ? (input.credits ?? current.allowance - current.granted) + granted
            : seen.view.rules.defaultAllowance,
          granted,
        };
        seen.view = {
          ...seen.view,
          teachers: seen.view.teachers.map((row) =>
            row.userId === userId ? next : row,
          ),
        };
        return contractJson(ALLOWANCE, "put", 200, next);
      },
    ),
  );
  return sent;
}

async function openAllowance(
  user: ReturnType<typeof renderCreditsPage>["user"],
  name: string,
) {
  await user.click(
    await screen.findByRole("button", { name: `Sửa hạn mức của ${name}` }),
  );
  return screen.findByRole("dialog", { name: `Hạn mức AI · ${name}` });
}

describe("the allowance dialog", () => {
  it("opens on the default allowance with the month used and no figure to type", async () => {
    serveCredits();
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    expect(
      within(dialog).getByText("Đã dùng 18.420 tín dụng trong tháng này."),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Mặc định · 50.000" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(within(dialog).getByRole("button", { name: "Tùy chỉnh" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(within(dialog).queryByRole("spinbutton")).toBeNull();
    expect(
      within(dialog).getByRole("switch", { name: "Tạm dừng AI cho giáo viên này" }),
    ).not.toBeChecked();
  });

  it("opens a custom teacher on their own figure, less what approved requests added", async () => {
    const view = credits({
      teachers: credits().teachers.map((row) =>
        row.userId === bao.userId ? { ...row, allowance: 100000, granted: 20000 } : row,
      ),
    });
    serveCredits(view);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, bao.name);
    expect(within(dialog).getByRole("button", { name: "Tùy chỉnh" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      within(dialog).getByRole("spinbutton", { name: "Số tín dụng mỗi tháng" }),
    ).toHaveValue(80000);
  });

  it("sends a custom figure, closes, tells the Admin and shows the row's new allowance", async () => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    await user.click(within(dialog).getByRole("button", { name: "Tùy chỉnh" }));
    const field = within(dialog).getByRole("spinbutton", {
      name: "Số tín dụng mỗi tháng",
    });
    expect(field).toHaveValue(50000);
    await user.clear(field);
    await user.type(field, "75000");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    expect(await screen.findByText("Đã lưu hạn mức của Giàu Bùi")).toBeVisible();
    expect(sent).toEqual([
      { userId: giau.userId, body: { custom: true, credits: 75000 } },
    ]);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(await screen.findByText("trên 75.000 · tùy chỉnh")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Sửa hạn mức của Giàu Bùi" }),
    ).toHaveFocus();
  });

  it("pauses a teacher with the switch alone, sending only the pause", async () => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    await user.click(
      within(dialog).getByRole("switch", { name: "Tạm dừng AI cho giáo viên này" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    await screen.findByText("Đã lưu hạn mức của Giàu Bùi");
    expect(sent).toEqual([{ userId: giau.userId, body: { paused: true } }]);
  });

  it("returns a custom teacher to the default and sends no figure", async () => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, bao.name);
    await user.click(within(dialog).getByRole("button", { name: "Mặc định · 50.000" }));
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    await screen.findByText("Đã lưu hạn mức của Đinh Quốc Bảo");
    expect(sent).toEqual([{ userId: bao.userId, body: { custom: false } }]);
    expect(await screen.findAllByText("trên 50.000")).not.toHaveLength(0);
    expect(screen.queryByText("trên 80.000 · tùy chỉnh")).toBeNull();
  });

  it("sends nothing when nothing changed, and closes", async () => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent).toEqual([]);
    expect(screen.queryByText("Đã lưu hạn mức của Giàu Bùi")).toBeNull();
  });

  it("sends one save for two presses in the same frame", async () => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    await user.click(
      within(dialog).getByRole("switch", { name: "Tạm dừng AI cho giáo viên này" }),
    );
    twice(within(dialog).getByRole("button", { name: "Lưu" }));
    await screen.findByText("Đã lưu hạn mức của Giàu Bùi");
    expect(sent).toEqual([{ userId: giau.userId, body: { paused: true } }]);
  });

  it("never undoes what another Admin set after it opened when nothing was changed here", async () => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    seen.view = {
      ...seen.view,
      teachers: seen.view.teachers.map((row) =>
        row.userId === giau.userId
          ? { ...row, custom: true, allowance: 77000, paused: true }
          : row,
      ),
    };
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent).toEqual([]);
    expect(screen.queryByText("Đã lưu hạn mức của Giàu Bùi")).toBeNull();
  });

  it("saves again after a save that changed nothing", async () => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    let dialog = await openAllowance(user, giau.name);
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    dialog = await openAllowance(user, giau.name);
    await user.click(
      within(dialog).getByRole("switch", { name: "Tạm dừng AI cho giáo viên này" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    await screen.findByText("Đã lưu hạn mức của Giàu Bùi");
    expect(sent).toEqual([{ userId: giau.userId, body: { paused: true } }]);
  });

  it.each([
    ["999", "Nhập ít nhất 1.000"],
    ["", "Nhập ít nhất 1.000"],
    ["10000001", "Nhập tối đa 10.000.000"],
    ["1500.5", "Nhập một số nguyên"],
  ])("refuses %j with its sentence and sends nothing", async (typed, sentence) => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    await user.click(within(dialog).getByRole("button", { name: "Tùy chỉnh" }));
    const field = within(dialog).getByRole("spinbutton", {
      name: "Số tín dụng mỗi tháng",
    });
    await user.clear(field);
    if (typed !== "") await user.type(field, typed);
    expect(within(dialog).queryByText(sentence)).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    expect(await within(dialog).findByText(sentence)).toBeVisible();
    expect(field).toHaveAttribute("aria-invalid", "true");
    expect(sent).toEqual([]);
    expect(screen.getByRole("dialog")).toBeVisible();
  });

  it("accepts the least and the most, 1,000 and 10,000,000", async () => {
    const seen = serveCredits();
    const sent = serveAllowance(seen);
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    await user.click(within(dialog).getByRole("button", { name: "Tùy chỉnh" }));
    const field = within(dialog).getByRole("spinbutton", {
      name: "Số tín dụng mỗi tháng",
    });
    await user.clear(field);
    await user.type(field, "1000");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    await screen.findByText("Đã lưu hạn mức của Giàu Bùi");
    const again = await openAllowance(user, giau.name);
    const second = within(again).getByRole("spinbutton", {
      name: "Số tín dụng mỗi tháng",
    });
    await user.clear(second);
    await user.type(second, "10000000");
    await user.click(within(again).getByRole("button", { name: "Lưu" }));
    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent.map((call) => call.body)).toEqual([
      { custom: true, credits: 1000 },
      { custom: true, credits: 10000000 },
    ]);
  });

  it("keeps the dialog open with the server's sentence when it refuses, and Cancel discards", async () => {
    serveCredits();
    server.use(
      http.put(`${BASE}/admin/ai/teachers/:userId/allowance`, () =>
        HttpResponse.json(
          {
            error: {
              code: "VALIDATION_FAILED",
              message: "Hạn mức không được thấp hơn mức đã dùng.",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 422 },
        ),
      ),
    );
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    await user.click(within(dialog).getByRole("button", { name: "Tùy chỉnh" }));
    const field = within(dialog).getByRole("spinbutton", {
      name: "Số tín dụng mỗi tháng",
    });
    await user.clear(field);
    await user.type(field, "5000");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Hạn mức không được thấp hơn mức đã dùng.",
    );
    expect(screen.getByRole("dialog")).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    const reopened = await openAllowance(user, giau.name);
    expect(within(reopened).queryByRole("alert")).toBeNull();
    expect(within(reopened).queryByRole("spinbutton")).toBeNull();
  });

  it("answers a second press while the save runs with nothing and keeps focus on Save", async () => {
    const seen = serveCredits();
    let calls = 0;
    let release!: () => void;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.put(`${BASE}/admin/ai/teachers/:userId/allowance`, async () => {
        calls += 1;
        await held;
        return contractJson(ALLOWANCE, "put", 200, {
          ...giau,
          paused: true,
          status: "paused",
        });
      }),
    );
    const { user } = renderCreditsPage();
    const dialog = await openAllowance(user, giau.name);
    await user.click(
      within(dialog).getByRole("switch", { name: "Tạm dừng AI cho giáo viên này" }),
    );
    const save = within(dialog).getByRole("button", { name: "Lưu" });
    await user.click(save);
    await waitFor(() => expect(save).toHaveAttribute("aria-disabled", "true"));
    await user.click(save);
    expect(calls).toBe(1);
    expect(save).toHaveFocus();
    release();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(seen.reads).toBeGreaterThan(0);
  });

  it("reads in English", async () => {
    await i18n.changeLanguage("en");
    serveCredits();
    const { user } = renderCreditsPage();
    await user.click(
      await screen.findByRole("button", { name: "Edit allowance for Giàu Bùi" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "AI allowance · Giàu Bùi",
    });
    expect(
      within(dialog).getByText("18,420 credits used so far this month."),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Default · 50,000" }),
    ).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Custom" }));
    expect(
      within(dialog).getByText("Takes effect right away. Usage so far still counts."),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("switch", { name: "Pause AI for this teacher" }),
    ).toBeVisible();
  });
});

describe("the add credits dialog", () => {
  async function openAdd(user: ReturnType<typeof renderCreditsPage>["user"]) {
    await user.click(await screen.findByRole("button", { name: "Thêm tín dụng" }));
    return screen.findByRole("dialog", { name: "Thêm tín dụng cho tháng này" });
  }

  it("offers three amounts, 50,000 first, and says the extra is paid for outside Quizzivy", async () => {
    serveCredits();
    const { user } = renderCreditsPage();
    const dialog = await openAdd(user);
    expect(
      within(dialog).getByText(
        "Được thêm vào quỹ chung của trung tâm ngay. Phần thêm do trung tâm thanh toán ngoài Quizzivy. Tín dụng thêm vào không chuyển sang tháng sau.",
      ),
    ).toBeVisible();
    const amounts = within(dialog).getByRole("group", { name: "Số lượng" });
    expect(
      within(amounts)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["50.000", "100.000", "250.000"]);
    expect(within(amounts).getByRole("button", { name: "50.000" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(document.body).not.toHaveTextContent(/hóa đơn|invoice/i);
  });

  it("adds the chosen amount: the pool grows at once, the Admin is told and the dialog closes", async () => {
    const seen = serveCredits();
    const sent: unknown[] = [];
    server.use(
      http.post(`${BASE}/admin/ai/credits/additions`, async ({ request }) => {
        const body = await request.json();
        assertContractRequest(ADDITIONS, "post", body);
        sent.push(body);
        const pool = {
          plan: 200000,
          added: 100000,
          used: 124840,
          reserved: 0,
          total: 300000,
          left: 175160,
        };
        seen.view = credits({ pool });
        return contractJson(ADDITIONS, "post", 201, pool);
      }),
    );
    const { user } = renderCreditsPage();
    const dialog = await openAdd(user);
    await user.click(within(dialog).getByRole("button", { name: "100.000" }));
    await user.click(within(dialog).getByRole("button", { name: "Thêm tín dụng" }));
    expect(
      await screen.findByText("Đã thêm 100.000 tín dụng vào quỹ chung"),
    ).toBeVisible();
    expect(sent).toEqual([{ credits: 100000 }]);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(await screen.findByText("trên 300.000 tín dụng đã dùng")).toBeVisible();
    expect(screen.getByRole("button", { name: "Thêm tín dụng" })).toHaveFocus();
  });

  it("starts from 50,000 each time it opens", async () => {
    serveCredits();
    const { user } = renderCreditsPage();
    const dialog = await openAdd(user);
    await user.click(within(dialog).getByRole("button", { name: "250.000" }));
    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    const reopened = await openAdd(user);
    expect(within(reopened).getByRole("button", { name: "50.000" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("shows why it could not add, leaves the pool alone and lets the Admin try again", async () => {
    serveCredits();
    server.use(
      http.post(`${BASE}/admin/ai/credits/additions`, () =>
        HttpResponse.json(
          {
            error: {
              code: "FORBIDDEN",
              message: "Bạn không có quyền thao tác trên tài khoản này.",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 403 },
        ),
      ),
    );
    const { user } = renderCreditsPage();
    const dialog = await openAdd(user);
    await user.click(within(dialog).getByRole("button", { name: "Thêm tín dụng" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Bạn không có quyền thao tác trên tài khoản này.",
    );
    expect(screen.getByRole("dialog")).toBeVisible();
    expect(screen.getByText("trên 200.000 tín dụng đã dùng")).toBeVisible();
  });

  it("sends the retry after a refusal", async () => {
    serveCredits();
    let calls = 0;
    server.use(
      http.post(`${BASE}/admin/ai/credits/additions`, () => {
        calls += 1;
        if (calls === 1)
          return HttpResponse.json(
            {
              error: {
                code: "INTERNAL",
                message: "Máy chủ gặp sự cố.",
                requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
              },
            },
            { status: 500 },
          );
        return contractJson(ADDITIONS, "post", 201, {
          plan: 200000,
          added: 50000,
          used: 124840,
          reserved: 0,
          total: 250000,
          left: 125160,
        });
      }),
    );
    const { user } = renderCreditsPage();
    const dialog = await openAdd(user);
    const submit = within(dialog).getByRole("button", { name: "Thêm tín dụng" });
    await user.click(submit);
    await within(dialog).findByRole("alert");
    await waitFor(() => expect(submit).not.toHaveAttribute("aria-disabled"));
    await user.click(submit);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(calls).toBe(2);
  });

  it("holds a second addition until one closed in flight has answered, and still says it was added", async () => {
    serveCredits();
    let calls = 0;
    let release!: () => void;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.post(`${BASE}/admin/ai/credits/additions`, async () => {
        calls += 1;
        await held;
        return contractJson(ADDITIONS, "post", 201, {
          plan: 200000,
          added: 50000,
          used: 124840,
          reserved: 0,
          total: 250000,
          left: 125160,
        });
      }),
    );
    const { user } = renderCreditsPage();
    let dialog = await openAdd(user);
    await user.click(within(dialog).getByRole("button", { name: "Thêm tín dụng" }));
    await waitFor(() => expect(calls).toBe(1));
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    dialog = await openAdd(user);
    await user.click(within(dialog).getByRole("button", { name: "Thêm tín dụng" }));
    expect(calls).toBe(1);
    release();
    expect(
      await screen.findByText("Đã thêm 50.000 tín dụng vào quỹ chung"),
    ).toBeVisible();
    expect(calls).toBe(1);
  });

  it("sends one request for a double press", async () => {
    serveCredits();
    let calls = 0;
    server.use(
      http.post(`${BASE}/admin/ai/credits/additions`, async () => {
        calls += 1;
        await delay(50);
        return contractJson(ADDITIONS, "post", 201, {
          plan: 200000,
          added: 50000,
          used: 124840,
          reserved: 0,
          total: 250000,
          left: 125160,
        });
      }),
    );
    const { user } = renderCreditsPage();
    const dialog = await openAdd(user);
    const submit = within(dialog).getByRole("button", { name: "Thêm tín dụng" });
    await user.click(submit);
    await user.click(submit);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(calls).toBe(1);
  });

  it("sends one request for two presses in the same frame", async () => {
    serveCredits();
    let calls = 0;
    server.use(
      http.post(`${BASE}/admin/ai/credits/additions`, () => {
        calls += 1;
        return contractJson(ADDITIONS, "post", 201, {
          plan: 200000,
          added: 50000,
          used: 124840,
          reserved: 0,
          total: 250000,
          left: 125160,
        });
      }),
    );
    const { user } = renderCreditsPage();
    const dialog = await openAdd(user);
    twice(within(dialog).getByRole("button", { name: "Thêm tín dụng" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(calls).toBe(1);
  });

  it("says it could not add while offline instead of waiting to send it later", async () => {
    serveCredits();
    let calls = 0;
    server.use(
      http.post(`${BASE}/admin/ai/credits/additions`, () => {
        calls += 1;
        return HttpResponse.error();
      }),
    );
    const { user } = renderCreditsPage();
    const dialog = await openAdd(user);
    onlineManager.setOnline(false);
    try {
      await user.click(within(dialog).getByRole("button", { name: "Thêm tín dụng" }));
      expect(await within(dialog).findByRole("alert")).toBeVisible();
      expect(calls).toBe(1);
    } finally {
      onlineManager.setOnline(true);
    }
  });

  it("says in English what the deck's invoice sentence became (DG-196)", async () => {
    await i18n.changeLanguage("en");
    serveCredits();
    const { user } = renderCreditsPage();
    await user.click(await screen.findByRole("button", { name: "Add credits" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Add credits to this month",
    });
    expect(
      within(dialog).getByText(
        "Added to the centre pool right away. The extra is paid for outside Quizzivy. Added credits don’t carry over to next month.",
      ),
    ).toBeVisible();
    expect(document.body).not.toHaveTextContent(/invoice/i);
  });
});

describe("the rules card", () => {
  const rule = (name: string) => screen.findByRole("group", { name });

  function serveRules(seen: { view: Credits }, rewritten = 0) {
    const sent: unknown[] = [];
    server.use(
      http.patch(`${BASE}/admin/ai/rules`, async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        assertContractRequest(RULES, "patch", body);
        sent.push(body);
        const rules = { ...seen.view.rules, ...body };
        seen.view = { ...seen.view, rules };
        return contractJson(RULES, "patch", 200, { rules, rewritten });
      }),
    );
    return sent;
  }

  it("shows the rules in force: the allowance, the warning and what a teacher who runs out may do", async () => {
    serveCredits();
    renderCreditsPage();
    const allowance = await rule("Hạn mức hằng tháng của mỗi giáo viên");
    expect(
      within(allowance)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual(["20.000", "50.000", "100.000"]);
    expect(within(allowance).getByRole("button", { name: "50.000" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    const warn = await rule("Cảnh báo giáo viên khi dùng đến");
    expect(within(warn).getByRole("button", { name: "80%" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(within(warn).getByRole("button", { name: "90%" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    const limit = await rule("Khi giáo viên dùng hết hạn mức");
    expect(
      within(limit).getByRole("button", { name: "Cho phép xin thêm" }),
    ).toHaveAttribute("aria-pressed", "true");
  });

  it("has one rule for the pool, a fixed line with nothing to choose", async () => {
    serveCredits();
    renderCreditsPage();
    const card = (await screen.findByRole("region", { name: "Quy tắc" })).closest(
      "section",
    )!;
    expect(within(card).getAllByRole("group")).toHaveLength(3);
    expect(within(card).getByText("Khi quỹ chung hết")).toBeVisible();
    expect(within(card).getByText("Tạm dừng AI cho mọi người")).toBeVisible();
    expect(
      within(card).getByText("Muốn cho dùng thêm, hãy thêm tín dụng."),
    ).toBeVisible();
    expect(within(card).getAllByRole("button")).toHaveLength(7);
    expect(within(card).queryByText(/10%/)).toBeNull();
  });

  it("saves a choice as it is made: the option is shown at once, the card says so and the month is read again", async () => {
    const seen = serveCredits();
    const sent = serveRules(seen, 6);
    const { user } = renderCreditsPage();
    const allowance = await rule("Hạn mức hằng tháng của mỗi giáo viên");
    await user.click(within(allowance).getByRole("button", { name: "100.000" }));
    expect(await screen.findByText("Đã lưu quy tắc")).toBeVisible();
    expect(sent).toEqual([{ defaultAllowance: 100000 }]);
    await waitFor(() => expect(seen.reads).toBeGreaterThan(1));
    expect(
      within(await rule("Hạn mức hằng tháng của mỗi giáo viên")).getByRole("button", {
        name: "100.000",
      }),
    ).toHaveAttribute("aria-pressed", "true");
  });

  it("sends the warning and the limit rule as their own changes", async () => {
    const seen = serveCredits();
    const sent = serveRules(seen);
    const { user } = renderCreditsPage();
    await user.click(
      within(await rule("Cảnh báo giáo viên khi dùng đến")).getByRole("button", {
        name: "90%",
      }),
    );
    await screen.findByText("Đã lưu quy tắc");
    await user.click(
      within(await rule("Khi giáo viên dùng hết hạn mức")).getByRole("button", {
        name: "Tạm dừng đến tháng sau",
      }),
    );
    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent).toEqual([{ warnPercent: 90 }, { onTeacherLimit: "pause" }]);
  });

  it("holds every option while a save runs and answers a second press with nothing", async () => {
    serveCredits();
    let calls = 0;
    let release!: () => void;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.patch(`${BASE}/admin/ai/rules`, async () => {
        calls += 1;
        await held;
        return contractJson(RULES, "patch", 200, {
          rules: { ...credits().rules, defaultAllowance: 20000 },
          rewritten: 0,
        });
      }),
    );
    const { user } = renderCreditsPage();
    const allowance = await rule("Hạn mức hằng tháng của mỗi giáo viên");
    const twenty = within(allowance).getByRole("button", { name: "20.000" });
    await user.click(twenty);
    await waitFor(() => expect(twenty).toHaveAttribute("aria-disabled", "true"));
    expect(twenty).toHaveAttribute("aria-pressed", "true");
    expect(within(allowance).getByRole("button", { name: "50.000" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    const warn = within(await rule("Cảnh báo giáo viên khi dùng đến")).getByRole(
      "button",
      { name: "90%" },
    );
    expect(warn).toHaveAttribute("aria-disabled", "true");
    await user.click(warn);
    await user.click(twenty);
    expect(calls).toBe(1);
    release();
    await screen.findByText("Đã lưu quy tắc");
    await waitFor(() => expect(twenty).not.toHaveAttribute("aria-disabled"));
  });

  it("sends one change for two presses in the same frame", async () => {
    const seen = serveCredits();
    const sent = serveRules(seen);
    renderCreditsPage();
    const allowance = await rule("Hạn mức hằng tháng của mỗi giáo viên");
    twice(within(allowance).getByRole("button", { name: "100.000" }));
    expect(await screen.findByText("Đã lưu quy tắc")).toBeVisible();
    expect(sent).toEqual([{ defaultAllowance: 100000 }]);
  });

  it("puts the rule in force back and says why when the server refuses", async () => {
    serveCredits();
    server.use(
      http.patch(`${BASE}/admin/ai/rules`, () =>
        HttpResponse.json(
          {
            error: {
              code: "FORBIDDEN",
              message: "Bạn không có quyền thao tác trên tài khoản này.",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 403 },
        ),
      ),
    );
    const { user } = renderCreditsPage();
    const allowance = await rule("Hạn mức hằng tháng của mỗi giáo viên");
    await user.click(within(allowance).getByRole("button", { name: "20.000" }));
    expect(
      await screen.findByText("Bạn không có quyền thao tác trên tài khoản này."),
    ).toBeVisible();
    await waitFor(() =>
      expect(within(allowance).getByRole("button", { name: "50.000" })).toHaveAttribute(
        "aria-pressed",
        "true",
      ),
    );
    expect(within(allowance).getByRole("button", { name: "20.000" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("shows a figure the buttons do not offer, as the maintenance command left it", async () => {
    serveCredits(
      credits({
        rules: {
          poolCredits: 200000,
          defaultAllowance: 75000,
          warnPercent: 85,
          onTeacherLimit: "pause",
        },
      }),
    );
    renderCreditsPage();
    const allowance = await rule("Hạn mức hằng tháng của mỗi giáo viên");
    expect(
      within(allowance)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual(["20.000", "50.000", "75.000", "100.000"]);
    expect(within(allowance).getByRole("button", { name: "75.000" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    const warn = await rule("Cảnh báo giáo viên khi dùng đến");
    expect(within(warn).getByRole("button", { name: "85%" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      within(await rule("Khi giáo viên dùng hết hạn mức")).getByRole("button", {
        name: "Tạm dừng đến tháng sau",
      }),
    ).toHaveAttribute("aria-pressed", "true");
  });

  it("reads in English", async () => {
    await i18n.changeLanguage("en");
    serveCredits();
    renderCreditsPage();
    const card = (await screen.findByRole("region", { name: "Rules" })).closest(
      "section",
    )!;
    expect(
      within(card).getByText("For every teacher without a custom allowance."),
    ).toBeVisible();
    expect(within(card).getByText("Pause AI for everyone")).toBeVisible();
    expect(
      within(card).getByRole("button", { name: "Let them ask for more" }),
    ).toHaveAttribute("aria-pressed", "true");
  });
});
