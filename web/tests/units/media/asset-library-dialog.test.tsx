import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import {
  AssetLibraryDialog,
  type LibraryKind,
} from "@/features/media/components/AssetLibraryDialog";
import type { LibraryAsset } from "@/features/media/api";
import { contractJson } from "@tests/support/contractResponse";
import {
  installIntersectionObserver,
  scrollAllIntoView,
} from "@tests/support/intersection";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

function asset(n: number, kind: "audio" | "image"): LibraryAsset {
  const id = `018f0000-0000-7000-8000-${String(n).padStart(12, "0")}`;
  const name = kind === "audio" ? `bai-nghe-${n}.mp3` : `anh-${n}.png`;
  return {
    id,
    kind,
    url: `https://example.test/${name}`,
    mimeType: kind === "audio" ? "audio/mpeg" : "image/png",
    bytes: 2_400_000,
    durationMs: kind === "audio" ? 110_000 : null,
    originalFilename: name,
    createdAt: "2026-01-01T00:00:00Z",
    displayName: name,
    defaultMaxPlays: null,
    width: kind === "image" ? 1200 : null,
    height: kind === "image" ? 800 : null,
    questionCount: n % 2,
  };
}

const LIBRARY = [
  ...Array.from({ length: 30 }, (_, i) => asset(i + 1, "audio")),
  ...Array.from({ length: 4 }, (_, i) => asset(i + 101, "image")),
];

let requests: URLSearchParams[] = [];
let failing = false;

beforeEach(() => {
  installIntersectionObserver();
  requests = [];
  failing = false;
  server.use(
    http.get(`${BASE}/teacher/media`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      requests.push(query);
      if (failing) return new Response(null, { status: 500 });
      const q = query.get("q") ?? "";
      const kind = query.get("kind");
      const page = Number(query.get("page") ?? "1");
      const limit = Number(query.get("limit") ?? "24");
      const matching = LIBRARY.filter((item) => item.displayName.includes(q));
      const shown = matching.filter((item) => kind === null || item.kind === kind);
      return contractJson("/teacher/media", "get", 200, {
        items: shown.slice((page - 1) * limit, page * limit),
        page,
        pageSize: limit,
        total: shown.length,
        totalBytes: 0,
        facets: {
          all: matching.length,
          audio: matching.filter((item) => item.kind === "audio").length,
          image: matching.filter((item) => item.kind === "image").length,
          unused: 0,
        },
        usage: { audioBytes: 0, imageBytes: 0, quotaBytes: 5_368_709_120 },
      });
    }),
  );
});

function renderDialog() {
  const onPick = vi.fn();
  const onUploadNew = vi.fn<(kind: LibraryKind) => void>();
  function Harness() {
    const [open, setOpen] = useState(false);
    return (
      <>
        <button type="button" onClick={() => setOpen(true)}>
          Mở
        </button>
        <AssetLibraryDialog
          open={open}
          onOpenChange={setOpen}
          onPick={onPick}
          onUploadNew={onUploadNew}
        />
      </>
    );
  }
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <Harness />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), onPick, onUploadNew };
}

async function open(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Mở" }));
  return screen.findByRole("dialog", { name: "Chọn từ Media" });
}

