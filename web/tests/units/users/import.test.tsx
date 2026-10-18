import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import i18n from "@/lib/i18n";
import en from "@/lib/i18n/locales/en.json";
import vi_ from "@/lib/i18n/locales/vi.json";
import { toast } from "@/components/ui/sonner";
import { ISSUE_CODES, issueReason } from "@/features/users/importText";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { loadSpec } from "@tests/support/openapi";
import { server } from "@tests/support/server";
import { BASE, renderUsers, serveUsers, user } from "./support";

vi.mock("@/components/ui/sonner", async (original) => ({
  ...(await original<typeof import("@/components/ui/sonner")>()),
  toast: vi.fn(),
}));

type Preview = components["schemas"]["UserImportPreview"];
type Job = components["schemas"]["UserImportJob"];

const JOB_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const ROWS = [user("1", "Nguyễn Văn An")];

function preview(overrides: Partial<Preview> = {}): Preview {
  return {
    rows: 31,
    newAccounts: 26,
    existingEmails: { count: 2, sample: ["an@example.com", "binh@example.com"] },
    invalid: [
      { row: 4, field: "email", code: "FORMULA_LIKE" },
      { row: 9, field: "role", code: "UNKNOWN_ROLE" },
      { row: 12, field: "full_name", code: "REQUIRED" },
    ],
    invalidCount: 3,
    classes: [
      { id: "019535d9-3df7-79fb-b466-fa907fa17f01", name: "TOEIC 600 Cuối tuần" },
      { id: "019535d9-3df7-79fb-b466-fa907fa17f02", name: "IELTS Nền tảng A" },
    ],
    emailBudget: { remainingToday: 300, limit: 300 },
    ...overrides,
  };
}

function job(overrides: Partial<Job> = {}): Job {
  return {
    id: JOB_ID,
    status: "running",
    total: 28,
    processed: 10,
    created: 9,
    skipped: 1,
    invalid: 3,
    delivery: "email_link",
    sheet: null,
    errorCode: null,
    createdAt: "2026-10-15T08:00:00Z",
    ...overrides,
  };
}

type Sent = Record<string, string>;

/**
 * recordForms records the fields of every multipart form the page builds, as
 * the browser built it: the mocked network drops a file's name and Node's
 * parser refuses a jsdom File, so the form is never parsed on the way in.
 */
function recordForms() {
  const forms: Sent[] = [];
  let fields: Sent = {};
  const append = FormData.prototype.append as (
    this: FormData,
    ...args: unknown[]
  ) => void;
  vi.spyOn(FormData.prototype, "append").mockImplementation(function (
    this: FormData,
    ...args: unknown[]
  ) {
    const [name, value] = args;
    if (name === "file" && value instanceof File) {
      forms.push({ ...fields, file: value.name });
      fields = {};
    } else fields[String(name)] = String(value);
    append.apply(this, args);
  } as never);
  return forms;
}

function servePreview(answer: (call: number) => Response) {
  let calls = 0;
  server.use(http.post(`${BASE}/admin/user-imports/preview`, () => answer(calls++)));
}

const ok = (body: Preview) =>
  contractJson("/admin/user-imports/preview", "post", 200, body);

function refusal(status: number, code: string, message = "server text") {
  return HttpResponse.json(
    { error: { code, message, requestId: "req-1" } },
    { status },
  );
}

const csv = (name = "hoc-vien-thang-10.csv", size = 120) =>
  new File(["x".repeat(size)], name, { type: "text/csv" });

const fileInput = () => document.querySelector<HTMLInputElement>('input[type="file"]')!;

async function openImport(person: ReturnType<typeof userEvent.setup>) {
  await person.click(await screen.findByRole("button", { name: "Nhập CSV" }));
  return within(await screen.findByRole("dialog", { name: "Nhập người dùng" }));
}

beforeEach(() => {
  serveUsers(ROWS);
});

afterEach(async () => {
  vi.restoreAllMocks();
  vi.useRealTimers();
  useAuthStore.getState().clearSession();
  await act(() => i18n.changeLanguage("vi"));
});

