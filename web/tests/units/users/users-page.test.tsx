import { afterEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import i18n from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { viewport } from "@tests/support/viewport";
import {
  ADMIN_ROLE,
  BASE,
  HEAD_ROLE,
  TEACHER_ROLE,
  headersOf,
  hoursAgo,
  lastQuery,
  renderUsers,
  rowOf,
  serveUsers,
  table,
  user,
} from "./support";

const THUONG = user("1", "Thường Võ", {
  role: ADMIN_ROLE,
  signIn: { password: true, google: true },
  lastActiveAt: hoursAgo(0.001),
});
const GIAU = user("2", "Giàu Bùi", {
  role: TEACHER_ROLE,
  classes: { count: 4, firstName: "IELTS 6.5 Tối" },
  lastActiveAt: hoursAgo(5),
});
const DUC = user("3", "Phan Minh Đức", {
  role: HEAD_ROLE,
  signIn: { password: false, google: true },
});
const AN = user("4", "Nguyễn Văn An", {
  classes: { count: 1, firstName: "IELTS Nền tảng A" },
  lastActiveAt: hoursAgo(48),
});
const BINH = user("5", "Trần Thị Bình", {
  status: "must_change_password",
  lastActiveAt: hoursAgo(72),
});
const CUONG = user("6", "Lê Quốc Cường", {
  status: "never_signed_in",
  signIn: { password: false, google: false },
  lastActiveAt: null,
});
const DUNG = user("7", "Vũ Thuỳ Dung", {
  role: TEACHER_ROLE,
  status: "disabled",
  lastActiveAt: hoursAgo(24 * 14),
});
const ROWS = [THUONG, GIAU, DUC, AN, BINH, CUONG, DUNG];

afterEach(async () => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  await act(() => i18n.changeLanguage("vi"));
});

