import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import i18n from "@/lib/i18n";
import { BASE_URL } from "@/lib/api/client";
import { saveDownloadedFile } from "@/lib/saveFile";
import { contentWidth } from "@tests/support/contentWidth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { REQUEST_ID, entry, renderAudit, serves } from "./support";

vi.mock("@/lib/saveFile", () => ({ saveDownloadedFile: vi.fn() }));

const ROWS = [entry({ id: 1 })];

beforeEach(() => {
  contentWidth(1320);
  vi.mocked(saveDownloadedFile).mockReset();
});

afterEach(async () => {
  await act(() => i18n.changeLanguage("vi"));
});

function exportAnswers(answer: (url: URL) => Response) {
  const asked: URL[] = [];
  server.use(
    http.get(`${BASE_URL}/admin/audit-log/export`, ({ request }) => {
      const url = new URL(request.url);
      asked.push(url);
      return answer(url);
    }),
  );
  return asked;
}

const file = () =>
  new Response("when,who\n", {
    headers: {
      "Content-Type": "text/csv",
      "Content-Disposition": 'attachment; filename="quizzivy-audit-7d.csv"',
    },
  });

async function openExport(search = "") {
  serves(() => ({ items: ROWS }));
  renderAudit(search);
  const user = userEvent.setup();
  await screen.findByRole("button", { name: /Đã tạo lớp/ });
  await user.click(screen.getByRole("button", { name: "Xuất CSV" }));
  const dialog = await screen.findByRole("dialog", { name: "Xuất nhật ký kiểm tra" });
  return { user, dialog };
}

