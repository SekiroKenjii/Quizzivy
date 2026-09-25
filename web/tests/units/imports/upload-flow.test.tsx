import { beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import NewImportPage from "@/features/imports/pages/NewImportPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
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
  | { kind: "create"; requestId: string; title: string }
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
    http.get(`${BASE}/admin/imports/limits`, () =>
      contractJson("/admin/imports/limits", "get", 200, {
        maxBytes: 25 * 1024 * 1024,
        formats: ["docx"],
      }),
    ),
    http.post(`${BASE}/admin/imports`, async ({ request }) => {
      const body = (await request.json()) as { requestId: string; title: string };
      calls.push({ kind: "create", ...body });
      return contractJson(
        "/admin/imports",
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
    http.post(`${BASE}/admin/imports/:id/sources`, ({ request }) => {
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
          "/admin/imports/{id}/sources",
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
      return contractJson("/admin/imports/{id}/sources", "post", 201, {
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
    http.post(`${BASE}/admin/imports/:id/process`, async ({ request }) => {
      const body = (await request.json()) as {
        requestId: string;
        expectedRevision: number;
      };
      calls.push({ kind: "process", ...body });
      return contractJson(
        "/admin/imports/{id}/process",
        "post",
        202,
        wordImport({ status: "queued", revision: revision + 1 }),
      );
    }),
  );
});

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/imports/new", element: <NewImportPage /> },
      { path: "/admin/imports/:id", element: <p>detail page</p> },
    ],
    { initialEntries: ["/admin/imports/new"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("starting a Word import", () => {
  it("shows the server's limits before a file is chosen", async () => {
    renderPage();
    expect(
      await screen.findByText(/Nhận tệp \.docx, tối đa 25\.0 MB/),
    ).toBeInTheDocument();
  });

  it("stops offering the upload once the server says processing is switched off", async () => {
    let processing = true;
    server.use(
      http.get(`${BASE}/admin/imports/capabilities`, () =>
        contractJson("/admin/imports/capabilities", "get", 200, {
          intakeEnabled: true,
          processingEnabled: processing,
        }),
      ),
      http.post(`${BASE}/admin/imports/:id/process`, () => {
        processing = false;
        return contractJson(
          "/admin/imports/{id}/process",
          "post",
          503,
          errorBody(
            "IMPORT_PROCESSING_UNAVAILABLE",
            "Máy chủ này chưa bật xử lý tài liệu Word nên chưa thể xử lý lượt nhập.",
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
      http.get(`${BASE}/admin/imports/capabilities`, () =>
        contractJson("/admin/imports/capabilities", "get", 200, {
          intakeEnabled: true,
          processingEnabled: processing,
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
    expect(screen.getByRole("link", { name: "Về lịch sử nhập đề" })).toHaveAttribute(
      "href",
      "/admin/imports",
    );
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
      [{ path: "/admin/imports/new", element: <NewImportPage /> }],
      { initialEntries: ["/admin/imports/new"] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    await screen.findByText(/Nhận tệp/);
    await user.upload(screen.getByLabelText("Tệp đề thi"), docx("de-thi.pdf"));

    expect(await screen.findByRole("alert")).toHaveTextContent(/de-thi\.pdf/);
    expect(screen.getByRole("button", { name: "Bắt đầu xử lý" })).toBeDisabled();
    await waitFor(() => expect(calls).toHaveLength(0));
  });

  it("stops quietly when the teacher leaves before the upload finishes", async () => {
    const gate = deferred<void>();
    server.use(
      http.post(`${BASE}/admin/imports`, async ({ request }) => {
        const body = (await request.json()) as { requestId: string; title: string };
        calls.push({ kind: "create", ...body });
        await gate.promise;
        return contractJson(
          "/admin/imports",
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
        { path: "/admin/imports/new", element: <NewImportPage /> },
        { path: "/admin/imports/:id", element: <p>detail page</p> },
        { path: "/admin/tests", element: <p>tests page</p> },
      ],
      { initialEntries: ["/admin/imports/new"] },
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

    await router.navigate("/admin/tests");
    expect(await screen.findByText("tests page")).toBeInTheDocument();
    gate.resolve();
    await new Promise((settle) => setTimeout(settle, 100));
    expect(router.state.location.pathname).toBe("/admin/tests");
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

  it("titles the card for the whole step and does not repeat the required tag", async () => {
    renderPage();
    expect(await screen.findByText("Chọn tệp để nhập")).toBeInTheDocument();
    expect(screen.getAllByText("Bắt buộc")).toHaveLength(1);
  });
});
