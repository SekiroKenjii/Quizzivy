import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { toast } from "@/components/ui/sonner";
import { delay, http, HttpResponse } from "msw";
import i18n from "@/lib/i18n";
import { server } from "@tests/support/server";
import {
  BASE,
  credits,
  duc,
  ducRequest,
  renderCreditsPage,
  serveCredits,
  serveOff,
} from "@tests/support/aiCredits";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";

const POOL = "Quỹ chung · Tháng 9";
const REQUESTS = "Yêu cầu thêm tín dụng đang chờ duyệt";

beforeEach(() => {
  Element.prototype.scrollTo = vi.fn();
});

afterEach(async () => {
  toast.dismiss();
  await i18n.changeLanguage("vi");
  vi.restoreAllMocks();
});

const fill = () =>
  document.querySelector<HTMLElement>("[data-slot='pool-fill']") as HTMLElement;
const tick = () => document.querySelector<HTMLElement>("[data-slot='pool-projection']");

describe("the header", () => {
  it("names the page, says when usage resets and offers Add credits once the month has loaded", async () => {
    serveCredits();
    renderCreditsPage();
    expect(
      screen.getByRole("heading", { level: 1, name: "Tín dụng AI" }),
    ).toBeVisible();
    expect(screen.queryByRole("button", { name: "Thêm tín dụng" })).toBeNull();
    expect(
      await screen.findByText(
        "Hạn mức hằng tháng cho các tính năng AI. Mức dùng được làm mới vào 1 tháng 10.",
      ),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Thêm tín dụng" })).toBeVisible();
  });

  it("writes the reset date in English as the deck does", async () => {
    await i18n.changeLanguage("en");
    serveCredits();
    renderCreditsPage();
    expect(
      await screen.findByText(
        "Monthly allowances for AI features. Usage resets on 1 October.",
      ),
    ).toBeVisible();
  });
});

describe("the pool card", () => {
  it("draws the month: the plan, what was used of the pool, the bar and three facts", async () => {
    serveCredits();
    renderCreditsPage();
    const card = (await screen.findByRole("region", { name: POOL })).closest(
      "section",
    )!;
    const within_ = within(card);
    expect(
      within_.getByText("Gói của trung tâm 200.000 · làm mới vào 1 tháng 10"),
    ).toBeVisible();
    expect(within_.getByText("124.840")).toBeVisible();
    expect(within_.getByText("trên 200.000 tín dụng đã dùng")).toBeVisible();
    expect(
      within_.getByRole("progressbar", {
        name: "Đã dùng 124.840 trên 200.000 tín dụng",
      }),
    ).toHaveAttribute("aria-valuenow", "62");
    expect(within_.getByText("Còn lại tháng này").nextElementSibling).toHaveTextContent(
      "75.160",
    );
    expect(within_.getByText("còn 6 ngày")).toBeVisible();
    expect(within_.getByText("~156.100")).toBeVisible();
    expect(
      within_.getByText("đến 30 tháng 9, trong giới hạn của quỹ chung"),
    ).toBeVisible();
    expect(within_.getByText("300.000")).toBeVisible();
    expect(
      within_.getByText(
        "Tổng có thể vượt quỹ chung, vì ít giáo viên dùng hết hạn mức của mình",
      ),
    ).toBeVisible();
  });

  it("writes what an Admin added after the plan", async () => {
    serveCredits(
      credits({
        pool: {
          plan: 200000,
          added: 50000,
          used: 100,
          reserved: 0,
          total: 250000,
          left: 249900,
        },
      }),
    );
    renderCreditsPage();
    expect(
      await screen.findByText(
        "Gói của trung tâm 200.000 + 50.000 đã thêm · làm mới vào 1 tháng 10",
      ),
    ).toBeVisible();
    expect(screen.getByText("trên 250.000 tín dụng đã dùng")).toBeVisible();
  });

  it.each([
    [148000, "bg-brand", 74],
    [150000, "bg-warning", 75],
    [178000, "bg-warning", 89],
    [180000, "bg-danger", 90],
  ])("fills the bar for %i credits (%i%) with %s", async (used, tone) => {
    serveCredits(
      credits({
        pool: {
          plan: 200000,
          added: 0,
          used,
          reserved: 0,
          total: 200000,
          left: 200000 - used,
        },
      }),
    );
    renderCreditsPage();
    await screen.findByRole("region", { name: POOL });
    expect(fill()).toHaveClass(tone);
    expect(fill().style.width).toBe(`${(used / 200000) * 100}%`);
  });

  it("marks the server's projection on the bar and never the browser's", async () => {
    serveCredits(credits({ onPace: 100000 }));
    renderCreditsPage();
    await screen.findByRole("region", { name: POOL });
    expect(tick()).not.toBeNull();
    expect(tick()!.style.left).toBe("50%");
    expect(tick()!.getAttribute("title")).toBe("Dự kiến đến 30 tháng 9");
    expect(screen.getByText("~100.000")).toBeVisible();
  });

  it("pins the tick inside the bar and warns when the month is on pace to pass the pool", async () => {
    serveCredits(credits({ onPace: 480000, withinPool: false }));
    renderCreditsPage();
    await screen.findByRole("region", { name: POOL });
    expect(tick()!.style.left).toBe("99.5%");
    const warning = screen.getByText(
      "Vượt quỹ chung. Hãy thêm tín dụng hoặc giảm hạn mức.",
    );
    expect(warning).toHaveClass("text-warning-ink");
    expect(screen.getByText("~480.000")).toBeVisible();
  });

  it("draws no tick before anything has been used, and says so on the last day of the month", async () => {
    serveCredits(
      credits({
        onPace: 0,
        daysToGo: 0,
        pool: {
          plan: 200000,
          added: 0,
          used: 0,
          reserved: 0,
          total: 200000,
          left: 200000,
        },
      }),
    );
    renderCreditsPage();
    await screen.findByRole("region", { name: POOL });
    expect(tick()).toBeNull();
    expect(screen.getByText("Hôm nay là ngày cuối tháng")).toBeVisible();
  });
});

describe("the requests", () => {
  it("draws one card per request with who asked, how much, the reason and its age", async () => {
    serveCredits();
    renderCreditsPage();
    const list = await screen.findByRole("list", { name: REQUESTS });
    const card = within(list).getByRole("listitem");
    expect(
      within(card).getByText(
        "Phan Minh Đức xin thêm 20.000 tín dụng · đã dùng 41.300 trên 50.000",
      ),
    ).toBeVisible();
    expect(
      within(card).getByText(`“${ducRequest.reason}” · 2 giờ trước`),
    ).toBeVisible();
    expect(within(card).getByRole("button", { name: "Từ chối" })).toBeVisible();
    expect(within(card).getByRole("button", { name: "Duyệt 20.000" })).toBeVisible();
  });

  it("leaves the section out when nothing is waiting", async () => {
    serveCredits(credits({ requests: [] }));
    renderCreditsPage();
    await screen.findByRole("region", { name: POOL });
    expect(screen.queryByRole("list", { name: REQUESTS })).toBeNull();
  });

  it("approves a request: the allowance it raised is in the toast and the card leaves", async () => {
    const seen = serveCredits();
    const sent: string[] = [];
    server.use(
      http.post(`${BASE}/admin/ai/credit-requests/:id/approve`, ({ params }) => {
        sent.push(String(params["id"]));
        seen.view = credits({
          requests: [],
          teachers: seen.view.teachers.map((row) =>
            row.userId === duc.userId
              ? { ...row, allowance: 70000, status: "on_track" }
              : row,
          ),
        });
        return contractJson("/admin/ai/credit-requests/{id}/approve", "post", 200, {
          request: {
            id: ducRequest.id,
            userId: ducRequest.userId,
            month: ducRequest.month,
            amount: ducRequest.amount,
            reason: ducRequest.reason,
            status: "approved",
            createdAt: ducRequest.createdAt,
            decidedAt: new Date().toISOString(),
          },
          allowance: 70000,
        });
      }),
    );
    const { user } = renderCreditsPage();
    await user.click(await screen.findByRole("button", { name: "Duyệt 20.000" }));
    expect(
      await screen.findByText("Hạn mức tháng này của Phan Minh Đức giờ là 70.000"),
    ).toBeVisible();
    expect(sent).toEqual([ducRequest.id]);
    await waitFor(() =>
      expect(screen.queryByRole("list", { name: REQUESTS })).toBeNull(),
    );
    await waitFor(() => expect(seen.reads).toBeGreaterThan(1));
    expect(await screen.findByText("trên 70.000")).toBeVisible();
    expect(screen.getByRole("heading", { level: 1 })).toHaveFocus();
  });

  it("declines a request and tells the Admin the teacher will be told", async () => {
    const seen = serveCredits();
    const sent: string[] = [];
    server.use(
      http.post(`${BASE}/admin/ai/credit-requests/:id/decline`, ({ params }) => {
        sent.push(String(params["id"]));
        seen.view = credits({ requests: [] });
        return contractJson("/admin/ai/credit-requests/{id}/decline", "post", 200, {
          request: {
            id: ducRequest.id,
            userId: ducRequest.userId,
            month: ducRequest.month,
            amount: ducRequest.amount,
            reason: ducRequest.reason,
            status: "declined",
            createdAt: ducRequest.createdAt,
            decidedAt: new Date().toISOString(),
          },
          allowance: 0,
        });
      }),
    );
    const { user } = renderCreditsPage();
    await user.click(await screen.findByRole("button", { name: "Từ chối" }));
    expect(
      await screen.findByText("Phan Minh Đức sẽ được báo là yêu cầu bị từ chối"),
    ).toBeVisible();
    expect(sent).toEqual([ducRequest.id]);
    await waitFor(() =>
      expect(screen.queryByRole("list", { name: REQUESTS })).toBeNull(),
    );
  });

  it("disables both buttons while the request is in flight and sends it once", async () => {
    serveCredits();
    let release: () => void = () => {};
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    let posts = 0;
    server.use(
      http.post(`${BASE}/admin/ai/credit-requests/:id/approve`, async () => {
        posts += 1;
        await gate;
        return contractJson("/admin/ai/credit-requests/{id}/approve", "post", 200, {
          request: {
            id: ducRequest.id,
            userId: ducRequest.userId,
            month: ducRequest.month,
            amount: ducRequest.amount,
            reason: ducRequest.reason,
            status: "approved",
            createdAt: ducRequest.createdAt,
            decidedAt: new Date().toISOString(),
          },
          allowance: 70000,
        });
      }),
    );
    const { user } = renderCreditsPage();
    const approve = await screen.findByRole("button", { name: "Duyệt 20.000" });
    const decline = screen.getByRole("button", { name: "Từ chối" });
    await user.click(approve);
    await waitFor(() => expect(approve).toHaveAttribute("aria-disabled", "true"));
    expect(decline).toHaveAttribute("aria-disabled", "true");
    await user.click(approve);
    await user.click(decline);
    release();
    await screen.findByText("Hạn mức tháng này của Phan Minh Đức giờ là 70.000");
    expect(posts).toBe(1);
  });

  it("shows the server's sentence when the request was decided elsewhere, and the card leaves on the reread", async () => {
    const seen = serveCredits();
    server.use(
      http.post(`${BASE}/admin/ai/credit-requests/:id/approve`, () => {
        seen.view = credits({ requests: [] });
        return HttpResponse.json(
          {
            error: {
              code: "AI_REQUEST_DECIDED",
              message: "Một quản trị viên khác đã xử lý yêu cầu này.",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 409 },
        );
      }),
    );
    const { user } = renderCreditsPage();
    await user.click(await screen.findByRole("button", { name: "Duyệt 20.000" }));
    expect(
      await screen.findByText("Một quản trị viên khác đã xử lý yêu cầu này."),
    ).toBeVisible();
    await waitFor(() =>
      expect(screen.queryByRole("list", { name: REQUESTS })).toBeNull(),
    );
  });

  it("keeps the card and lets the Admin try again after a failure", async () => {
    serveCredits();
    server.use(
      http.post(`${BASE}/admin/ai/credit-requests/:id/decline`, () =>
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
    const decline = await screen.findByRole("button", { name: "Từ chối" });
    await user.click(decline);
    expect(
      await screen.findByText("Bạn không có quyền thao tác trên tài khoản này."),
    ).toBeVisible();
    await waitFor(() => expect(decline).not.toHaveAttribute("aria-disabled"));
    expect(screen.getByRole("list", { name: REQUESTS })).toBeVisible();
  });
});

describe("the teachers", () => {
  it("lists them by the share of their allowance used, with the figures, bars and standings", async () => {
    serveCredits();
    renderCreditsPage();
    const card = (await screen.findByRole("region", { name: "Giáo viên" })).closest(
      "section",
    )!;
    const rows = within(card).getAllByRole("listitem");
    expect(rows.map((row) => row.querySelector("p")!.textContent)).toEqual([
      "Trương Văn Khải",
      "Phan Minh Đức",
      "Đinh Quốc Bảo",
      "Giàu Bùi",
      "Ngô Thu Trang",
      "Hà Mỹ Linh",
    ]);
    const bao = rows[2]!;
    expect(within(bao).getByText("bao.dinh@quizzivy.com")).toBeVisible();
    expect(within(bao).getByText("30.100")).toBeVisible();
    expect(within(bao).getByText("trên 80.000 · tùy chỉnh")).toBeVisible();
    expect(
      within(bao).getByRole("progressbar", {
        name: "Đinh Quốc Bảo: đã dùng 30.100 trên 80.000 tín dụng",
      }),
    ).toHaveAttribute("aria-valuenow", "38");
    expect(within(rows[3]!).getByText("trên 50.000")).toBeVisible();
    expect(within(rows[0]!).getByText("Chạm hạn mức")).toBeVisible();
    expect(within(rows[1]!).getByText("Gần hạn mức")).toBeVisible();
    expect(within(rows[3]!).getByText("Ổn định")).toBeVisible();
    expect(within(rows[5]!).getByText("Tạm dừng")).toBeVisible();
  });

  it("counts the teachers who used AI and those who have not yet", async () => {
    serveCredits();
    renderCreditsPage();
    expect(
      await screen.findByText("6 giáo viên đã dùng AI tháng này · 3 chưa dùng"),
    ).toBeVisible();
  });

  it("colours each bar by the standing the server read", async () => {
    serveCredits();
    renderCreditsPage();
    const card = (await screen.findByRole("region", { name: "Giáo viên" })).closest(
      "section",
    )!;
    const fillOf = (name: string) =>
      within(card).getByRole("progressbar", { name: new RegExp(`^${name}:`) })
        .firstElementChild;
    expect(fillOf("Trương Văn Khải")).toHaveClass("bg-danger");
    expect(fillOf("Phan Minh Đức")).toHaveClass("bg-warning");
    expect(fillOf("Giàu Bùi")).toHaveClass("bg-brand");
    expect(fillOf("Hà Mỹ Linh")).toHaveClass("bg-danger");
  });

  it("says so when no teacher can use AI yet", async () => {
    serveCredits(credits({ teachers: [], active: 0, idle: 0, requests: [] }));
    renderCreditsPage();
    expect(
      await screen.findByText("Chưa có giáo viên nào được dùng AI."),
    ).toBeVisible();
    expect(
      screen.getByText("0 giáo viên đã dùng AI tháng này · 0 chưa dùng"),
    ).toBeVisible();
  });

  it("reads its figures in English", async () => {
    await i18n.changeLanguage("en");
    serveCredits();
    renderCreditsPage();
    expect(await screen.findByText("Centre pool · September")).toBeVisible();
    expect(screen.getByText("Centre plan 200,000 · resets 1 October")).toBeVisible();
    expect(screen.getByText("by 30 September, within the pool")).toBeVisible();
    expect(
      screen.getByText("6 teachers used AI this month · 3 haven’t yet"),
    ).toBeVisible();
    expect(screen.getByText("of 80,000 · custom")).toBeVisible();
    expect(screen.getByText("Near limit")).toBeVisible();
    expect(
      screen.getByText(
        "Phan Minh Đức asked for 20,000 more credits · 41,300 of 50,000 used",
      ),
    ).toBeVisible();
  });
});

describe("the states", () => {
  it("draws a skeleton for each card while the month is read", async () => {
    server.use(
      http.get(`${BASE}/admin/ai/credits`, async () => {
        await delay("infinite");
        return HttpResponse.json({});
      }),
    );
    renderCreditsPage();
    const loading = await screen.findByRole("status", { name: "Đang tải…" });
    expect(loading.querySelectorAll("[data-slot='skeleton']").length).toBeGreaterThan(
      6,
    );
    expect(loading.children).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "Thêm tín dụng" })).toBeNull();
  });

  it("says it could not load the credits, and Retry reads them again", async () => {
    let failing = true;
    const view = credits();
    server.use(
      http.get(`${BASE}/admin/ai/credits`, () => {
        if (!failing) return contractJson("/admin/ai/credits", "get", 200, view);
        return HttpResponse.json(
          {
            error: {
              code: "FORBIDDEN",
              message: "Không đủ quyền.",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 403 },
        );
      }),
    );
    const { user } = renderCreditsPage();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không tải được tín dụng AI.",
    );
    expect(screen.queryByRole("region", { name: POOL })).toBeNull();
    failing = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("region", { name: POOL })).toBeVisible();
  });

  it("answers a server without an assistant with one sentence and no card", async () => {
    const seen = serveOff();
    renderCreditsPage();
    expect(
      await screen.findByText("AI chưa được bật cho trung tâm này."),
    ).toBeVisible();
    expect(seen.reads).toBe(1);
    expect(screen.queryByRole("region", { name: POOL })).toBeNull();
    expect(screen.queryByRole("region", { name: "Giáo viên" })).toBeNull();
    expect(screen.queryByRole("region", { name: "Quy tắc" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Thêm tín dụng" })).toBeNull();
    expect(screen.queryByRole("list")).toBeNull();
  });

  it("says the same in English", async () => {
    await i18n.changeLanguage("en");
    serveOff();
    renderCreditsPage();
    expect(
      await screen.findByText("AI is not turned on for this school."),
    ).toBeVisible();
  });

  it("leaves what is drawn in place when a later read fails", async () => {
    const seen = serveCredits();
    const { client } = renderCreditsPage();
    await screen.findByRole("region", { name: POOL });
    server.use(
      http.get(`${BASE}/admin/ai/credits`, () =>
        HttpResponse.json(
          {
            error: {
              code: "FORBIDDEN",
              message: "Không đủ quyền.",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 403 },
        ),
      ),
    );
    await client.invalidateQueries({ queryKey: ["admin", "ai-credits"] });
    expect(seen.reads).toBe(1);
    expect(screen.getByRole("region", { name: POOL })).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("the layout", () => {
  it("docks the rules beside the teachers from a content width of 900 and not below", async () => {
    contentWidth(900);
    serveCredits();
    renderCreditsPage();
    const rules = (await screen.findByRole("region", { name: "Quy tắc" })).closest(
      "[data-sticky-aside]",
    );
    expect(rules).toHaveClass("sticky-column");
  });

  it("lets the rules scroll with the page below 900", async () => {
    contentWidth(899);
    serveCredits();
    renderCreditsPage();
    const rules = (await screen.findByRole("region", { name: "Quy tắc" })).closest(
      "[data-sticky-aside]",
    );
    expect(rules).not.toHaveClass("sticky-column");
  });
});