describe("the Users page", () => {
  it("says what it is and lists every account, newest first as the server sends them", async () => {
    serveUsers(ROWS);
    renderUsers();

    expect(
      await screen.findByRole("heading", { level: 1, name: "Người dùng" }),
    ).toBeVisible();
    expect(screen.getByText("Mọi người có thể đăng nhập vào Quizzivy.")).toBeVisible();
    const rows = within(await table())
      .getAllByRole("row")
      .slice(1);
    expect(
      rows.map((row) => ROWS.findIndex((one) => within(row).queryByText(one.fullName))),
    ).toEqual(ROWS.map((_, index) => index));
  });

  it("offers Import CSV and no action the page cannot take yet", async () => {
    serveUsers(ROWS);
    renderUsers();
    await table();
    expect(screen.getByRole("button", { name: "Nhập CSV" })).toBeVisible();
    expect(screen.queryByRole("button", { name: /Thêm người dùng/ })).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
  });

  it("shows a skeleton until the first page arrives", async () => {
    let release: () => void = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${BASE}/admin/users`, async () => {
        await held;
        return HttpResponse.json({ code: "INTERNAL" }, { status: 500 });
      }),
    );
    renderUsers();
    expect(await screen.findByRole("status", { name: "Đang tải…" })).toBeVisible();
    release();
    expect(await screen.findByRole("alert")).toBeVisible();
  });

  it("says so when the list cannot load, and retries", async () => {
    let healthy = false;
    const asked = serveUsers(ROWS);
    server.use(
      http.get(`${BASE}/admin/users`, () => {
        if (healthy) return undefined;
        return HttpResponse.json({ code: "INTERNAL" }, { status: 500 });
      }),
    );
    const { user: person } = renderUsers();
    const alert = await screen.findByRole("alert");
    expect(
      within(alert).getByText("Không tải được danh sách người dùng."),
    ).toBeVisible();
    healthy = true;
    await person.click(within(alert).getByRole("button", { name: "Thử lại" }));
    expect(await rowOf("Thường Võ")).toBeVisible();
    expect(asked.length).toBeGreaterThan(0);
  });
});

describe("the tabs", () => {
  it("count what each would list, from the facets of the answer", async () => {
    serveUsers(ROWS, {
      facets: { all: 312, teachers: 9, students: 300, admins: 3, disabled: 6 },
      total: 312,
    });
    renderUsers();
    await table();
    for (const [name, pressed] of [
      ["Tất cả 312", true],
      ["Giáo viên 9", false],
      ["Học viên 300", false],
      ["Quản trị viên 3", false],
      ["Đã vô hiệu hoá 6", false],
    ] as const) {
      expect(screen.getByRole("button", { name })).toHaveAttribute(
        "aria-pressed",
        String(pressed),
      );
    }
  });

  it("ask for the tab and write it to the URL, All writing nothing", async () => {
    const asked = serveUsers(ROWS);
    const { user: person, router } = renderUsers();
    await table();
    expect((await lastQuery(asked)).get("tab")).toBeNull();

    await person.click(screen.getByRole("button", { name: /^Học viên/ }));
    await waitFor(() => expect(asked.at(-1)?.get("tab")).toBe("students"));
    expect(router.state.location.search).toBe("?tab=students");
    expect(await rowOf("Nguyễn Văn An")).toBeVisible();
    expect(within(await table()).queryByRole("row", { name: /Thường Võ/ })).toBeNull();

    await person.click(screen.getByRole("button", { name: /^Tất cả/ }));
    await waitFor(() => expect(asked.at(-1)?.get("tab")).toBeNull());
    expect(router.state.location.search).toBe("");
  });

  it("read the tab from the URL, and read an unknown one as All", async () => {
    const asked = serveUsers(ROWS);
    renderUsers("/admin/users?tab=admins");
    await table();
    expect((await lastQuery(asked)).get("tab")).toBe("admins");
    expect(screen.getByRole("button", { name: /^Quản trị viên/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("fall back to All for a tab no build knows", async () => {
    const asked = serveUsers(ROWS);
    renderUsers("/admin/users?tab=owners");
    await table();
    expect((await lastQuery(asked)).get("tab")).toBeNull();
    expect(screen.getByRole("button", { name: /^Tất cả/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("send the page back to the first when another is chosen", async () => {
    const asked = serveUsers(ROWS, { total: 95 });
    const { user: person, router } = renderUsers("/admin/users?page=3");
    await table();
    expect((await lastQuery(asked)).get("page")).toBe("3");
    await person.click(screen.getByRole("button", { name: /^Giáo viên/ }));
    await waitFor(() => expect(router.state.location.search).toBe("?tab=teachers"));
    await waitFor(() => expect(asked.at(-1)?.get("page")).toBeNull());
  });
});

describe("the search", () => {
  it("sends nothing under three characters and says why", async () => {
    const asked = serveUsers(ROWS);
    const { user: person } = renderUsers();
    await table();
    const before = asked.length;

    await person.type(screen.getByRole("searchbox"), "ng");
    expect(await screen.findByText("Nhập ít nhất 3 ký tự để tìm kiếm.")).toBeVisible();
    await new Promise((resolve) => setTimeout(resolve, 450));
    expect(asked).toHaveLength(before);
    expect(asked.every((query) => !query.has("q"))).toBe(true);
  });

  it("sends the text from the third character, trimmed, and clears the hint", async () => {
    const asked = serveUsers(ROWS);
    const { user: person, router } = renderUsers();
    await table();

    await person.type(screen.getByRole("searchbox"), "  ngu ");
    await waitFor(() => expect(asked.at(-1)?.get("q")).toBe("ngu"), { timeout: 4000 });
    expect(router.state.location.search).toBe("?q=ngu");
    expect(screen.queryByText("Nhập ít nhất 3 ký tự để tìm kiếm.")).toBeNull();
    expect(await rowOf("Nguyễn Văn An")).toBeVisible();
  });

  it("counts a character, not a code unit", async () => {
    const asked = serveUsers(ROWS);
    const { user: person } = renderUsers();
    await table();
    await person.type(screen.getByRole("searchbox"), "Đức");
    await waitFor(() => expect(asked.at(-1)?.get("q")).toBe("Đức"), { timeout: 4000 });
  });

  it("returns to the whole list when the text is cut below three characters", async () => {
    const asked = serveUsers(ROWS);
    const { user: person } = renderUsers("/admin/users?q=nguyen");
    await table();
    expect((await lastQuery(asked)).get("q")).toBe("nguyen");

    await person.clear(screen.getByRole("searchbox"));
    await person.type(screen.getByRole("searchbox"), "ng");
    await waitFor(() => expect(asked.at(-1)?.has("q")).toBe(false), { timeout: 4000 });
    expect(screen.getByText("Nhập ít nhất 3 ký tự để tìm kiếm.")).toBeVisible();
  });

  it("keeps the counts of the tabs under the search the server counted them for", async () => {
    serveUsers(ROWS);
    const { user: person } = renderUsers();
    await table();
    await person.type(screen.getByRole("searchbox"), "ngu");
    await waitFor(
      () => expect(screen.getByRole("button", { name: "Tất cả 1" })).toBeVisible(),
      { timeout: 4000 },
    );
    expect(screen.getByRole("button", { name: "Học viên 1" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Giáo viên 0" })).toBeVisible();
  });
});

describe("the state chip", () => {
  it("shows the state the Overview linked to as one chip, and asks for it", async () => {
    const asked = serveUsers(ROWS);
    renderUsers("/admin/users?status=must_change_password");
    expect(await rowOf("Trần Thị Bình")).toBeVisible();
    expect((await lastQuery(asked)).get("status")).toBe("must_change_password");
    const chip = screen.getByRole("group", { name: "Bộ lọc đang áp dụng" });
    expect(within(chip).getByText("Phải đổi mật khẩu")).toBeVisible();
    expect(within(await table()).queryByRole("row", { name: /Thường Võ/ })).toBeNull();
  });

  it("is removed by its own button, leaving the other parameters", async () => {
    const asked = serveUsers(ROWS);
    const { user: person, router } = renderUsers(
      "/admin/users?tab=students&status=never_signed_in&size=20",
    );
    await table();
    await person.click(screen.getByRole("button", { name: "Bỏ lọc Chưa đăng nhập" }));
    await waitFor(() => expect(asked.at(-1)?.has("status")).toBe(false));
    expect(router.state.location.search).toBe("?tab=students&size=20");
    expect(screen.queryByRole("group", { name: "Bộ lọc đang áp dụng" })).toBeNull();
  });

  it("ignores a state no build knows", async () => {
    const asked = serveUsers(ROWS);
    renderUsers("/admin/users?status=banned");
    await table();
    expect((await lastQuery(asked)).has("status")).toBe(false);
    expect(screen.queryByRole("group", { name: "Bộ lọc đang áp dụng" })).toBeNull();
  });
});

describe("an empty answer", () => {
  it("says no one matches and clears the tab, the search, the state and the page", async () => {
    serveUsers([]);
    const { user: person, router } = renderUsers(
      "/admin/users?tab=admins&q=zzz&status=never_signed_in&page=2&size=20",
    );
    expect(await screen.findByText("Không có người dùng phù hợp.")).toBeVisible();
    await person.click(screen.getByRole("button", { name: "Bỏ lọc" }));
    await waitFor(() => expect(router.state.location.search).toBe("?size=20"));
  });

  it("offers nothing to clear when nothing was asked for", async () => {
    serveUsers([]);
    renderUsers();
    expect(await screen.findByText("Không có người dùng phù hợp.")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Bỏ lọc" })).toBeNull();
  });
});

describe("the pager", () => {
  it("reads the range of the page against the total and asks for ten by default", async () => {
    const asked = serveUsers(ROWS, { total: 312 });
    renderUsers();
    await table();
    expect(await screen.findByText("1–10 trên 312 người dùng")).toBeVisible();
    expect((await lastQuery(asked)).get("limit")).toBe("10");
  });

  it("asks for the page and the size the URL names", async () => {
    const asked = serveUsers(ROWS, { total: 312 });
    renderUsers("/admin/users?page=3&size=50");
    await table();
    const query = await lastQuery(asked);
    expect(query.get("page")).toBe("3");
    expect(query.get("limit")).toBe("50");
    expect(await screen.findByText("101–150 trên 312 người dùng")).toBeVisible();
  });

  it("offers ten, twenty and fifty rows to a page, and a size it does not offer is ten", async () => {
    const asked = serveUsers(ROWS);
    const { user: person, router } = renderUsers("/admin/users?size=30");
    await table();
    expect((await lastQuery(asked)).get("limit")).toBe("10");
    await person.click(screen.getByRole("combobox", { name: "Số dòng mỗi trang" }));
    const list = await screen.findByRole("listbox");
    expect(
      within(list)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["10", "20", "50"]);
    await person.click(within(list).getByRole("option", { name: "20" }));
    await waitFor(() => expect(router.state.location.search).toBe("?size=20"));
    await waitFor(() => expect(asked.at(-1)?.get("limit")).toBe("20"));
  });

  it("hides the page size on a phone", async () => {
    viewport("phone");
    serveUsers(ROWS);
    renderUsers("/admin/users", 328);
    expect(await screen.findByRole("list", { name: "Người dùng" })).toBeVisible();
    expect(screen.queryByRole("combobox", { name: "Số dòng mỗi trang" })).toBeNull();
  });
});

describe("the columns", () => {
  it.each([
    [560, ["Người dùng", "Trạng thái"]],
    [639, ["Người dùng", "Trạng thái"]],
    [640, ["Người dùng", "Vai trò", "Trạng thái"]],
    [819, ["Người dùng", "Vai trò", "Trạng thái"]],
    [820, ["Người dùng", "Vai trò", "Hoạt động cuối", "Trạng thái"]],
    [959, ["Người dùng", "Vai trò", "Hoạt động cuối", "Trạng thái"]],
    [960, ["Người dùng", "Vai trò", "Lớp", "Hoạt động cuối", "Trạng thái"]],
    [1079, ["Người dùng", "Vai trò", "Lớp", "Hoạt động cuối", "Trạng thái"]],
    [
      1080,
      ["Người dùng", "Vai trò", "Đăng nhập", "Lớp", "Hoạt động cuối", "Trạng thái"],
    ],
  ])("join by the content's width: %i px draws %j", async (width, headers) => {
    serveUsers(ROWS);
    renderUsers("/admin/users", width);
    await table();
    expect((await headersOf()).filter((name) => name !== "")).toEqual(headers);
  });

  it("folds the role and the last activity under the name while their columns are gone", async () => {
    serveUsers(ROWS);
    const { widths } = renderUsers("/admin/users", 560);
    const giau = await rowOf("Giàu Bùi");
    expect(
      within(giau).getByText("2@example.com · Giáo viên · 5 giờ trước"),
    ).toBeVisible();

    widths.resize(700);
    await waitFor(() =>
      expect(within(giau).getByText("2@example.com · 5 giờ trước")).toBeVisible(),
    );
    widths.resize(900);
    await waitFor(() => expect(within(giau).getByText("2@example.com")).toBeVisible());
  });

  it('folds "never" under the name of an account that has not been active', async () => {
    serveUsers(ROWS);
    renderUsers("/admin/users", 560);
    const cuong = await rowOf("Lê Quốc Cường");
    expect(
      within(cuong).getByText("6@example.com · Học viên · Chưa bao giờ"),
    ).toBeVisible();
  });

  it("shortens a temporary password's pill while the last activity is folded, keeping the full label as its title", async () => {
    serveUsers(ROWS);
    const { widths } = renderUsers("/admin/users", 700);
    const binh = await rowOf("Trần Thị Bình");
    const pill = within(binh).getByText("Đổi mật khẩu");
    expect(pill.closest("[title]")).toHaveAttribute("title", "Phải đổi mật khẩu");
    widths.resize(820);
    await waitFor(() =>
      expect(within(binh).getByText("Phải đổi mật khẩu")).toBeVisible(),
    );
  });
});

describe("the cells", () => {
  it("name the role by its own name and icon, a built-in one in the language of the page", async () => {
    serveUsers(ROWS);
    renderUsers("/admin/users", 1280);
    const duc = await rowOf("Phan Minh Đức");
    expect(within(duc).getByText("Tổ trưởng bộ môn")).toBeVisible();
    const giau = await rowOf("Giàu Bùi");
    expect(within(giau).getByText("Giáo viên")).toBeVisible();

    await act(() => i18n.changeLanguage("en"));
    expect(
      await screen.findByRole("heading", { level: 1, name: "Users" }),
    ).toBeVisible();
    const english = await screen.findByRole("table", { name: "Users" });
    expect(
      within(within(english).getByRole("row", { name: /Giàu Bùi/ })).getByText(
        "Teacher",
      ),
    ).toBeVisible();
    expect(
      within(within(english).getByRole("row", { name: /Phan Minh Đức/ })).getByText(
        "Tổ trưởng bộ môn",
      ),
    ).toBeVisible();
  });

  it("draw each way in: the key and Password, the mark and Google, or none yet", async () => {
    serveUsers(ROWS);
    renderUsers("/admin/users", 1280);
    const thuong = await rowOf("Thường Võ");
    expect(within(thuong).getByText("Mật khẩu")).toBeVisible();
    expect(within(thuong).getByText("Google")).toBeVisible();
    const duc = await rowOf("Phan Minh Đức");
    expect(within(duc).queryByText("Mật khẩu")).toBeNull();
    expect(within(duc).getByText("Google")).toBeVisible();
    expect(within(await rowOf("Lê Quốc Cường")).getByText("Chưa có")).toBeVisible();
  });

  it("draw the first class and how many more, a student in none and anyone else in none", async () => {
    serveUsers(ROWS);
    renderUsers("/admin/users", 1280);
    const giau = await rowOf("Giàu Bùi");
    expect(within(giau).getByText("IELTS 6.5 Tối")).toBeVisible();
    expect(within(giau).getByText("+3")).toBeInTheDocument();
    expect(within(giau).getByText("và 3 lớp khác")).toBeInTheDocument();
    expect(within(await rowOf("Nguyễn Văn An")).queryByText(/^\+/)).toBeNull();
    expect(
      within(await rowOf("Trần Thị Bình")).getByText("Chưa vào lớp"),
    ).toBeVisible();
    expect(within(await rowOf("Thường Võ")).getByText("—")).toBeVisible();
  });

  it("write the last activity as the bare short value, with the exact moment as its title", async () => {
    serveUsers(ROWS);
    renderUsers("/admin/users", 1280);
    const giau = await rowOf("Giàu Bùi");
    const time = within(giau).getByText("5 giờ trước");
    expect(time.tagName).toBe("TIME");
    expect(time).toHaveAttribute("dateTime", GIAU.lastActiveAt!);
    expect(time.getAttribute("title")).toMatch(/^\d\d:\d\d, \d\d\/\d\d\/\d{4}$/);
    expect(within(await rowOf("Thường Võ")).getByText("vừa xong")).toBeVisible();
    expect(
      within(await rowOf("Lê Quốc Cường")).getByText("Chưa bao giờ"),
    ).toBeVisible();
    expect(within(await rowOf("Vũ Thuỳ Dung")).getByText("2 tuần trước")).toBeVisible();
  });

  it("draw the four states: active, must change, never signed in, disabled", async () => {
    serveUsers(ROWS);
    renderUsers("/admin/users", 1280);
    expect(within(await rowOf("Giàu Bùi")).getByText("Đang hoạt động")).toBeVisible();
    expect(
      within(await rowOf("Trần Thị Bình")).getByText("Phải đổi mật khẩu"),
    ).toBeVisible();
    expect(
      within(await rowOf("Lê Quốc Cường")).getByText("Chưa đăng nhập"),
    ).toBeVisible();
    expect(
      within(await rowOf("Vũ Thuỳ Dung")).getByText("Đã vô hiệu hoá"),
    ).toBeVisible();
  });

  it("truncate a long role name and keep it whole in the title", async () => {
    const long = "Trưởng bộ phận khảo thí và kiểm định chất lượng đào tạo";
    serveUsers([user("9", "Hà Thu", { role: { ...HEAD_ROLE, name: long } })]);
    renderUsers("/admin/users", 1280);
    const row = await rowOf("Hà Thu");
    expect(within(row).getByText(long).closest("[title]")).toHaveAttribute(
      "title",
      long,
    );
  });
});

describe("cards", () => {
  it("replace the table below 560px of content, each with the role, the state and the last activity", async () => {
    serveUsers(ROWS);
    renderUsers("/admin/users", 559);
    const list = await screen.findByRole("list", { name: "Người dùng" });
    expect(screen.queryByRole("table")).toBeNull();
    const cards = within(list).getAllByRole("listitem");
    expect(cards).toHaveLength(ROWS.length);
    const binh = cards.find((card) => within(card).queryByText("Trần Thị Bình"));
    expect(binh).toBeDefined();
    expect(within(binh!).getByText("5@example.com")).toBeVisible();
    expect(within(binh!).getByText("Học viên")).toBeVisible();
    expect(within(binh!).getByText("Phải đổi mật khẩu")).toBeVisible();
    expect(within(binh!).getByText("3 ngày trước")).toBeVisible();
  });

  it("do not depend on the window: a wide window with narrow content is cards", async () => {
    serveUsers(ROWS);
    renderUsers("/admin/users", 464);
    expect(await screen.findByRole("list", { name: "Người dùng" })).toBeVisible();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("turn back into the table when the content widens past 560px", async () => {
    serveUsers(ROWS);
    const { widths } = renderUsers("/admin/users", 400);
    await screen.findByRole("list", { name: "Người dùng" });
    widths.resize(760);
    expect(await table()).toBeVisible();
    expect(screen.queryByRole("list", { name: "Người dùng" })).toBeNull();
  });
});