describe("the Import users dialog", () => {
  it("asks for a file, the role for rows without one and how sign-in details reach people", async () => {
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    expect(dialog.getByText("Cột: full_name, email, role, class.")).toBeVisible();
    expect(dialog.getByText("CSV hoặc XLSX · tối đa 5 MB")).toBeVisible();
    expect(
      dialog.getByRole("group", { name: "Vai trò nếu tệp không có" }),
    ).toBeVisible();
    expect(dialog.getByRole("button", { name: "Học viên" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      dialog.getByRole("switch", { name: "Gửi thông tin đăng nhập qua email" }),
    ).toBeChecked();
    expect(
      dialog.getByText("Nếu tắt, tải bảng mật khẩu tạm để phát tay"),
    ).toBeVisible();
    expect(dialog.getByRole("button", { name: "Nhập" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("previews a file: new accounts, skipped emails, classes and the rows refused", async () => {
    const forms = recordForms();
    servePreview(() => ok(preview()));
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv());

    expect(await dialog.findByText("26 tài khoản mới")).toBeVisible();
    expect(dialog.getByText("2 email đã có và sẽ được bỏ qua")).toBeVisible();
    expect(dialog.getByText("Ví dụ: an@example.com, binh@example.com")).toBeVisible();
    expect(
      dialog.getByText("Thêm vào TOEIC 600 Cuối tuần và IELTS Nền tảng A"),
    ).toBeVisible();
    expect(dialog.getByText("3 dòng có lỗi")).toBeVisible();
    expect(forms).toEqual([
      { defaultRole: "student", delivery: "email_link", file: "hoc-vien-thang-10.csv" },
    ]);

    const disclose = dialog.getByRole("button", { name: "Xem lỗi" });
    expect(disclose).toHaveAttribute("aria-expanded", "false");
    await person.click(disclose);
    expect(disclose).toHaveAttribute("aria-expanded", "true");
    const table = dialog.getByRole("table", { name: "Các ô có lỗi" });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows.map((row) => row.textContent)).toEqual([
      "4email" + issueReason("FORMULA_LIKE", i18n.t),
      "9role" + issueReason("UNKNOWN_ROLE", i18n.t),
      "12full_name" + issueReason("REQUIRED", i18n.t),
    ]);
    expect(dialog.getByRole("button", { name: "Nhập 26 người" })).not.toHaveAttribute(
      "aria-disabled",
    );
  });

  it("reads the preview again when the role or the hand-out changes", async () => {
    const forms = recordForms();
    servePreview(() => ok(preview({ emailBudget: null })));
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv());
    await dialog.findByText("26 tài khoản mới");
    await person.click(dialog.getByRole("button", { name: "Giáo viên" }));
    await waitFor(() => expect(forms).toHaveLength(2));
    await person.click(dialog.getByRole("switch"));
    await waitFor(() => expect(forms).toHaveLength(3));

    expect(forms.map(({ defaultRole, delivery }) => [defaultRole, delivery])).toEqual([
      ["student", "email_link"],
      ["teacher", "email_link"],
      ["teacher", "sheet"],
    ]);
  });

  it("warns when the links outrun what the centre can still send today", async () => {
    servePreview(() =>
      ok(
        preview({ newAccounts: 181, emailBudget: { remainingToday: 180, limit: 300 } }),
      ),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv());

    expect(
      await dialog.findByText(
        "Hôm nay còn gửi được 180 email; phần còn lại sẽ gửi vào những ngày sau.",
      ),
    ).toBeVisible();
  });

  it("does not warn when today's budget covers every link", async () => {
    servePreview(() =>
      ok(
        preview({ newAccounts: 180, emailBudget: { remainingToday: 180, limit: 300 } }),
      ),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv());

    await dialog.findByText("180 tài khoản mới");
    expect(dialog.queryByText(/Hôm nay còn gửi được/)).toBeNull();
  });

  it("keeps Import paused when no row would create an account", async () => {
    servePreview(() => ok(preview({ newAccounts: 0 })));
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv());

    expect(await dialog.findByText("0 tài khoản mới")).toBeVisible();
    expect(dialog.getByText(/^Không có dòng nào tạo được tài khoản mới/)).toBeVisible();
    expect(dialog.getByRole("button", { name: "Nhập" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("refuses a file over 5 MB without sending it, and names CSV UTF-8 as the way out", async () => {
    const forms = recordForms();
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv("lon.xlsx", 5 * 1024 * 1024 + 1));

    expect(await dialog.findByRole("alert")).toHaveTextContent(
      "Tệp lớn hơn 5 MB. Hãy chia danh sách thành nhiều tệp nhỏ hơn, hoặc lưu thành CSV UTF-8 cho nhẹ hơn, rồi tải lên.",
    );
    expect(forms).toEqual([]);
  });

  it("sends a file of exactly 5 MiB", async () => {
    const forms = recordForms();
    servePreview(() => ok(preview()));
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv("vua.csv", 5 * 1024 * 1024));

    expect(await dialog.findByText("26 tài khoản mới")).toBeVisible();
    expect(forms).toHaveLength(1);
    expect(dialog.queryByRole("alert")).toBeNull();
  });

  it.each([
    [
      413,
      "USER_IMPORT_FILE_TOO_LARGE",
      "Tệp lớn hơn 5 MB. Hãy chia danh sách thành nhiều tệp nhỏ hơn, hoặc lưu thành CSV UTF-8 cho nhẹ hơn, rồi tải lên.",
    ],
    [
      422,
      "CSV_NOT_UTF8",
      'Tệp CSV không dùng mã hoá UTF-8. Hãy lưu lại thành "CSV UTF-8" rồi tải lên.',
    ],
    [
      422,
      "USER_IMPORT_TOO_MANY_ROWS",
      "Tệp có hơn 2.000 dòng dữ liệu. Hãy chia thành nhiều tệp, mỗi tệp tối đa 2.000 dòng.",
    ],
    [422, "USER_IMPORT_COLUMNS", "Dòng tiêu đề thiếu cột bắt buộc: email."],
  ])("says why a file was refused (%i %s)", async (status, code, message) => {
    servePreview(() =>
      refusal(status, code, "Dòng tiêu đề thiếu cột bắt buộc: email."),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv());

    expect(await dialog.findByRole("alert")).toHaveTextContent(message);
    expect(dialog.queryByRole("button", { name: "Thử lại" })).toBeNull();
    expect(dialog.getByRole("button", { name: "Nhập" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("retries a start whose answer was lost with the same requestId, so it cannot import twice", async () => {
    const forms = recordForms();
    servePreview(() => ok(preview()));
    let starts = 0;
    server.use(
      http.post(`${BASE}/admin/user-imports`, () => {
        starts += 1;
        return starts === 1
          ? refusal(503, "INTERNAL")
          : contractJson(
              "/admin/user-imports",
              "post",
              202,
              job({ status: "queued", processed: 0, created: 0, skipped: 0 }),
            );
      }),
      http.get(`${BASE}/admin/user-imports/:id`, () =>
        contractJson(
          "/admin/user-imports/{id}",
          "get",
          200,
          job({ status: "queued", processed: 0, created: 0, skipped: 0 }),
        ),
      ),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);
    await person.upload(fileInput(), csv());

    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));
    await person.click(await dialog.findByRole("button", { name: "Thử lại" }));

    expect(await dialog.findByText("Đang tạo tài khoản… 0/28")).toBeVisible();
    const creates = forms.filter((form) => "requestId" in form);
    expect(creates).toHaveLength(2);
    expect(creates[0]!.requestId).toMatch(/^[0-9a-f-]{36}$/);
    expect(creates[1]!.requestId).toBe(creates[0]!.requestId);
  });

  it("follows the job every two seconds to its end and reads the Users list again", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    servePreview(() => ok(preview()));
    const states = [
      job(),
      job({ processed: 20, created: 19 }),
      job({ status: "done", processed: 28, created: 26, skipped: 2 }),
    ];
    let reads = 0;
    const lists = serveUsers(ROWS);
    server.use(
      http.post(`${BASE}/admin/user-imports`, () =>
        contractJson(
          "/admin/user-imports",
          "post",
          202,
          job({ status: "queued", processed: 0, created: 0, skipped: 0 }),
        ),
      ),
      http.get(`${BASE}/admin/user-imports/:id`, () =>
        contractJson(
          "/admin/user-imports/{id}",
          "get",
          200,
          states[Math.min(reads++, states.length - 1)],
        ),
      ),
    );
    renderUsers();
    const person = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const dialog = await openImport(person);
    await person.upload(fileInput(), csv());
    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));

    expect(await dialog.findByText("Đang tạo tài khoản… 10/28")).toBeVisible();
    expect(
      dialog.getByRole("progressbar", { name: "Tiến độ nhập người dùng" }),
    ).toHaveAttribute("aria-valuenow", "36");
    const before = lists.length;
    const firstReads = reads;
    await act(() => vi.advanceTimersByTimeAsync(1_700));
    expect(reads).toBe(firstReads);
    await act(() => vi.advanceTimersByTimeAsync(300));
    expect(await dialog.findByText("Đang tạo tài khoản… 20/28")).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(2_000));

    expect(await dialog.findByText("26 tài khoản đã tạo · 2 bỏ qua")).toBeVisible();
    expect(dialog.getByText("3 dòng có lỗi không được nhập.")).toBeVisible();
    expect(dialog.getByRole("button", { name: "Nhập tệp khác" })).toBeVisible();
    await waitFor(() => expect(lists.length).toBeGreaterThan(before));
    const settledReads = reads;
    await act(() => vi.advanceTimersByTimeAsync(6_000));
    expect(reads).toBe(settledReads);
  });

  it("keeps following a job after the dialog closes, shows it again on reopening, and says how it ended", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    servePreview(() => ok(preview()));
    let reads = 0;
    server.use(
      http.post(`${BASE}/admin/user-imports`, () =>
        contractJson("/admin/user-imports", "post", 202, job()),
      ),
      http.get(`${BASE}/admin/user-imports/:id`, () => {
        reads += 1;
        return contractJson(
          "/admin/user-imports/{id}",
          "get",
          200,
          reads < 3
            ? job()
            : job({ status: "done", processed: 28, created: 26, skipped: 2 }),
        );
      }),
    );
    renderUsers();
    const person = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    let dialog = await openImport(person);
    await person.upload(fileInput(), csv());
    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));
    await dialog.findByText("Đang tạo tài khoản… 10/28");

    await person.click(dialog.getAllByRole("button", { name: "Đóng" }).at(-1)!);
    expect(screen.queryByRole("dialog")).toBeNull();
    dialog = await openImport(person);
    expect(dialog.getByText("Đang tạo tài khoản… 10/28")).toBeVisible();
    await person.click(dialog.getAllByRole("button", { name: "Đóng" }).at(-1)!);

    const closedAt = reads;
    await act(() => vi.advanceTimersByTimeAsync(2_000));
    expect(reads).toBe(closedAt + 1);
    await act(() => vi.advanceTimersByTimeAsync(2_000));
    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith("26 tài khoản đã tạo · 2 bỏ qua"),
    );
  });

  it("hands the password sheet out once, saying so before the download", async () => {
    const created = vi.fn(() => "blob:sheet");
    vi.stubGlobal(
      "URL",
      Object.assign(URL, { createObjectURL: created, revokeObjectURL: vi.fn() }),
    );
    servePreview(() => ok(preview({ emailBudget: null })));
    let taken = false;
    let downloads = 0;
    const sheetJob = () =>
      job({
        status: "done",
        processed: 28,
        created: 26,
        skipped: 2,
        delivery: "sheet",
        sheet: {
          available: !taken,
          expiresAt: "2026-10-16T08:05:00Z",
          downloadedAt: taken ? "2026-10-15T08:07:00Z" : null,
        },
      });
    server.use(
      http.post(`${BASE}/admin/user-imports`, () =>
        contractJson("/admin/user-imports", "post", 202, sheetJob()),
      ),
      http.get(`${BASE}/admin/user-imports/:id`, () =>
        contractJson("/admin/user-imports/{id}", "get", 200, sheetJob()),
      ),
      http.post(`${BASE}/admin/user-imports/:id/sheet`, () => {
        downloads += 1;
        if (taken) return refusal(410, "SHEET_UNAVAILABLE");
        taken = true;
        return new HttpResponse("full_name,email,password\r\n", {
          headers: {
            "Content-Type": "text/csv",
            "Content-Disposition": 'attachment; filename="new-accounts-20261015.csv"',
          },
        });
      }),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);
    await person.upload(fileInput(), csv());
    await person.click(await dialog.findByRole("switch"));
    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));

    expect(
      await dialog.findByText(/Bảng mật khẩu chỉ tải được một lần và bị xoá lúc/),
    ).toBeVisible();
    await person.click(dialog.getByRole("button", { name: "Tải bảng mật khẩu" }));

    expect(await dialog.findByText(/^Đã tải lúc /)).toBeVisible();
    expect(dialog.queryByRole("button", { name: "Tải bảng mật khẩu" })).toBeNull();
    expect(downloads).toBe(1);
    expect(created).toHaveBeenCalledOnce();
  });

  it("says the sheet is gone when the server no longer has it", async () => {
    servePreview(() => ok(preview({ emailBudget: null })));
    const doneSheet = job({
      status: "done",
      processed: 28,
      created: 26,
      skipped: 2,
      delivery: "sheet",
      sheet: { available: true, expiresAt: "2026-10-16T08:05:00Z", downloadedAt: null },
    });
    server.use(
      http.post(`${BASE}/admin/user-imports`, () =>
        contractJson("/admin/user-imports", "post", 202, doneSheet),
      ),
      http.get(`${BASE}/admin/user-imports/:id`, () =>
        contractJson("/admin/user-imports/{id}", "get", 200, doneSheet),
      ),
      http.post(`${BASE}/admin/user-imports/:id/sheet`, () =>
        refusal(410, "SHEET_UNAVAILABLE"),
      ),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);
    await person.upload(fileInput(), csv());
    await person.click(await dialog.findByRole("switch"));
    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));
    await person.click(
      await dialog.findByRole("button", { name: "Tải bảng mật khẩu" }),
    );

    expect(await dialog.findByRole("alert")).toHaveTextContent(
      "Bảng mật khẩu không còn để tải: đã được tải hoặc đã quá 24 giờ.",
    );
  });

  it("says why a job stopped and that the accounts made before it stay", async () => {
    servePreview(() => ok(preview()));
    const failed = job({
      status: "failed",
      processed: 12,
      created: 11,
      errorCode: "WORKER_INTERRUPTED",
    });
    server.use(
      http.post(`${BASE}/admin/user-imports`, () =>
        contractJson("/admin/user-imports", "post", 202, failed),
      ),
      http.get(`${BASE}/admin/user-imports/:id`, () =>
        contractJson("/admin/user-imports/{id}", "get", 200, failed),
      ),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);
    await person.upload(fileInput(), csv());
    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));

    expect(
      await dialog.findByText("Việc nhập bị gián đoạn nhiều lần nên đã dừng."),
    ).toBeVisible();
    expect(
      dialog.getByText("11 tài khoản đã tạo trước đó vẫn được giữ."),
    ).toBeVisible();
  });
});

