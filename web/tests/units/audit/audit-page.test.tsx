import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import i18n from "@/lib/i18n";
import { BASE_URL } from "@/lib/api/client";
import { contentWidth } from "@tests/support/contentWidth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import {
  ACTOR,
  FACETS,
  REQUEST_ID,
  entry,
  lastAsked,
  renderAudit,
  serves,
} from "./support";

const CREATED = entry({ id: 3 });
const SYSTEM = entry({
  id: 2,
  actor: null,
  ip: null,
  device: null,
  action: "import.files_removed",
  category: "imports",
  entity: { type: "word_import", id: null, label: null },
  changes: [],
  details: { files: "2 tệp · 3,4 MB" },
});
const UNKNOWN = entry({
  id: 1,
  action: "widget.frobnicated_all",
  category: null,
  entity: { type: "organization", id: null, label: null },
  changes: [],
});
const ROWS = [CREATED, SYSTEM, UNKNOWN];

beforeEach(() => contentWidth(1320));
afterEach(async () => {
  await act(() => i18n.changeLanguage("vi"));
});

const header = (name: string) => screen.queryByRole("columnheader", { name });

describe("the audit log's rows", () => {
  it("heads the page and draws a row as the deck does", async () => {
    serves(() => ({ items: ROWS, total: 1248 }));
    renderAudit();

    expect(
      screen.getByRole("heading", { level: 1, name: "Nhật ký kiểm tra" }),
    ).toBeVisible();
    expect(
      screen.getByText("Bản ghi các thay đổi quan trọng. Không thể sửa hay xoá."),
    ).toBeVisible();
    const row = await screen.findByRole("button", { name: /Đã tạo lớp/ });
    const cells = within(row.closest("[role=row]") as HTMLElement);
    expect(cells.getByText("24 Th9, 20:31")).toBeVisible();
    expect(cells.getByText("Giàu Bùi")).toBeVisible();
    expect(cells.getByText("Đã tạo lớp")).toBeVisible();
    expect(cells.getByText("Lớp · IELTS 6.5 Evening")).toBeVisible();
    expect(cells.getByText("203.0.113.8")).toBeVisible();
    expect(screen.getByText("1–10 trên 1.248 mục")).toBeVisible();
  });

  it("reads in English with the deck's month names", async () => {
    await act(() => i18n.changeLanguage("en"));
    serves(() => ({ items: [CREATED] }));
    renderAudit();

    const row = await screen.findByRole("button", { name: /Created a class/ });
    expect(
      within(row.closest("[role=row]") as HTMLElement).getByText("24 Sep, 20:31"),
    ).toBeVisible();
    expect(screen.getByRole("columnheader", { name: "IP address" })).toBeVisible();
  });

  it("names the system for an entry with no actor, and says what an entity is when its row is gone", async () => {
    serves(() => ({
      items: [
        SYSTEM,
        entry({ id: 5, entity: { type: "class", id: CREATED.entity.id, label: null } }),
      ],
    }));
    renderAudit();

    const row = await screen.findByRole("button", {
      name: /Đã xoá tệp của lần nhập Word/,
    });
    const cells = within(row.closest("[role=row]") as HTMLElement);
    expect(cells.getByText("Hệ thống")).toBeVisible();
    expect(cells.getByText("Lần nhập Word")).toBeVisible();
    expect(cells.getByText("—")).toBeVisible();
    expect(screen.getByText("Lớp đã xoá")).toBeVisible();
  });

  it.each([
    ["role", "Vai trò"],
    ["question", "Câu hỏi"],
    ["data_export", "Xuất dữ liệu"],
  ])(
    "does not call a %s deleted when the server sent no label for it",
    async (type, word) => {
      serves(() => ({
        items: [
          entry({
            id: 6,
            action: "role.created",
            entity: { type, id: CREATED.entity.id, label: null },
          }),
        ],
      }));
      renderAudit();

      const row = await screen.findByRole("button", { name: /Đã tạo vai trò/ });
      const cells = within(row.closest("[role=row]") as HTMLElement);
      expect(cells.getByText(word)).toBeVisible();
      expect(screen.queryByText(`${word} đã xoá`)).toBeNull();
    },
  );

  it("words an action the client has no sentence for from its key", async () => {
    serves(() => ({ items: [UNKNOWN] }));
    renderAudit();

    expect(await screen.findByText("widget frobnicated all")).toBeVisible();
    expect(screen.getByText("Trung tâm")).toBeVisible();
  });

  it("opens a row to its diff and the device, and closes it again", async () => {
    serves(() => ({ items: ROWS }));
    renderAudit();
    const user = userEvent.setup();

    const row = await screen.findByRole("button", { name: /Đã tạo lớp/ });
    expect(row).toHaveAttribute("aria-expanded", "false");
    await user.click(row);

    expect(row).toHaveAttribute("aria-expanded", "true");
    const panel = document.getElementById(row.getAttribute("aria-controls")!)!;
    expect(within(panel).getByText("code")).toBeVisible();
    expect(within(panel).getByText("H2LV-5QNC").tagName).toBe("DEL");
    expect(within(panel).getByText("K7QM-2PXA").tagName).toBe("INS");
    expect(within(panel).getByText("Mac · Chrome")).toBeVisible();

    await user.click(row);
    expect(row).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("H2LV-5QNC")).toBeNull();
  });

  it("opens from a click anywhere on the row", async () => {
    serves(() => ({ items: ROWS }));
    renderAudit();
    const user = userEvent.setup();

    const row = await screen.findByRole("button", { name: /Đã tạo lớp/ });
    await user.click(
      within(row.closest("[role=row]") as HTMLElement).getByText("203.0.113.8"),
    );

    expect(row).toHaveAttribute("aria-expanded", "true");
  });

  it("shows what an entry with no field changes recorded instead", async () => {
    serves(() => ({ items: ROWS }));
    renderAudit();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: /Đã xoá tệp của lần nhập Word/ }),
    );
    expect(screen.getByText("files")).toBeVisible();
    expect(screen.getByText("2 tệp · 3,4 MB")).toBeVisible();

    await user.click(screen.getByRole("button", { name: /widget frobnicated all/ }));
    expect(
      screen.getByText("Mục này không ghi lại thay đổi nào ở từng trường."),
    ).toBeVisible();
  });

  it("draws a value that is not text as JSON and one that is missing as a dash", async () => {
    serves(() => ({
      items: [
        entry({
          id: 9,
          changes: [
            { field: "members", from: null, to: ["a", "b"] },
            { field: "limit", from: 2, to: true },
          ],
        }),
      ],
    }));
    renderAudit();
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: /Đã tạo lớp/ }));

    expect(screen.getByText('["a","b"]')).toBeVisible();
    expect(screen.getByText("2")).toBeVisible();
    expect(screen.getByText("true")).toBeVisible();
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });
});