describe("exporting the log", () => {
  it("offers the three periods and the two formats, on 7 days as CSV", async () => {
    const { dialog } = await openExport();

    const periods = within(dialog).getByRole("group", { name: "Khoảng thời gian" });
    expect(within(periods).getByRole("button", { name: "7 ngày qua" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(within(periods).getByRole("button", { name: "30 ngày qua" })).toBeVisible();
    expect(within(periods).getByRole("button", { name: "Năm nay" })).toBeVisible();
    const formats = within(dialog).getByRole("group", { name: "Định dạng" });
    expect(within(formats).getByRole("button", { name: "CSV" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(within(formats).getByRole("button", { name: "JSON" })).toBeVisible();
  });

  it("opens on the period nearest the range in force", async () => {
    const { dialog } = await openExport("?range=30d");
    expect(within(dialog).getByRole("button", { name: "30 ngày qua" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it.each([
    ["2026-10-14", "2026-10-15", "7 ngày qua"],
    ["2026-10-09", "2026-10-15", "7 ngày qua"],
    ["2026-10-08", "2026-10-15", "30 ngày qua"],
    ["2026-09-16", "2026-10-15", "30 ngày qua"],
    ["2026-09-15", "2026-10-15", "Năm nay"],
  ])(
    "opens a custom range from %s to %s on the shortest period that covers it: %s",
    async (from, to, period) => {
      const { dialog } = await openExport(`?range=custom&from=${from}&to=${to}`);
      expect(within(dialog).getByRole("button", { name: period })).toHaveAttribute(
        "aria-pressed",
        "true",
      );
    },
  );

  it("opens on this year for every range that reaches further back", async () => {
    const { dialog } = await openExport("?range=all");
    expect(within(dialog).getByRole("button", { name: "Năm nay" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("has no switch for the current filters while no category and no search is set", async () => {
    const { dialog } = await openExport("?range=30d");
    expect(within(dialog).queryByRole("switch")).toBeNull();
  });

  it("downloads the period and format chosen, saves the file as the server named it and closes", async () => {
    const asked = exportAnswers(file);
    const { user, dialog } = await openExport();

    await user.click(within(dialog).getByRole("button", { name: "30 ngày qua" }));
    await user.click(within(dialog).getByRole("button", { name: "JSON" }));
    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));

    await waitFor(() => expect(saveDownloadedFile).toHaveBeenCalledOnce());
    expect(asked).toHaveLength(1);
    expect(Object.fromEntries(asked[0]!.searchParams)).toEqual({
      period: "30d",
      format: "json",
    });
    const [saved, fallback] = vi.mocked(saveDownloadedFile).mock.calls[0]!;
    expect(saved.filename).toBe("quizzivy-audit-7d.csv");
    expect(fallback).toBe("audit-log.json");
    expect(await screen.findByText("Đã tải xuống nhật ký kiểm tra.")).toBeVisible();
    expect(screen.queryByRole("dialog", { name: "Xuất nhật ký kiểm tra" })).toBeNull();
  });

  it("downloads once however fast the button is pressed", async () => {
    const asked = exportAnswers(file);
    const { user, dialog } = await openExport();

    await user.dblClick(within(dialog).getByRole("button", { name: "Xuất tệp" }));

    await waitFor(() => expect(saveDownloadedFile).toHaveBeenCalledOnce());
    expect(asked).toHaveLength(1);
  });

  it("stays open on Escape while the file is on its way, and closes when it has arrived", async () => {
    let release: () => void = () => undefined;
    const asked: URL[] = [];
    server.use(
      http.get(`${BASE_URL}/admin/audit-log/export`, ({ request }) => {
        asked.push(new URL(request.url));
        return new Promise<Response>((resolve) => {
          release = () => resolve(file());
        });
      }),
    );
    const { user, dialog } = await openExport();

    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));
    await waitFor(() => expect(asked).toHaveLength(1));
    await user.keyboard("{Escape}");

    expect(screen.getByRole("dialog", { name: "Xuất nhật ký kiểm tra" })).toBeVisible();
    release();
    await waitFor(() => expect(saveDownloadedFile).toHaveBeenCalledOnce());
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Xuất nhật ký kiểm tra" }),
      ).toBeNull(),
    );
  });

  it("can be pressed again after a refusal", async () => {
    let answers = 0;
    exportAnswers(() => {
      answers += 1;
      return answers === 1
        ? contractJson("/admin/audit-log/export", "get", 500, {
            error: { code: "INTERNAL", message: "Lỗi máy chủ.", requestId: REQUEST_ID },
          })
        : file();
    });
    const { user, dialog } = await openExport();

    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));
    expect(await within(dialog).findByRole("alert")).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));

    await waitFor(() => expect(saveDownloadedFile).toHaveBeenCalledOnce());
  });

  it("narrows by the category and search in force, which the switch starts on for", async () => {
    const asked = exportAnswers(file);
    const { user, dialog } = await openExport("?category=tests&q=nam");

    expect(
      within(dialog).getByRole("switch", { name: "Chỉ bộ lọc hiện tại" }),
    ).toBeChecked();
    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));

    await waitFor(() => expect(asked).toHaveLength(1));
    expect(Object.fromEntries(asked[0]!.searchParams)).toEqual({
      period: "7d",
      format: "csv",
      category: "tests",
      q: "nam",
    });
  });

  it("sends neither when the switch is turned off, and never sends the range or a custom day", async () => {
    const asked = exportAnswers(file);
    const { user, dialog } = await openExport(
      "?category=tests&q=nam&range=custom&from=2026-09-18&to=2026-09-24",
    );

    await user.click(
      within(dialog).getByRole("switch", { name: "Chỉ bộ lọc hiện tại" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));

    await waitFor(() => expect(asked).toHaveLength(1));
    const sent = asked[0]!.searchParams;
    expect(Object.fromEntries(sent)).toEqual({ period: "7d", format: "csv" });
    expect(sent.has("range")).toBe(false);
    expect(sent.has("from")).toBe(false);
  });

  it("asks for a shorter period when the server refuses one as too large, and lets it be tried again", async () => {
    let refuse = true;
    const asked = exportAnswers(() =>
      refuse
        ? contractJson("/admin/audit-log/export", "get", 422, {
            error: {
              code: "EXPORT_TOO_LARGE",
              message: "too large",
              requestId: REQUEST_ID,
            },
          })
        : file(),
    );
    const { user, dialog } = await openExport("?range=all");

    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Khoảng này có hơn 100.000 mục. Hãy chọn khoảng ngắn hơn.",
    );
    expect(saveDownloadedFile).not.toHaveBeenCalled();

    refuse = false;
    await user.click(within(dialog).getByRole("button", { name: "30 ngày qua" }));
    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));

    await waitFor(() => expect(saveDownloadedFile).toHaveBeenCalledOnce());
    expect(asked.map((url) => url.searchParams.get("period"))).toEqual(["year", "30d"]);
  });

  it("says it failed, in its own words, for any other refusal", async () => {
    exportAnswers(() =>
      contractJson("/admin/audit-log/export", "get", 429, {
        error: { code: "RATE_LIMITED", message: "slow down", requestId: REQUEST_ID },
      }),
    );
    const { user, dialog } = await openExport();

    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));

    expect(await within(dialog).findByRole("alert")).toBeVisible();
    expect(saveDownloadedFile).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Xuất nhật ký kiểm tra" })).toBeVisible();
  });

  it("forgets a refusal when the dialog is closed and opened again", async () => {
    exportAnswers(() =>
      contractJson("/admin/audit-log/export", "get", 422, {
        error: {
          code: "EXPORT_TOO_LARGE",
          message: "too large",
          requestId: REQUEST_ID,
        },
      }),
    );
    const { user, dialog } = await openExport();

    await user.click(within(dialog).getByRole("button", { name: "Xuất tệp" }));
    await within(dialog).findByRole("alert");
    await user.keyboard("{Escape}");
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Xuất nhật ký kiểm tra" }),
      ).toBeNull(),
    );
    await user.click(screen.getByRole("button", { name: "Xuất CSV" }));

    const again = await screen.findByRole("dialog", { name: "Xuất nhật ký kiểm tra" });
    expect(within(again).queryByRole("alert")).toBeNull();
  });
});