function sheetJob(sheet: Partial<NonNullable<Job["sheet"]>> = {}): Job {
  return job({
    status: "done",
    processed: 28,
    created: 26,
    skipped: 2,
    delivery: "sheet",
    sheet: {
      available: true,
      expiresAt: "2026-10-16T08:05:00Z",
      downloadedAt: null,
      ...sheet,
    },
  });
}

function serveJob(read: () => Job | Response) {
  server.use(
    http.post(`${BASE}/admin/user-imports`, () => {
      const first = read();
      return first instanceof Response
        ? first
        : contractJson("/admin/user-imports", "post", 202, first);
    }),
    http.get(`${BASE}/admin/user-imports/:id`, () => {
      const now = read();
      return now instanceof Response
        ? now
        : contractJson("/admin/user-imports/{id}", "get", 200, now);
    }),
  );
}

async function startImport(
  person: ReturnType<typeof userEvent.setup>,
  { sheet = false } = {},
) {
  const dialog = await openImport(person);
  await person.upload(fileInput(), csv());
  if (sheet) await person.click(await dialog.findByRole("switch"));
  await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));
  return dialog;
}

function stubObjectUrls() {
  const created = vi.fn(() => "blob:sheet");
  vi.stubGlobal(
    "URL",
    Object.assign(URL, { createObjectURL: created, revokeObjectURL: vi.fn() }),
  );
  return created;
}