describe("the columns fold with the content's width", () => {
  it.each([
    [1320, ["Thời gian", "Người thực hiện", "Thao tác", "Đối tượng", "Địa chỉ IP"]],
    [1000, ["Thời gian", "Người thực hiện", "Thao tác", "Đối tượng"]],
    [800, ["Thời gian", "Người thực hiện", "Thao tác"]],
    [600, ["Người thực hiện", "Thao tác"]],
  ])("at %ipx draws %j", async (width, headers) => {
    contentWidth(width);
    serves(() => ({ items: [CREATED] }));
    renderAudit();

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    for (const name of [
      "Thời gian",
      "Người thực hiện",
      "Thao tác",
      "Đối tượng",
      "Địa chỉ IP",
    ]) {
      expect(header(name) !== null).toBe(headers.includes(name));
    }
  });

  it("puts the time under the name below 700px and the entity under the sentence below 860px", async () => {
    contentWidth(600);
    serves(() => ({ items: [CREATED] }));
    renderAudit();

    const row = (await screen.findByRole("button", { name: /Đã tạo lớp/ })).closest(
      "[role=row]",
    ) as HTMLElement;
    expect(within(row).getByText("24 Th9, 20:31")).toBeVisible();
    expect(within(row).getByText("Lớp · IELTS 6.5 Evening")).toBeVisible();
    expect(within(row).queryByText("203.0.113.8")).toBeNull();
  });

  it("names the open button by the entry, so it is not only a time", async () => {
    contentWidth(1320);
    serves(() => ({ items: [CREATED] }));
    renderAudit();

    expect(
      await screen.findByRole("button", {
        name: "Đã tạo lớp, Giàu Bùi, 24 Th9, 20:31",
      }),
    ).toBeVisible();
  });
});

