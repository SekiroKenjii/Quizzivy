import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportsGate from "@/features/imports/pages/ImportsGate";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { BASE } from "./fixtures";
import "@/lib/i18n";

function serveCapabilities(answer: () => Response) {
  server.use(http.get(`${BASE}/admin/imports/capabilities`, answer));
}

function renderGate() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      {
        path: "/admin/imports",
        element: <ImportsGate />,
        children: [{ index: true, element: <p>history</p> }],
      },
      { path: "/admin/tests", element: <p>tests</p> },
    ],
    { initialEntries: ["/admin/imports"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the Word import routes", () => {
  it("explain that import is not enabled instead of loading a screen that would fail", async () => {
    serveCapabilities(() =>
      contractJson("/admin/imports/capabilities", "get", 200, {
        intakeEnabled: false,
        processingEnabled: false,
        retention: { afterCommitDays: 30, afterCancelDays: 7, idleDays: 60 },
      }),
    );
    const user = renderGate();

    expect(
      await screen.findByText("Máy chủ này chưa bật nhập đề từ Word/PDF."),
    ).toBeInTheDocument();
    expect(screen.queryByText("history")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Về danh sách đề thi" }));
    expect(await screen.findByText("tests")).toBeInTheDocument();
  });

  it("open the screen when import storage is configured, even with processing off", async () => {
    serveCapabilities(() =>
      contractJson("/admin/imports/capabilities", "get", 200, {
        intakeEnabled: true,
        processingEnabled: false,
        retention: { afterCommitDays: 30, afterCancelDays: 7, idleDays: 60 },
      }),
    );
    renderGate();

    expect(await screen.findByText("history")).toBeInTheDocument();
  });

  it("offer a retry when the check itself fails", async () => {
    let failing = true;
    serveCapabilities(() =>
      failing
        ? new Response(null, { status: 503 })
        : contractJson("/admin/imports/capabilities", "get", 200, {
            intakeEnabled: true,
            processingEnabled: true,
            retention: { afterCommitDays: 30, afterCancelDays: 7, idleDays: 60 },
          }),
    );
    const user = renderGate();

    expect(
      await screen.findByText(
        "Không kiểm tra được máy chủ có hỗ trợ nhập đề từ Word/PDF hay không.",
      ),
    ).toBeInTheDocument();
    failing = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByText("history")).toBeInTheDocument();
  });
});