describe("the Import users dialog at its edges", () => {
  it("takes a new requestId when another file is chosen", async () => {
    const forms = recordForms();
    servePreview(() => ok(preview()));
    server.use(http.post(`${BASE}/admin/user-imports`, () => refusal(503, "INTERNAL")));
    const { user: person } = renderUsers();
    const dialog = await openImport(person);
    await person.upload(fileInput(), csv("mot.csv"));
    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));
    await dialog.findByRole("button", { name: "Thử lại" });

    await person.upload(fileInput(), csv("hai.csv"));
    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));
    await dialog.findByRole("button", { name: "Thử lại" });

    const creates = forms.filter((form) => "requestId" in form);
    expect(creates.map((form) => form.file)).toEqual(["mot.csv", "hai.csv"]);
    expect(creates[1]!.requestId).not.toBe(creates[0]!.requestId);
  });

  it("keeps Import paused while the preview is read, and sends one start for two quick clicks", async () => {
    let release: () => void = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    servePreview(() => ok(preview()));
    server.use(
      http.post(`${BASE}/admin/user-imports/preview`, async () => {
        await held;
        return ok(preview());
      }),
    );
    let starts = 0;
    serveJob(() => job());
    server.use(
      http.post(`${BASE}/admin/user-imports`, async () => {
        starts += 1;
        await new Promise((resolve) => setTimeout(resolve, 50));
        return contractJson("/admin/user-imports", "post", 202, job());
      }),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);
    await person.upload(fileInput(), csv());

    await dialog.findByText("Đang đọc tệp…");
    expect(dialog.getByRole("button", { name: "Nhập" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    release();
    await person.dblClick(await dialog.findByRole("button", { name: "Nhập 26 người" }));

    expect(await dialog.findByText("Đang tạo tài khoản… 10/28")).toBeVisible();
    expect(starts).toBe(1);
  });

  it("forgets a refused start when the role or the hand-out changes", async () => {
    servePreview(() => ok(preview()));
    server.use(
      http.post(`${BASE}/admin/user-imports`, () =>
        refusal(422, "USER_IMPORT_NOTHING_TO_CREATE"),
      ),
    );
    const { user: person } = renderUsers();
    const dialog = await openImport(person);
    await person.upload(fileInput(), csv());
    await person.click(await dialog.findByRole("button", { name: "Nhập 26 người" }));
    await dialog.findByRole("alert");

    await person.click(dialog.getByRole("button", { name: "Giáo viên" }));

    expect(await dialog.findByText("26 tài khoản mới")).toBeVisible();
    expect(dialog.queryByRole("alert")).toBeNull();
  });

  it("does not offer Retry on a server that is not set up for imports, nor name its settings", async () => {
    servePreview(() => refusal(501, "USER_IMPORT_OFF"));
    const { user: person } = renderUsers();
    const dialog = await openImport(person);

    await person.upload(fileInput(), csv());

    const alert = await dialog.findByRole("alert");
    expect(alert).toHaveTextContent(
      "Máy chủ chưa được cấu hình để nhập người dùng. Hãy báo người vận hành.",
    );
    expect(alert).not.toHaveTextContent("SECRETS_KEY");
    expect(dialog.queryByRole("button", { name: "Thử lại" })).toBeNull();
  });

  it("slows to a read every thirty seconds when reads fail, says so with Retry, and recovers", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    servePreview(() => ok(preview()));
    let reads = 0;
    let failing = false;
    serveJob(() => {
      if (failing) {
        reads += 1;
        return refusal(403, "FORBIDDEN");
      }
      return job();
    });
    renderUsers();
    const person = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const dialog = await startImport(person);
    await dialog.findByText("Đang tạo tài khoản… 10/28");

    failing = true;
    await act(() => vi.advanceTimersByTimeAsync(2_000));
    expect(await dialog.findByRole("alert")).toHaveTextContent(
      "Không đọc được tiến độ. Việc nhập có thể vẫn đang chạy trên máy chủ; hãy thử lại.",
    );
    expect(dialog.getByRole("button", { name: "Thử lại" })).toBeVisible();
    const failedReads = reads;
    await act(() => vi.advanceTimersByTimeAsync(6_000));
    expect(reads).toBe(failedReads);

    failing = false;
    await act(() => vi.advanceTimersByTimeAsync(24_000));
    await waitFor(() => expect(dialog.queryByRole("alert")).toBeNull());
  });

  it("reads the preview of a file chosen after a reset, never the previous file's", async () => {
    const forms = recordForms();
    servePreview((call) =>
      ok(
        call === 0
          ? preview({ newAccounts: 1, emailBudget: null })
          : preview({ newAccounts: 3, emailBudget: null }),
      ),
    );
    const { user: person } = renderUsers();
    let dialog = await openImport(person);
    await person.upload(fileInput(), csv("a.csv"));
    await dialog.findByText("1 tài khoản mới");
    await person.click(dialog.getByRole("button", { name: "Huỷ" }));

    dialog = await openImport(person);
    await person.upload(fileInput(), csv("b.csv"));

    expect(await dialog.findByText("3 tài khoản mới")).toBeVisible();
    expect(dialog.getByRole("button", { name: "Nhập 3 người" })).toBeVisible();
    expect(forms.map((form) => form.file)).toEqual(["a.csv", "b.csv"]);
  });

  it("says the sheet is downloading while it is, and does not ask for a reset after it was saved", async () => {
    stubObjectUrls();
    servePreview(() => ok(preview({ emailBudget: null })));
    let taken = false;
    let release: () => void = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    serveJob(() =>
      sheetJob(taken ? { available: false, downloadedAt: "2026-10-15T08:07:00Z" } : {}),
    );
    server.use(
      http.post(`${BASE}/admin/user-imports/:id/sheet`, async () => {
        await held;
        taken = true;
        return new HttpResponse("full_name,email,password\r\n", {
          headers: { "Content-Type": "text/csv" },
        });
      }),
    );
    const { user: person } = renderUsers();
    let dialog = await startImport(person, { sheet: true });

    await person.click(
      await dialog.findByRole("button", { name: "Tải bảng mật khẩu" }),
    );
    const busy = await dialog.findByRole("button", { name: "Đang tải…" });
    expect(busy).toHaveAttribute("aria-busy", "true");
    release();
    expect(await dialog.findByText(/^Đã tải lúc /)).toBeVisible();

    await person.click(dialog.getByRole("button", { name: "Xong" }));
    dialog = await openImport(person);
    expect(dialog.queryByText(/hãy đặt lại mật khẩu/)).toBeNull();
  });

  it("tells a sheet import to stay on the page, and asks before the tab closes", async () => {
    servePreview(() => ok(preview({ emailBudget: null })));
    serveJob(() => job({ delivery: "sheet" }));
    const { user: person } = renderUsers();
    const dialog = await startImport(person, { sheet: true });

    expect(
      await dialog.findByText(
        "Bạn có thể đóng hộp thoại; việc nhập vẫn tiếp tục. Hãy ở lại trang này: khi việc nhập xong, bảng mật khẩu chỉ tải được ở đây.",
      ),
    ).toBeVisible();
    const leaving = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(leaving);
    expect(leaving.defaultPrevented).toBe(true);
  });

  it("puts the once-only warning before the download button, and keeps the sheet when the dialog closes", async () => {
    servePreview(() => ok(preview({ emailBudget: null })));
    serveJob(() => sheetJob());
    const { user: person } = renderUsers();
    let dialog = await startImport(person, { sheet: true });

    const warning = await dialog.findByText(/Bảng mật khẩu chỉ tải được một lần/);
    const button = dialog.getByRole("button", { name: "Tải bảng mật khẩu" });
    expect(
      warning.compareDocumentPosition(button) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    await person.click(dialog.getByRole("button", { name: "Xong" }));
    dialog = await openImport(person);

    expect(dialog.getByRole("button", { name: "Tải bảng mật khẩu" })).toBeVisible();
  });

  it("never reads as downloaded when the file did not arrive, and says to reset the passwords", async () => {
    const created = stubObjectUrls();
    servePreview(() => ok(preview({ emailBudget: null })));
    let taken = false;
    serveJob(() =>
      sheetJob(taken ? { available: false, downloadedAt: "2026-10-15T08:07:00Z" } : {}),
    );
    server.use(
      http.post(`${BASE}/admin/user-imports/:id/sheet`, () => {
        taken = true;
        return HttpResponse.error();
      }),
    );
    const { user: person } = renderUsers();
    const dialog = await startImport(person, { sheet: true });

    await person.click(
      await dialog.findByRole("button", { name: "Tải bảng mật khẩu" }),
    );

    expect(await dialog.findByRole("alert")).toHaveTextContent(
      "Tệp không tải về được, nhưng máy chủ đã xoá bảng mật khẩu. Hãy đặt lại mật khẩu cho những tài khoản này từ danh sách Người dùng.",
    );
    expect(dialog.queryByText(/^Đã tải lúc/)).toBeNull();
    expect(created).not.toHaveBeenCalled();
  });

  it("offers the download again when it failed before the server erased the sheet", async () => {
    stubObjectUrls();
    servePreview(() => ok(preview({ emailBudget: null })));
    serveJob(() => sheetJob());
    server.use(
      http.post(`${BASE}/admin/user-imports/:id/sheet`, () => refusal(503, "INTERNAL")),
    );
    const { user: person } = renderUsers();
    const dialog = await startImport(person, { sheet: true });

    await person.click(
      await dialog.findByRole("button", { name: "Tải bảng mật khẩu" }),
    );

    expect(await dialog.findByRole("alert")).toHaveTextContent(
      /^Không tải được bảng mật khẩu. Hãy thử lại ngay/,
    );
    expect(dialog.getByRole("button", { name: "Tải bảng mật khẩu" })).toBeVisible();
  });

  it("says a sheet taken earlier may need a password reset", async () => {
    servePreview(() => ok(preview({ emailBudget: null })));
    serveJob(() =>
      sheetJob({ available: false, downloadedAt: "2026-10-15T08:07:00Z" }),
    );
    const { user: person } = renderUsers();
    const dialog = await startImport(person, { sheet: true });

    expect(
      await dialog.findByText(/^Bảng mật khẩu đã được tải lúc .*hãy đặt lại mật khẩu/),
    ).toBeVisible();
  });

  it("says the accounts a failed sheet import made have no password", async () => {
    servePreview(() => ok(preview({ emailBudget: null })));
    serveJob(() =>
      job({
        status: "failed",
        processed: 12,
        created: 11,
        delivery: "sheet",
        sheet: null,
        errorCode: "WORKER_INTERRUPTED",
      }),
    );
    const { user: person } = renderUsers();
    const dialog = await startImport(person, { sheet: true });

    expect(
      await dialog.findByText(
        "11 tài khoản đã tạo trước đó vẫn được giữ nhưng chưa có mật khẩu. Hãy đặt lại mật khẩu từ danh sách Người dùng.",
      ),
    ).toBeVisible();
  });
});

describe("the row codes", () => {
  it("are the contract's, each with a reason in Vietnamese and in English", () => {
    const spec = loadSpec() as {
      components: {
        schemas: { UserImportIssue: { properties: { code: { enum: string[] } } } };
      };
    };
    expect([...ISSUE_CODES]).toEqual(
      spec.components.schemas.UserImportIssue.properties.code.enum,
    );
    const reasons = (locale: typeof vi_) => locale.users.import.reason;
    for (const locale of [vi_, en]) {
      const map = reasons(locale) as Record<string, string>;
      expect(Object.keys(map)).toHaveLength(ISSUE_CODES.length);
      for (const text of Object.values(map)) expect(text.length).toBeGreaterThan(0);
    }
    expect(reasons(vi_).formulaLike).toMatch(/công thức/);
    expect(reasons(en).formulaLike).toMatch(/formula/);
  });

  it.each([
    ["REQUIRED", "required"],
    ["TOO_LONG", "tooLong"],
    ["INVALID", "invalid"],
    ["FORMULA_LIKE", "formulaLike"],
    ["DUPLICATE_IN_FILE", "duplicateInFile"],
    ["UNKNOWN_ROLE", "unknownRole"],
    ["AMBIGUOUS_ROLE", "ambiguousRole"],
    ["ROLE_NOT_ALLOWED", "roleNotAllowed"],
    ["ROLE_NOT_GIVABLE", "roleNotGivable"],
    ["UNKNOWN_CLASS", "unknownClass"],
    ["AMBIGUOUS_CLASS", "ambiguousClass"],
    ["CLASS_NOT_ALLOWED", "classNotAllowed"],
  ] as const)("say %s with its own reason", (code, key) => {
    expect(issueReason(code, i18n.t)).toBe(vi_.users.import.reason[key]);
  });
});