describe("the diff's columns", () => {
  it.each([
    [580, true],
    [560, true],
    [559, false],
    [520, false],
  ])("at %ipx of content draws the field column: %s", async (width, drawn) => {
    contentWidth(width);
    serves(() => ({ items: [CREATED] }));
    renderAudit();
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: /Đã tạo lớp/ }));

    const item = screen.getByText("code").closest("li") as HTMLElement;
    expect(item.className.includes("grid-cols-[8.75rem_")).toBe(drawn);
  });
});

describe("the categories", () => {
  it("shows each option's count from the facets, grouped for the locale, a zero too", async () => {
    serves(() => ({ items: ROWS, facets: FACETS }));
    renderAudit();

    for (const [name, label] of [
      ["Tất cả", "1.248"],
      ["Tài khoản", "312"],
      ["Lớp học", "140"],
      ["Lượt làm bài", "410"],
      ["Bài kiểm tra", "301"],
      ["Phương tiện", "0"],
      ["Nhập Word", "7"],
    ] as const) {
      expect(
        await screen.findByRole("button", { name: `${name} ${label}` }),
      ).toBeVisible();
    }
  });

  it("asks for the category chosen, resets the page, and keeps the counts as they are", async () => {
    const asked = serves((url) => ({
      items: ROWS,
      total: url.searchParams.get("category") === null ? 1248 : 410,
    }));
    const { router } = renderAudit("?page=3");
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Lượt làm bài 410" }));

    await waitFor(() =>
      expect(lastAsked(asked).searchParams.get("category")).toBe("attempts"),
    );
    expect(lastAsked(asked).searchParams.get("page")).toBe("1");
    expect(router.state.location.search).toBe("?category=attempts");
    expect(screen.getByRole("button", { name: "Lượt làm bài 410" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "Tất cả 1.248" })).toBeVisible();
    expect(await screen.findByText("1–10 trên 410 mục")).toBeVisible();
  });

  it("goes back to every category on All", async () => {
    const asked = serves(() => ({ items: ROWS }));
    renderAudit("?category=tests");
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: /^Tất cả/ }));

    await waitFor(() =>
      expect(lastAsked(asked).searchParams.has("category")).toBe(false),
    );
  });

  it("follows the search and the range in the counts it shows", async () => {
    const asked = serves((url) => ({
      items: ROWS,
      facets:
        url.searchParams.get("q") === "nam"
          ? {
              all: 5,
              accounts: 1,
              classes: 1,
              attempts: 2,
              tests: 1,
              media: 0,
              imports: 0,
            }
          : FACETS,
    }));
    renderAudit();
    const user = userEvent.setup();

    expect(await screen.findByRole("button", { name: "Tất cả 1.248" })).toBeVisible();
    await user.type(
      screen.getByRole("textbox", { name: "Tìm trong nhật ký kiểm tra" }),
      "nam",
    );

    expect(
      await screen.findByRole("button", { name: "Tất cả 5" }, { timeout: 4000 }),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Lượt làm bài 2" })).toBeVisible();
    expect(lastAsked(asked).searchParams.get("q")).toBe("nam");
  });
});

