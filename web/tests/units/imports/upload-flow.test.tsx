import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportWorkspacePage from "@/features/imports/pages/teacher/ImportWorkspacePage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { useAuthStore } from "@/stores/auth";
import { aiOn, month } from "../ai/support";
import {
  BASE,
  capabilities,
  deferred,
  errorBody,
  source,
  wordImport,
} from "./fixtures";
import "@/lib/i18n";

type Call =
  | { kind: "create"; requestId: string; title: string; readWithAi?: boolean }
  | { kind: "upload"; role: string; uploadId: string; expectedRevision: number }
  | { kind: "process"; requestId: string; expectedRevision: number };

let calls: Call[] = [];
let revision = 1;
let rejectKeyOnce = false;

function docx(name: string, bytes = 2048): File {
  return new File([new Uint8Array(bytes)], name, {
    type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  });
}

beforeEach(() => {
  calls = [];
  revision = 1;
  rejectKeyOnce = false;
  server.use(
    http.get(`${BASE}/teacher/imports/limits`, () =>
      contractJson("/teacher/imports/limits", "get", 200, {
        pasteMaxCharacters: 100000,
        maxBytes: 25 * 1024 * 1024,
        formats: ["docx", "pdf"],
      }),
    ),
    http.post(`${BASE}/teacher/imports`, async ({ request }) => {
      const body = (await request.json()) as { requestId: string; title: string };
      calls.push({ kind: "create", ...body });
      return contractJson(
        "/teacher/imports",
        "post",
        201,
        wordImport({
          title: body.title,
          status: "awaiting_sources",
          revision,
          sourceRevision: 0,
          sources: [],
        }),
      );
    }),
    http.post(`${BASE}/teacher/imports/:id/sources`, ({ request }) => {
      const url = new URL(request.url);
      const role = url.searchParams.get("role") ?? "";
      calls.push({
        kind: "upload",
        role,
        uploadId: url.searchParams.get("uploadId") ?? "",
        expectedRevision: Number(url.searchParams.get("expectedRevision")),
      });
      if (role === "answer_key" && rejectKeyOnce) {
        rejectKeyOnce = false;
        return contractJson(
          "/teacher/imports/{id}/sources",
          "post",
          415,
          errorBody(
            "IMPORT_SOURCE_UNSUPPORTED",
            "Tệp đáp án có macro nên không được nhận.",
          ),
        );
      }
      revision += 1;
      const exam = calls.some((call) => call.kind === "upload" && call.role === "exam");
      const key = role === "answer_key";
      return contractJson("/teacher/imports/{id}/sources", "post", 201, {
        import: wordImport({
          status: "awaiting_sources",
          revision,
          sourceRevision: revision - 1,
          sources: [
            ...(exam ? [source("exam")] : []),
            ...(key ? [source("answer_key")] : []),
          ],
        }),
        source: source(role === "exam" ? "exam" : "answer_key"),
        sourceRevision: revision - 1,
      });
    }),
    http.post(`${BASE}/teacher/imports/:id/process`, async ({ request }) => {
      const body = (await request.json()) as {
        requestId: string;
        expectedRevision: number;
      };
      calls.push({ kind: "process", ...body });
      return contractJson(
        "/teacher/imports/{id}/process",
        "post",
        202,
        wordImport({ status: "queued", revision: revision + 1 }),
      );
    }),
  );
});

function renderPage(entry = "/teacher/imports/new") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports/new", element: <ImportWorkspacePage /> },
      { path: "/teacher/imports/:id", element: <p>detail page</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { ...userEvent.setup(), router };
}