describe("Choose from Media", () => {
  it("opens on All with counts, a page of files and Attach disabled", async () => {
    const { user } = renderDialog();
    const dialog = await open(user);

    expect(within(dialog).getByText("Các tệp bạn đã tải lên.")).toBeInTheDocument();
    expect(
      await within(dialog).findByRole("button", { name: /bai-nghe-1\.mp3/ }),
    ).toBeVisible();
    const kinds = within(dialog).getByRole("group", { name: "Loại tệp" });
    expect(within(kinds).getByRole("button", { name: /Tất cả/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(kinds).toHaveTextContent(/Tất cả\s*34/);
    expect(kinds).toHaveTextContent(/Hình ảnh\s*4/);
    expect(kinds).toHaveTextContent(/Âm thanh\s*30/);
    expect(
      within(dialog)
        .getAllByRole("listitem")
        .filter((li) => li.querySelector("button")),
    ).toHaveLength(24);
    expect(within(dialog).getByRole("button", { name: "Đính kèm" })).toBeDisabled();
    expect(requests[0]?.get("kind")).toBeNull();
  });

  it("shows each file's meta and its use", async () => {
    const { user } = renderDialog();
    const dialog = await open(user);

    const first = await within(dialog).findByRole("button", {
      name: /bai-nghe-1\.mp3/,
    });
    expect(first).toHaveTextContent("1:50 · 2.3 MB");
    expect(first).toHaveTextContent("Dùng trong 1 câu hỏi");
    expect(
      within(dialog).getByRole("button", { name: /bai-nghe-2\.mp3/ }),
    ).toHaveTextContent("Chưa dùng");
  });

  it("filters by kind and the counts follow the search", async () => {
    const { user } = renderDialog();
    const dialog = await open(user);
    await within(dialog).findByRole("button", { name: /bai-nghe-1\.mp3/ });

    await user.click(within(dialog).getByRole("button", { name: /Hình ảnh/ }));
    const image = await within(dialog).findByRole("button", { name: /anh-101\.png/ });
    expect(image).toHaveTextContent("1200 × 800 · 2.3 MB");
    expect(
      within(dialog).queryByRole("button", { name: /bai-nghe-1\.mp3/ }),
    ).toBeNull();
    expect(requests.at(-1)?.get("kind")).toBe("image");

    await user.click(within(dialog).getByRole("button", { name: /Tất cả/ }));
    await user.type(within(dialog).getByRole("searchbox"), "nghe-1");
    await waitFor(() => expect(requests.at(-1)?.get("q")).toBe("nghe-1"));
    const kinds = within(dialog).getByRole("group", { name: "Loại tệp" });
    await waitFor(() => expect(kinds).toHaveTextContent(/Hình ảnh\s*0/));
    expect(kinds).toHaveTextContent(/Tất cả\s*11/);
  });

  it("says when no file matches the search", async () => {
    const { user } = renderDialog();
    const dialog = await open(user);
    await within(dialog).findByRole("button", { name: /bai-nghe-1\.mp3/ });

    await user.type(within(dialog).getByRole("searchbox"), "khong-co");

    expect(
      await within(dialog).findByText("Không có tệp nào khớp."),
    ).toBeInTheDocument();
  });

  it("selects one file at a time and attaches it", async () => {
    const { user, onPick } = renderDialog();
    const dialog = await open(user);

    const first = await within(dialog).findByRole("button", {
      name: /bai-nghe-1\.mp3/,
    });
    const second = within(dialog).getByRole("button", { name: /bai-nghe-2\.mp3/ });
    await user.click(first);
    await user.click(second);

    expect(first).toHaveAttribute("aria-pressed", "false");
    expect(second).toHaveAttribute("aria-pressed", "true");
    await user.click(within(dialog).getByRole("button", { name: "Đính kèm" }));

    expect(onPick).toHaveBeenCalledWith(
      expect.objectContaining({ displayName: "bai-nghe-2.mp3" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: "Mở" })).toHaveFocus();
  });

  it("picks a file on a double-click", async () => {
    const { user, onPick } = renderDialog();
    const dialog = await open(user);

    await user.dblClick(
      await within(dialog).findByRole("button", { name: /bai-nghe-3\.mp3/ }),
    );

    expect(onPick).toHaveBeenCalledOnce();
    expect(onPick).toHaveBeenCalledWith(
      expect.objectContaining({ displayName: "bai-nghe-3.mp3" }),
    );
  });

  it("loads the next page when the end of the grid comes into view", async () => {
    const { user } = renderDialog();
    const dialog = await open(user);
    await within(dialog).findByRole("button", { name: /bai-nghe-24\.mp3/ });
    expect(
      within(dialog).queryByRole("button", { name: /bai-nghe-25\.mp3/ }),
    ).toBeNull();

    act(() => scrollAllIntoView());

    expect(
      await within(dialog).findByRole("button", { name: /anh-104\.png/ }),
    ).toBeVisible();
    expect(requests.at(-1)?.get("page")).toBe("2");
  });

  it("hands Upload new the selected kind and closes", async () => {
    const { user, onUploadNew } = renderDialog();
    const dialog = await open(user);
    await within(dialog).findByRole("button", { name: /bai-nghe-1\.mp3/ });

    await user.click(within(dialog).getByRole("button", { name: /Âm thanh/ }));
    await user.click(within(dialog).getByRole("button", { name: "Tải tệp mới" }));

    expect(onUploadNew).toHaveBeenCalledWith("audio");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("offers a retry when the library cannot be read", async () => {
    failing = true;
    const { user } = renderDialog();
    const dialog = await open(user);

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Không tải được thư viện tệp.",
    );
    failing = false;
    await user.click(within(dialog).getByRole("button", { name: "Thử lại" }));

    expect(
      await within(dialog).findByRole("button", { name: /bai-nghe-1\.mp3/ }),
    ).toBeVisible();
  });

  it("says when the library is empty", async () => {
    server.use(
      http.get(`${BASE}/teacher/media`, () =>
        contractJson("/teacher/media", "get", 200, {
          items: [],
          page: 1,
          pageSize: 24,
          total: 0,
          totalBytes: 0,
          facets: { all: 0, audio: 0, image: 0, unused: 0 },
          usage: { audioBytes: 0, imageBytes: 0, quotaBytes: 5_368_709_120 },
        }),
      ),
    );
    const { user } = renderDialog();
    const dialog = await open(user);

    expect(
      await within(dialog).findByText("Chưa có tệp nào. Tải tệp mới lên để bắt đầu."),
    ).toBeInTheDocument();
  });
});