describe("the search", () => {
  it("asks once the typing has paused, keeps the words in the address and clears from its button", async () => {
    const asked = serves(() => ({ items: ROWS }));
    const { router } = renderAudit();
    const user = userEvent.setup();

    const box = screen.getByRole("textbox", { name: "Tìm trong nhật ký kiểm tra" });
    expect(screen.queryByRole("button", { name: "Xoá tìm kiếm" })).toBeNull();
    await user.type(box, "Hoàng");
    expect(router.state.location.search).toBe("?q=Ho%C3%A0ng");
    await waitFor(() => expect(lastAsked(asked).searchParams.get("q")).toBe("Hoàng"));
    expect(asked.every((url) => url.searchParams.get("q") !== "H")).toBe(true);

    await user.click(screen.getByRole("button", { name: "Xoá tìm kiếm" }));

    expect(box).toHaveValue("");
    expect(router.state.location.search).toBe("");
    await waitFor(() => expect(lastAsked(asked).searchParams.has("q")).toBe(false));
  });

  it("stops at the 200 characters the contract takes, in the field and in an address that has more", async () => {
    const asked = serves(() => ({ items: ROWS }));
    renderAudit(`?q=${"x".repeat(250)}`);

    const box = screen.getByRole("textbox", { name: "Tìm trong nhật ký kiểm tra" });
    expect(box).toHaveAttribute("maxlength", "200");
    expect(box).toHaveValue("x".repeat(200));
    await waitFor(() =>
      expect(lastAsked(asked).searchParams.get("q")).toBe("x".repeat(200)),
    );
  });

  it("reads the words from the address", async () => {
    const asked = serves(() => ({ items: ROWS }));
    renderAudit("?q=lớp");

    expect(
      screen.getByRole("textbox", { name: "Tìm trong nhật ký kiểm tra" }),
    ).toHaveValue("lớp");
    await waitFor(() => expect(lastAsked(asked).searchParams.get("q")).toBe("lớp"));
  });
});