describe("starting a Word import", () => {
  it("states how long the files are kept before anything is uploaded", async () => {
    renderPage();
    expect(
      await screen.findByText(
        /^Tệp gốc và bản rà soát được giữ 30 ngày sau khi tạo đề và 7 ngày sau khi huỷ\. Lượt nhập không có thay đổi trong 60 ngày sẽ tự đóng\.$/,
      ),
    ).toBeInTheDocument();
  });

  it("shows the server's limits before a file is chosen", async () => {
    renderPage();
    expect(
      await screen.findByText(/Nhận tệp \.docx và \.pdf, tối đa 25 MB/),
    ).toBeInTheDocument();
  });

  it("stops offering the upload once the server says processing is switched off", async () => {
    let processing = true;
    server.use(
      http.get(`${BASE}/teacher/imports/capabilities`, () =>
        contractJson("/teacher/imports/capabilities", "get", 200, {
          intakeEnabled: true,
          processingEnabled: processing,
          retention: { afterCommitDays: 30, afterCancelDays: 7, idleDays: 60 },
        }),
      ),
      http.post(`${BASE}/teacher/imports/:id/process`, () => {
        processing = false;
        return contractJson(
          "/teacher/imports/{id}/process",
          "post",
          503,
          errorBody(
            "IMPORT_PROCESSING_UNAVAILABLE",
            "Máy chủ này chưa bật xử lý tài liệu nên chưa thể xử lý lượt nhập.",
          ),
        );
      }),
    );
    const user = renderPage();
    await screen.findByText(/Nhận tệp/);
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("de-thi.docx"));
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));

    expect(
      await screen.findByText("Chưa nhập được đề mới lúc này."),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Tệp đề thi")).toBeNull();
    expect(screen.queryByText("detail page")).toBeNull();
  });

  it("checks processing is still on before creating anything", async () => {
    let processing = true;
    server.use(
      http.get(`${BASE}/teacher/imports/capabilities`, () =>
        contractJson("/teacher/imports/capabilities", "get", 200, {
          intakeEnabled: true,
          processingEnabled: processing,
          retention: { afterCommitDays: 30, afterCancelDays: 7, idleDays: 60 },
        }),
      ),
    );
    const user = renderPage();
    await screen.findByText(/Nhận tệp/);
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("de-thi.docx"));
    processing = false;
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));

    expect(
      await screen.findByText("Chưa nhập được đề mới lúc này."),
    ).toBeInTheDocument();
    expect(calls).toEqual([]);
  });

  it("explains, instead of offering the upload, while processing is switched off", async () => {
    server.use(capabilities(false));
    renderPage();

    expect(
      await screen.findByText("Chưa nhập được đề mới lúc này."),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Tệp đề thi")).toBeNull();
    expect(screen.getByRole("link", { name: "Về mục Nhập đề" })).toHaveAttribute(
      "href",
      "/teacher/imports",
    );
  });

  it("takes a PDF exam and names the test after it", async () => {
    const user = renderPage();
    await screen.findByText(/^Nhận tệp \.docx và \.pdf,/);
    await user.upload(
      screen.getByLabelText("Tệp đề thi"),
      new File(["%PDF-1.4"], "De thi HK2.PDF", { type: "application/pdf" }),
    );

    expect(screen.getByLabelText("Tên đề")).toHaveValue("De thi HK2");
    expect(screen.queryByText(/không phải định dạng được hỗ trợ/)).toBeNull();
  });

  it("creates, uploads the exam, then the key against the exam's revision, then processes", async () => {
    const user = renderPage();
    await screen.findByText(/Nhận tệp/);
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("De thi HK1.docx"));
    await user.upload(screen.getByLabelText("Tệp đáp án"), docx("Dap an HK1.docx"));

    expect(screen.getByLabelText("Tên đề")).toHaveValue("De thi HK1");
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));

    expect(await screen.findByText("detail page")).toBeInTheDocument();
    expect(
      calls.map((call) => (call.kind === "upload" ? call.role : call.kind)),
    ).toEqual(["create", "exam", "answer_key", "process"]);
    const [create, exam, key, process] = calls;
    expect(create).toMatchObject({ kind: "create", title: "De thi HK1" });
    expect(exam).toMatchObject({ kind: "upload", expectedRevision: 1 });
    expect(key, "the key uploads against the revision the exam produced").toMatchObject(
      {
        kind: "upload",
        expectedRevision: 2,
      },
    );
    expect(process).toMatchObject({ kind: "process", expectedRevision: 3 });
  });

  it("puts a refused key beside that file and keeps the accepted exam when the key is replaced", async () => {
    rejectKeyOnce = true;
    const user = renderPage();
    await screen.findByText(/Nhận tệp/);
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("de-thi.docx"));
    await user.upload(screen.getByLabelText("Tệp đáp án"), docx("dap-an.docx"));
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));

    const keyGroup = await screen.findByRole("group", { name: "Tải tệp đáp án" });
    expect(await within(keyGroup.parentElement!).findByRole("alert")).toHaveTextContent(
      "Tệp đáp án có macro nên không được nhận.",
    );
    const examGroup = screen.getByRole("group", { name: "Tải tệp đề thi" });
    expect(within(examGroup).getByText("Đã tải lên")).toBeInTheDocument();
    expect(calls.some((call) => call.kind === "process")).toBe(false);
    expect(screen.getByRole("button", { name: "Bắt đầu xử lý" })).toBeDisabled();

    await user.upload(screen.getByLabelText("Tệp đáp án"), docx("dap-an-sach.docx"));
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));

    expect(await screen.findByText("detail page")).toBeInTheDocument();
    const creates = calls.filter((call) => call.kind === "create");
    const examUploads = calls.filter(
      (call) => call.kind === "upload" && call.role === "exam",
    );
    const keyUploads = calls.filter(
      (call): call is Extract<Call, { kind: "upload" }> =>
        call.kind === "upload" && call.role === "answer_key",
    );
    expect(creates, "the import is created once").toHaveLength(1);
    expect(examUploads, "the accepted exam is not sent again").toHaveLength(1);
    expect(keyUploads).toHaveLength(2);
    expect(keyUploads[1]!.uploadId, "a replacement file is a new upload").not.toBe(
      keyUploads[0]!.uploadId,
    );
    expect(calls.at(-1)).toMatchObject({ kind: "process", expectedRevision: 3 });
  });

  it("refuses a file of the wrong type without contacting the server", async () => {
    const user = userEvent.setup({ applyAccept: false });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter(
      [{ path: "/teacher/imports/new", element: <ImportWorkspacePage /> }],
      { initialEntries: ["/teacher/imports/new"] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    await screen.findByText(/Nhận tệp/);
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("de-thi.odt"));

    expect(await screen.findByRole("alert")).toHaveTextContent(/de-thi\.odt/);
    expect(screen.getByRole("button", { name: "Bắt đầu xử lý" })).toBeDisabled();
    await waitFor(() => expect(calls).toHaveLength(0));
  });

  it("stops quietly when the teacher leaves before the upload finishes", async () => {
    const gate = deferred<void>();
    server.use(
      http.post(`${BASE}/teacher/imports`, async ({ request }) => {
        const body = (await request.json()) as { requestId: string; title: string };
        calls.push({ kind: "create", ...body });
        await gate.promise;
        return contractJson(
          "/teacher/imports",
          "post",
          201,
          wordImport({ status: "awaiting_sources", revision: 1, sources: [] }),
        );
      }),
    );
    const user = userEvent.setup();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter(
      [
        { path: "/teacher/imports/new", element: <ImportWorkspacePage /> },
        { path: "/teacher/imports/:id", element: <p>detail page</p> },
        { path: "/teacher/tests", element: <p>tests page</p> },
      ],
      { initialEntries: ["/teacher/imports/new"] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    await screen.findByText(/Nhận tệp/);
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("de-thi.docx"));
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));
    await waitFor(() => expect(calls).toHaveLength(1));

    await router.navigate("/teacher/tests");
    expect(await screen.findByText("tests page")).toBeInTheDocument();
    gate.resolve();
    await new Promise((settle) => setTimeout(settle, 100));
    expect(router.state.location.pathname).toBe("/teacher/tests");
    expect(calls.map((call) => call.kind)).toEqual(["create"]);
  });

  it("keeps keyboard focus in the slot when a file is chosen and removed", async () => {
    const user = renderPage();
    await screen.findByText(/Nhận tệp/);
    screen.getByRole("button", { name: "Chọn tệp đề thi" }).focus();
    fireEvent.change(screen.getByLabelText("Tệp đề thi"), {
      target: { files: [docx("de-thi.docx")] },
    });
    expect(
      await screen.findByRole("button", { name: "Đổi tệp de-thi.docx" }),
    ).toHaveFocus();

    await user.click(screen.getByRole("button", { name: "Bỏ tệp de-thi.docx" }));
    expect(screen.getByRole("button", { name: "Chọn tệp đề thi" })).toHaveFocus();
  });

  it("lists the formats in the deck's order when the server takes .doc (QA-51-13)", async () => {
    server.use(
      http.get(`${BASE}/teacher/imports/limits`, () =>
        contractJson("/teacher/imports/limits", "get", 200, {
          pasteMaxCharacters: 100000,
          maxBytes: 25 * 1024 * 1024,
          formats: ["docx", "pdf", "doc"],
        }),
      ),
    );
    renderPage();
    const files = screen.getByRole("region", { name: "Nguồn" });
    expect(
      await within(files).findByText("DOCX, DOC hoặc PDF · tối đa 25 MB mỗi tệp"),
    ).toBeInTheDocument();
    expect(
      within(files).getByText(
        /^Nhận tệp \.docx, \.doc và \.pdf, tối đa 25 MB mỗi tệp\./,
      ),
    ).toBeInTheDocument();
  });

  it("draws the workspace's head, its Sources pane and what will run in file mode (DG-207)", async () => {
    renderPage();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Nhập đề từ Word/PDF",
    );
    expect(
      screen.getByText(/^Tải lên đề thi Word hoặc PDF và, nếu có, tệp đáp án riêng\./),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Lịch sử nhập" })).toHaveAttribute(
      "href",
      "/teacher/imports",
    );
    expect(screen.getByRole("button", { name: "Tải tệp lên" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    const files = screen.getByRole("region", { name: "Nguồn" });
    expect(
      await within(files).findByText("DOCX hoặc PDF · tối đa 25 MB mỗi tệp"),
    ).toBeInTheDocument();
    expect(within(files).getAllByText("Bắt buộc")).toHaveLength(1);
    for (const drop of within(files).getAllByText("Kéo tệp vào đây hoặc chọn tệp"))
      expect(drop).toHaveClass("text-ui", "leading-normal");
    expect(within(files).getAllByText("Kéo tệp vào đây hoặc chọn tệp")).toHaveLength(2);
    expect(
      within(files).getByText("Nội dung của bạn được xử lý ở đâu."),
    ).toBeInTheDocument();
    const steps = screen.getByRole("region", { name: "Các bước sẽ chạy" });
    expect(
      within(steps)
        .getAllByRole("listitem")
        .map((item) => item.textContent),
    ).toEqual([
      "1Kiểm tra tệpchưa bắt đầu",
      "2Đọc nội dungchưa bắt đầu",
      "3Nhận diện cấu trúcchưa bắt đầu",
      "4Đối chiếu đáp ánchưa bắt đầu",
      "5Kiểm tra đềchưa bắt đầu",
    ]);
    expect(within(steps).getByLabelText("Tên đề")).toBeInTheDocument();
    expect(
      within(steps).getByText(/^Hệ thống tự nhận diện phần, nhóm câu/),
    ).toHaveClass("text-pretty");
    expect(within(steps).getByText("Tự động nhận diện")).toBeInTheDocument();
    expect(within(steps).getByRole("button", { name: "Bắt đầu xử lý" })).toBeDisabled();
    expect(within(steps).getByRole("link", { name: "Huỷ" })).toHaveAttribute(
      "href",
      "/teacher/imports",
    );
  });

  it("reads paste mode from the URL and shows the text box in place of the files", async () => {
    const user = renderPage("/teacher/imports/new?source=paste&keep=1");
    expect(screen.getByRole("button", { name: "Dán văn bản" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("textbox", { name: "Nội dung đề" })).toBeVisible();
    expect(
      screen.getByRole("group", { name: "Tải tệp đề thi", hidden: true }),
    ).not.toBeVisible();
    expect(screen.getByRole("region", { name: "Đã đọc được" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bắt đầu xử lý" })).toBeDisabled();
    expect(screen.getByText("Cách đọc nội dung")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Tải tệp lên" }));
    expect(user.router.state.location.search).toBe("?keep=1");
    expect(screen.getByRole("button", { name: "Tải tệp lên" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await screen.findByText(/Nhận tệp/);
    expect(calls).toEqual([]);
  });

  it("keeps both chosen files and a typed title across source switches, then uses the real file flow", async () => {
    const user = renderPage("/teacher/imports/new?source=other&keep=1");
    await screen.findByText(/Nhận tệp/);
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("de-thi.docx"));
    await user.upload(screen.getByLabelText("Tệp đáp án"), docx("dap-an.docx"));
    await user.clear(screen.getByLabelText("Tên đề"));
    await user.type(screen.getByLabelText("Tên đề"), "Đề đã sửa");
    await user.click(screen.getByRole("button", { name: "Dán văn bản" }));
    expect(user.router.state.location.search).toBe("?source=paste&keep=1");
    expect(screen.getByLabelText("Tên đề")).toHaveValue("Đề đã sửa");
    expect(screen.getByRole("button", { name: "Bắt đầu xử lý" })).toBeDisabled();
    expect(calls).toEqual([]);
    await user.click(screen.getByRole("button", { name: "Tải tệp lên" }));
    expect(screen.getByText("de-thi.docx")).toBeInTheDocument();
    expect(screen.getByText("dap-an.docx")).toBeInTheDocument();
    expect(screen.getByLabelText("Tên đề")).toHaveValue("Đề đã sửa");
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));
    expect(await screen.findByText("detail page")).toBeInTheDocument();
    expect(
      calls.map((call) => (call.kind === "upload" ? call.role : call.kind)),
    ).toEqual(["create", "exam", "answer_key", "process"]);
    expect(calls[0]).toMatchObject({ title: "Đề đã sửa" });
    expect(user.router.state.historyAction, "the draft's address is replaced").toBe(
      "REPLACE",
    );
  });
});

describe("the Read with AI card (V-11)", () => {
  const card = "Đọc bằng AI";
  const scansAccepted =
    /^Nhận tệp \.docx và \.pdf, tối đa 25 MB mỗi tệp, kể cả PDF chụp hoặc scan\./;
  const scansRefused =
    /^Nhận tệp \.docx và \.pdf, tối đa 25 MB mỗi tệp\. Không nhận .* PDF chụp hoặc scan/;
  const privacyAi =
    /Trang quét hoặc ảnh chụp trong lượt nhập này được một dịch vụ AI bên ngoài Quizzivy đọc\./;

  beforeEach(() =>
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    ),
  );

  afterEach(() => {
    vi.unstubAllGlobals();
    useAuthStore.setState({ accessToken: null, user: null });
  });

  function readImports(on: boolean) {
    useAuthStore.setState({
      user: { ...teacherUser, preferences: { ai: { readImports: on } } },
    });
  }

  async function startWith(user: ReturnType<typeof renderPage>, file: File) {
    await user.upload(screen.getByLabelText("Tệp đề thi"), file);
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));
    expect(await screen.findByText("detail page")).toBeInTheDocument();
    return calls.find((call) => call.kind === "create");
  }

  it("is absent while the server has no assistant", async () => {
    aiOn({ ai: false });
    renderPage();
    await screen.findByText(scansRefused);
    expect(screen.queryByRole("switch", { name: card })).toBeNull();
  });

  it("is absent for a teacher without ai.use", async () => {
    aiOn();
    useAuthStore.setState({
      user: {
        ...teacherUser,
        permissions: teacherUser.permissions.filter((key) => key !== "ai.use"),
      },
    });
    renderPage();
    await screen.findByText(scansRefused);
    expect(screen.queryByRole("switch", { name: card })).toBeNull();
  });

  it("starts off, as the imports preference does by default, and creates as before", async () => {
    aiOn();
    const user = renderPage();
    const toggle = await screen.findByRole("switch", { name: card });
    expect(toggle).not.toBeChecked();
    expect(toggle).toHaveAccessibleDescription(
      "Đọc được bản quét, ảnh chụp đề in và bố cục lạ.",
    );
    expect(screen.getByText(scansRefused)).toBeInTheDocument();
    expect(screen.queryByText(privacyAi)).toBeNull();
    const create = await startWith(user, docx("de-thi.docx"));
    expect(create).not.toHaveProperty("readWithAi");
  });

  it("starts on from the preference, switches the accepts line and the privacy note, and sends readWithAi", async () => {
    aiOn();
    readImports(true);
    const user = renderPage();
    const toggle = await screen.findByRole("switch", { name: card });
    expect(toggle).toBeChecked();
    expect(screen.getByText(scansAccepted)).toBeInTheDocument();
    expect(screen.getByText(privacyAi)).toBeInTheDocument();
    expect(
      within(screen.getByRole("region", { name: "Các bước sẽ chạy" })).getByText(
        "Bản quét được đọc bằng AI",
      ),
    ).toBeInTheDocument();
    const create = await startWith(
      user,
      new File(["%PDF-1.4"], "scan.pdf", { type: "application/pdf" }),
    );
    expect(create).toMatchObject({ title: "scan", readWithAi: true });
  });

  it("turns on for this import alone, with the keyboard, and writes no preference", async () => {
    aiOn();
    const user = renderPage();
    const toggle = await screen.findByRole("switch", { name: card });
    toggle.focus();
    await user.keyboard(" ");
    expect(toggle).toBeChecked();
    expect(screen.getByText(scansAccepted)).toBeInTheDocument();
    expect(useAuthStore.getState().user?.preferences?.ai).toBeUndefined();
    const create = await startWith(user, docx("de-thi.docx"));
    expect(create).toMatchObject({ readWithAi: true });
  });

  it("stays drawn but cannot be turned on while the allowance is used up, and says why", async () => {
    aiOn({ usage: month({ used: 20000, left: 0 }) });
    readImports(true);
    const user = renderPage();
    await screen.findByText("Bạn đã dùng hết hạn mức. Hãy xin thêm trong Cài đặt.");
    const toggle = screen.getByRole("switch", { name: card });
    expect(toggle).toHaveAttribute("aria-disabled", "true");
    expect(toggle).not.toBeChecked();
    expect(toggle).toHaveAccessibleDescription(
      /Bạn đã dùng hết hạn mức\. Hãy xin thêm trong Cài đặt\.$/,
    );
    await user.click(toggle);
    expect(toggle).not.toBeChecked();
    expect(screen.getByText(scansRefused)).toBeInTheDocument();
    const create = await startWith(user, docx("de-thi.docx"));
    expect(create).not.toHaveProperty("readWithAi");
  });

  it("says why while the teacher is paused", async () => {
    aiOn({ usage: month({ paused: true }) });
    renderPage();
    expect(
      await screen.findByText("AI đang tạm dừng với tài khoản của bạn."),
    ).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: card })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("is not offered for pasted text, which is sent as before", async () => {
    aiOn();
    readImports(true);
    const user = renderPage("/teacher/imports/new?source=paste");
    await screen.findByRole("region", { name: "Đã đọc được" });
    expect(screen.queryByRole("switch", { name: card })).toBeNull();
    expect(screen.queryByText(privacyAi)).toBeNull();
    await user.click(screen.getByRole("button", { name: "Tải tệp lên" }));
    expect(await screen.findByRole("switch", { name: card })).toBeChecked();
  });

  it("is not offered when the server takes no PDF", async () => {
    aiOn();
    server.use(
      http.get(`${BASE}/teacher/imports/limits`, () =>
        contractJson("/teacher/imports/limits", "get", 200, {
          pasteMaxCharacters: 100000,
          maxBytes: 25 * 1024 * 1024,
          formats: ["docx"],
        }),
      ),
    );
    renderPage();
    await screen.findByText(/^Nhận tệp \.docx, tối đa 25 MB/);
    expect(screen.queryByRole("switch", { name: card })).toBeNull();
  });

  it("reads the assistant again when the create refuses readWithAi, and then creates without it", async () => {
    aiOn();
    readImports(true);
    let ai = true;
    let refused = false;
    server.use(
      http.get(`${BASE}/public/status`, () =>
        contractJson("/public/status", "get", 200, { maintenance: null, ai }),
      ),
      http.post(`${BASE}/teacher/imports`, async ({ request }) => {
        const body = (await request.json()) as {
          requestId: string;
          title: string;
          readWithAi?: boolean;
        };
        calls.push({ kind: "create", ...body });
        if (body.readWithAi === true && !refused) {
          refused = true;
          ai = false;
          return contractJson(
            "/teacher/imports",
            "post",
            422,
            errorBody("VALIDATION_FAILED", "Máy chủ chưa bật trợ lý AI.", {
              readWithAi: "unavailable",
            }),
          );
        }
        return contractJson(
          "/teacher/imports",
          "post",
          201,
          wordImport({
            title: body.title,
            status: "awaiting_sources",
            revision,
            sourceRevision: 0,
            sources: [],
          }),
        );
      }),
    );
    const user = renderPage();
    await screen.findByRole("switch", { name: card });
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("de-thi.docx"));
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));
    expect(await screen.findByText("Máy chủ chưa bật trợ lý AI.")).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole("switch", { name: card })).toBeNull(),
    );
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));
    expect(await screen.findByText("detail page")).toBeInTheDocument();
    const creates = calls.filter((call) => call.kind === "create");
    expect(creates).toHaveLength(2);
    expect(creates[1]).not.toHaveProperty("readWithAi");
    expect(creates[1]?.requestId).not.toBe(creates[0]?.requestId);
  });
});