describe("the date range", () => {
  const menu = () => screen.getByRole("button", { name: /^Khoảng thời gian:/ });

  it("opens on the last 7 days, which the request names and the address does not", async () => {
    const asked = serves(() => ({ items: ROWS }));
    const { router } = renderAudit();

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    expect(menu()).toHaveTextContent("7 ngày qua");
    expect(lastAsked(asked).searchParams.get("range")).toBe("7d");
    expect(router.state.location.search).toBe("");
  });

  it.each([
    ["Hôm nay", "today", "?range=today"],
    ["30 ngày qua", "30d", "?range=30d"],
    ["Mọi lúc", "all", "?range=all"],
  ])("asks for %s", async (name, range, search) => {
    const asked = serves(() => ({ items: ROWS }));
    const { router } = renderAudit("?page=2");
    const user = userEvent.setup();

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    await user.click(menu());
    await user.click(await screen.findByRole("menuitemradio", { name }));

    await waitFor(() => expect(lastAsked(asked).searchParams.get("range")).toBe(range));
    expect(lastAsked(asked).searchParams.get("page")).toBe("1");
    expect(router.state.location.search).toBe(search);
    expect(menu()).toHaveTextContent(name);
  });

  it("ticks the range in force and goes back to the default from the menu", async () => {
    const asked = serves(() => ({ items: ROWS }));
    const { router } = renderAudit("?range=30d");
    const user = userEvent.setup();

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    await user.click(menu());
    expect(
      await screen.findByRole("menuitemradio", { name: "30 ngày qua" }),
    ).toBeChecked();
    await user.click(screen.getByRole("menuitemradio", { name: "7 ngày qua" }));

    await waitFor(() => expect(lastAsked(asked).searchParams.get("range")).toBe("7d"));
    expect(router.state.location.search).toBe("");
  });

  it("reads a custom range from the address as its two days", async () => {
    const asked = serves(() => ({ items: ROWS }));
    renderAudit("?range=custom&from=2026-09-18&to=2026-09-24");

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    expect(menu()).toHaveTextContent("18 Th9 – 24 Th9");
    const url = lastAsked(asked);
    expect(url.searchParams.get("range")).toBe("custom");
    expect(url.searchParams.get("from")).toBe("2026-09-18");
    expect(url.searchParams.get("to")).toBe("2026-09-24");
  });

  it("falls back to the default for a custom range that has lost a day or runs backwards", async () => {
    const asked = serves(() => ({ items: ROWS }));
    renderAudit("?range=custom&from=2026-09-24&to=2026-09-18");

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    expect(lastAsked(asked).searchParams.get("range")).toBe("7d");
    expect(lastAsked(asked).searchParams.has("from")).toBe(false);
  });

  it("opens the custom dialog on the last seven days and applies them", async () => {
    const asked = serves(() => ({ items: ROWS }));
    const { router } = renderAudit();
    const user = userEvent.setup();

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    await user.click(menu());
    await user.click(await screen.findByRole("menuitem", { name: "Khoảng tuỳ chọn…" }));
    const dialog = await screen.findByRole("dialog", { name: "Khoảng ngày tuỳ chọn" });
    await user.click(within(dialog).getByRole("button", { name: "Áp dụng" }));

    await waitFor(() =>
      expect(lastAsked(asked).searchParams.get("range")).toBe("custom"),
    );
    expect(router.state.location.search).toMatch(
      /^\?range=custom&from=\d{4}-\d{2}-\d{2}&to=\d{4}-\d{2}-\d{2}$/,
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("where focus goes", () => {
  const search = () =>
    screen.getByRole("textbox", { name: "Tìm trong nhật ký kiểm tra" });

  it("returns to the search field after its clear button, which leaves", async () => {
    serves(() => ({ items: ROWS }));
    renderAudit("?q=lớp");
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Xoá tìm kiếm" }));

    expect(screen.queryByRole("button", { name: "Xoá tìm kiếm" })).toBeNull();
    expect(search()).toHaveFocus();
  });

  it("goes to the search field after Reset, which leaves", async () => {
    serves(() => ({ items: ROWS }));
    renderAudit("?range=30d");
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Đặt lại" }));

    expect(screen.queryByRole("button", { name: "Đặt lại" })).toBeNull();
    expect(search()).toHaveFocus();
  });

  it.each([
    ["applying it", "Áp dụng"],
    ["cancelling it", "Huỷ"],
  ])("returns to the range button after the custom dialog, %s", async (_, name) => {
    serves(() => ({ items: ROWS }));
    renderAudit();
    const user = userEvent.setup();

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    await user.click(screen.getByRole("button", { name: /^Khoảng thời gian:/ }));
    await user.click(await screen.findByRole("menuitem", { name: "Khoảng tuỳ chọn…" }));
    const dialog = await screen.findByRole("dialog", { name: "Khoảng ngày tuỳ chọn" });
    await user.click(within(dialog).getByRole("button", { name }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /^Khoảng thời gian:/ })).toHaveFocus(),
    );
  });
});

describe("reset", () => {
  it("shows while a search or another range is set, and clears both but keeps the category", async () => {
    const asked = serves(() => ({ items: ROWS }));
    const { router } = renderAudit("?category=tests&q=nam&range=30d&page=2");
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Đặt lại" }));

    expect(router.state.location.search).toBe("?category=tests");
    await waitFor(() => expect(lastAsked(asked).searchParams.get("range")).toBe("7d"));
    expect(lastAsked(asked).searchParams.has("q")).toBe(false);
    expect(screen.queryByRole("button", { name: "Đặt lại" })).toBeNull();
  });

  it("is not there on the default range with no search, a category included", async () => {
    serves(() => ({ items: ROWS }));
    renderAudit("?category=tests");

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    expect(screen.queryByRole("button", { name: "Đặt lại" })).toBeNull();
  });
});

describe("the states", () => {
  it("draws skeleton rows while the first read is out", async () => {
    server.use(http.get(`${BASE_URL}/admin/audit-log`, () => new Promise(() => {})));
    renderAudit();

    expect(screen.getByRole("status", { name: "Đang tải…" })).toBeVisible();
    expect(screen.getByRole("heading", { level: 1 })).toBeVisible();
  });

  it("says the log is empty in the default range", async () => {
    serves(() => ({ items: [], total: 0, facets: { ...FACETS, all: 0 } }));
    renderAudit();

    expect(await screen.findByText("Không có mục nào trong khoảng này.")).toBeVisible();
    expect(screen.getByText("Không có mục nào")).toBeVisible();
  });

  it.each([
    ["a search", "?q=zzz"],
    ["a category", "?category=media"],
    ["a range", "?range=today"],
  ])("says no entry matches when %s is set", async (_, search) => {
    serves(() => ({ items: [], total: 0 }));
    renderAudit(search);

    expect(await screen.findByText("Không có mục nào khớp bộ lọc này.")).toBeVisible();
  });

  it("says so, with a retry, when the log cannot be read", async () => {
    let failing = true;
    server.use(
      http.get(`${BASE_URL}/admin/audit-log`, () =>
        failing
          ? contractJson("/admin/audit-log", "get", 403, {
              error: {
                code: "FORBIDDEN",
                message: "Không đủ quyền.",
                requestId: REQUEST_ID,
              },
            })
          : contractJson("/admin/audit-log", "get", 200, {
              items: [CREATED],
              page: 1,
              pageSize: 10,
              total: 1,
              facets: FACETS,
            }),
      ),
    );
    renderAudit();
    const user = userEvent.setup();

    expect(await screen.findByText("Không tải được nhật ký kiểm tra.")).toBeVisible();
    failing = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    expect(await screen.findByRole("button", { name: /Đã tạo lớp/ })).toBeVisible();
  });
});

describe("the pager", () => {
  it("counts the filtered rows, ten to a page, with no size control", async () => {
    serves(() => ({ items: ROWS, total: 312 }));
    renderAudit("?q=lớp");

    expect(await screen.findByText("1–10 trên 312 mục")).toBeVisible();
    expect(screen.getByText("Trang 1 trên 32")).toBeVisible();
    expect(screen.queryByText("Số dòng mỗi trang")).toBeNull();
  });

  it("replaces a page past the last in the address by the last, and shows its rows", async () => {
    const asked = serves((url) => ({
      items: Number(url.searchParams.get("page")) > 126 ? [] : ROWS,
      total: 1253,
    }));
    const { router } = renderAudit("?page=1000");

    expect(await screen.findByRole("button", { name: /Đã tạo lớp/ })).toBeVisible();
    expect(router.state.location.search).toBe("?page=126");
    expect(lastAsked(asked).searchParams.get("page")).toBe("126");
    expect(screen.queryByText("Không có mục nào trong khoảng này.")).toBeNull();
  });

  it("leaves a page the log has alone, and one while the rows of another filter are still on screen", async () => {
    serves((url) => ({
      items: ROWS,
      total: url.searchParams.get("q") === "nam" ? 10 : 312,
    }));
    const { router } = renderAudit("?page=4");

    await screen.findByText("31–40 trên 312 mục");
    expect(router.state.location.search).toBe("?page=4");
  });

  it("asks for the page the address names", async () => {
    const asked = serves(() => ({ items: ROWS, total: 312 }));
    renderAudit("?page=4");

    await screen.findByText("31–40 trên 312 mục");
    expect(lastAsked(asked).searchParams.get("page")).toBe("4");
    expect(lastAsked(asked).searchParams.get("limit")).toBe("10");
  });

  it("is not tied to who did what: nothing in the request names an actor", async () => {
    const asked = serves(() => ({ items: ROWS }));
    renderAudit();

    await screen.findByRole("button", { name: /Đã tạo lớp/ });
    expect(
      [...lastAsked(asked).searchParams.keys()].sort((a, b) => a.localeCompare(b)),
    ).toEqual(["limit", "page", "range"]);
    expect(ACTOR.name).toBe("Giàu Bùi");
  });
});
