import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import MediaPage from "@/features/media/pages/teacher/MediaPage";
import type { LibraryAsset } from "@/features/media/api";
import { MAX_IMAGE_BYTES } from "@/features/media/limits";
import { Toaster } from "@/components/ui/sonner";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const QUOTA = 5 * 1024 * 1024 * 1024;

function asset(overrides: Partial<LibraryAsset> & { id: string }): LibraryAsset {
  return {
    kind: "audio",
    url: `https://example.test/${overrides.id}.mp3`,
    mimeType: "audio/mpeg",
    bytes: 2_516_582,
    durationMs: 156_000,
    originalFilename: "file.mp3",
    createdAt: "2026-10-01T00:00:00Z",
    displayName: "file.mp3",
    defaultMaxPlays: null,
    width: null,
    height: null,
    questionCount: 0,
    usageCount: 0,
    usedIn: [],
    ...overrides,
  };
}

const AIRPORT = asset({
  id: "018f0000-0000-7000-8000-0000000000a1",
  displayName: "Unit 4 · Airport announcements.mp3",
  questionCount: 3,
  defaultMaxPlays: 1,
});
const PODCAST = asset({
  id: "018f0000-0000-7000-8000-0000000000a2",
  displayName: "Podcast · city parks.mp3",
  durationMs: 365_000,
  defaultMaxPlays: 0,
});
const MAP = asset({
  id: "018f0000-0000-7000-8000-0000000000a3",
  kind: "image",
  url: "https://example.test/map.png",
  mimeType: "image/png",
  bytes: 245_760,
  durationMs: null,
  displayName: "Directions map.png",
  width: 1200,
  height: 800,
  questionCount: 2,
});

let lists: URLSearchParams[] = [];
let items: LibraryAsset[] = [AIRPORT, PODCAST, MAP];
let failList = false;

function libraryBody(query: URLSearchParams) {
  const searched = query.get("q") === null ? items : items.slice(0, 1);
  const shown = searched.filter(
    (row) =>
      (query.get("kind") === null || row.kind === query.get("kind")) &&
      (query.get("unused") !== "true" || row.questionCount === 0),
  );
  return {
    items: shown,
    page: 1,
    pageSize: 24,
    total: shown.length,
    totalBytes: shown.reduce((sum, row) => sum + row.bytes, 0),
    facets: {
      all: searched.length,
      audio: searched.filter((row) => row.kind === "audio").length,
      image: searched.filter((row) => row.kind === "image").length,
      unused: searched.filter((row) => row.questionCount === 0).length,
    },
    usage: { audioBytes: 1_127_428_915, imageBytes: 214_748_365, quotaBytes: QUOTA },
  };
}

/**
 * durationUnreadable makes every audio element fail to load, as jsdom reads no
 * media: the pre-check's duration read then settles as unreadable, which lets
 * the upload go, as it does in a browser.
 */
function durationUnreadable() {
  Object.defineProperty(HTMLMediaElement.prototype, "src", {
    configurable: true,
    set(this: HTMLMediaElement) {
      setTimeout(() => this.onerror?.(new Event("error")), 0);
    },
  });
}

beforeEach(() => {
  lists = [];
  items = [AIRPORT, PODCAST, MAP];
  failList = false;
  durationUnreadable();
  server.use(
    http.get(`${BASE}/teacher/media`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      lists.push(query);
      if (failList) return new Response(null, { status: 500 });
      return contractJson("/teacher/media", "get", 200, libraryBody(query));
    }),
  );
});

function renderPage(path = "/teacher/media", applyAccept = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/media", element: <MediaPage /> },
      { path: "/teacher/tests/:id", element: <p>test</p> },
    ],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup({ applyAccept });
}

const card = (name: string) =>
  screen.getByRole("heading", { level: 2, name }).closest("article") as HTMLElement;

async function openMenu(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(
    await screen.findByRole("button", { name: `Thao tác với tệp ${name}` }),
  );
}

describe("the Media page", () => {
  it("heads the page, states the storage against the quota and draws each card", async () => {
    renderPage();

    expect(
      screen.getByRole("heading", { level: 1, name: "Thư viện media" }),
    ).toBeVisible();
    expect(screen.getByText("Âm thanh và hình ảnh dùng trong câu hỏi.")).toBeVisible();
    expect(
      await screen.findByText("1.25 GB trên 5 GB · âm thanh 1.05 GB · hình ảnh 0.2 GB"),
    ).toBeVisible();

    const airport = card("Unit 4 · Airport announcements.mp3");
    expect(within(airport).getByText("2:36 · 2.4 MB")).toBeVisible();
    expect(within(airport).getByText("Dùng trong 3 câu hỏi")).toBeVisible();
    expect(within(airport).getByText("1 lượt nghe")).toBeVisible();

    const podcast = card("Podcast · city parks.mp3");
    expect(within(podcast).getByText("Chưa dùng")).toHaveClass("text-warning-ink");
    expect(within(podcast).getByText("Không giới hạn")).toBeVisible();

    const map = card("Directions map.png");
    expect(within(map).getByText("1200 × 800 · 240 KB")).toBeVisible();
    expect(map.querySelector("img")).toHaveAttribute(
      "src",
      "https://example.test/map.png",
    );
    expect(within(map).queryByText(/lượt nghe|Không giới hạn/)).toBeNull();
  });

  it("counts each tab from the facets, and the counts follow the search", async () => {
    const user = renderPage();
    const tabs = await screen.findByRole("group", { name: "Lọc tệp" });
    await waitFor(() =>
      expect(within(tabs).getByRole("button", { name: /Tất cả/ })).toHaveTextContent(
        "3",
      ),
    );
    expect(within(tabs).getByRole("button", { name: /Âm thanh/ })).toHaveTextContent(
      "2",
    );
    expect(within(tabs).getByRole("button", { name: /Hình ảnh/ })).toHaveTextContent(
      "1",
    );
    expect(within(tabs).getByRole("button", { name: /Chưa dùng/ })).toHaveTextContent(
      "1",
    );

    await user.type(screen.getByRole("searchbox", { name: "Tìm tệp" }), "san bay");

    await waitFor(() => expect(lists.at(-1)?.get("q")).toBe("san bay"));
    await waitFor(() =>
      expect(within(tabs).getByRole("button", { name: /Tất cả/ })).toHaveTextContent(
        "1",
      ),
    );
    expect(within(tabs).getByRole("button", { name: /Hình ảnh/ })).toHaveTextContent(
      "0",
    );
  });

  it("asks for a kind, or for the unused files, when a tab is chosen", async () => {
    const user = renderPage();
    const tabs = await screen.findByRole("group", { name: "Lọc tệp" });

    await user.click(within(tabs).getByRole("button", { name: /Hình ảnh/ }));
    await waitFor(() => expect(lists.at(-1)?.get("kind")).toBe("image"));

    await user.click(within(tabs).getByRole("button", { name: /Chưa dùng/ }));
    await waitFor(() => expect(lists.at(-1)?.get("unused")).toBe("true"));
    expect(lists.at(-1)?.get("kind")).toBeNull();
    expect(
      within(tabs).getByRole("button", { name: /Chưa dùng/, pressed: true }),
    ).toBeVisible();
  });

  it("offers an upload when the library is empty", async () => {
    items = [];
    renderPage();

    expect(await screen.findByText("Chưa có media")).toBeVisible();
    expect(screen.getAllByRole("button", { name: "Tải lên" })).toHaveLength(2);
  });

  it("says nothing matches, and clears the filters, when a search finds nothing", async () => {
    items = [];
    const user = renderPage("/teacher/media?tab=audio");

    expect(await screen.findByText("Không có tệp nào khớp.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Xoá bộ lọc" }));

    expect(await screen.findByText("Chưa có media")).toBeVisible();
  });

  it("shows the error with a retry when the library cannot be read", async () => {
    failList = true;
    const user = renderPage();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không tải được thư viện tệp.",
    );
    failList = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    expect(await screen.findByText("Directions map.png")).toBeVisible();
  });

  it("downloads through the signed URL, in a new tab", async () => {
    const user = renderPage();
    await openMenu(user, "Directions map.png");

    const download = await screen.findByRole("menuitem", { name: "Tải xuống" });
    expect(download).toHaveAttribute("href", "https://example.test/map.png");
    expect(download).toHaveAttribute("target", "_blank");
    expect(download).toHaveAttribute("rel", "noopener noreferrer");
  });
});

describe("a file's menu", () => {
  it("renames the file through updateMedia", async () => {
    let body: unknown = null;
    server.use(
      http.patch(`${BASE}/teacher/media/:id`, async ({ request, params }) => {
        body = await request.json();
        return contractJson("/teacher/media/{id}", "patch", 200, {
          ...PODCAST,
          id: params["id"],
          displayName: "Công viên thành phố.mp3",
        });
      }),
    );
    const user = renderPage();
    await openMenu(user, "Podcast · city parks.mp3");
    await user.click(await screen.findByRole("menuitem", { name: "Đổi tên" }));

    const dialog = await screen.findByRole("dialog", { name: "Đổi tên tệp" });
    const name = within(dialog).getByRole("textbox", { name: "Tên" });
    expect(name).toHaveValue("Podcast · city parks.mp3");
    await user.clear(name);
    await user.type(name, "  Công viên thành phố.mp3 ");
    await user.click(within(dialog).getByRole("button", { name: "Đổi tên" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(body).toEqual({ displayName: "Công viên thành phố.mp3" });
  });

  it("replaces the file with DG-09's copy, then re-reads the library", async () => {
    let replaced = 0;
    server.use(
      http.post(`${BASE}/teacher/media/:id/replace`, ({ params }) => {
        replaced += 1;
        expect(params["id"]).toBe(AIRPORT.id);
        return contractJson("/teacher/media/{id}/replace", "post", 201, {
          asset: { ...AIRPORT, id: "018f0000-0000-7000-8000-0000000000b1" },
          repointed: { questions: 3, groups: 0 },
          left: { questions: 0, groups: 0 },
        });
      }),
    );
    const user = renderPage();
    await openMenu(user, "Unit 4 · Airport announcements.mp3");
    await user.click(await screen.findByRole("menuitem", { name: "Thay tệp" }));

    const dialog = await screen.findByRole("dialog", { name: "Thay tệp" });
    expect(dialog).toHaveTextContent(
      "Câu hỏi trong ngân hàng và bản nháp sẽ dùng tệp mới. Các phiên bản đã xuất bản vẫn giữ tệp cũ.",
    );
    expect(dialog).toHaveTextContent("MP3 hoặc M4A · tối đa 50 MB và 5 phút");
    const before = lists.length;
    const input = dialog.querySelector<HTMLInputElement>("input[type=file]")!;
    expect(input).toHaveAttribute("accept", ".mp3,.m4a,audio/mpeg,audio/mp4");
    await user.upload(input, new File([new Uint8Array(16)], "moi.m4a"));
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(await screen.findByText("Đã thay tệp")).toBeVisible();
    expect(replaced).toBe(1);
    await waitFor(() => expect(lists.length).toBeGreaterThan(before));
  });

  it("refuses, before sending, a replacement image over 10 MB", async () => {
    let replaced = 0;
    server.use(
      http.post(`${BASE}/teacher/media/:id/replace`, () => {
        replaced += 1;
        return new Response(null, { status: 500 });
      }),
    );
    const user = renderPage();
    await openMenu(user, "Directions map.png");
    await user.click(await screen.findByRole("menuitem", { name: "Thay tệp" }));

    const dialog = await screen.findByRole("dialog", { name: "Thay tệp" });
    expect(dialog).toHaveTextContent("JPG, PNG hoặc WebP · tối đa 10 MB");
    const big = new File([new Uint8Array(16)], "ban-do-moi.png");
    Object.defineProperty(big, "size", { value: MAX_IMAGE_BYTES + 1 });
    await user.upload(dialog.querySelector<HTMLInputElement>("input[type=file]")!, big);
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(await within(dialog).findByText(/vượt quá 10 MB/)).toBeVisible();
    expect(replaced).toBe(0);
  });

  it("re-reads the library after a failed replacement, which may have committed", async () => {
    server.use(
      http.post(`${BASE}/teacher/media/:id/replace`, () =>
        contractJson("/teacher/media/{id}/replace", "post", 409, {
          error: {
            code: "MEDIA_QUOTA_EXCEEDED",
            message: "Thư viện đã đầy.",
            requestId: "018f0000-0000-7000-8000-0000000000f1",
          },
        }),
      ),
    );
    const user = renderPage();
    await openMenu(user, "Unit 4 · Airport announcements.mp3");
    await user.click(await screen.findByRole("menuitem", { name: "Thay tệp" }));
    const dialog = await screen.findByRole("dialog", { name: "Thay tệp" });
    const before = lists.length;
    await user.upload(
      dialog.querySelector<HTMLInputElement>("input[type=file]")!,
      new File([new Uint8Array(16)], "moi.mp3"),
    );
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Thư viện đã đầy.",
    );
    await waitFor(() => expect(lists.length).toBeGreaterThan(before));
  });

  it("asks before deleting an unused file, then deletes it", async () => {
    let deleted: unknown = null;
    server.use(
      http.delete(`${BASE}/teacher/media/:id`, ({ params }) => {
        deleted = params["id"];
        return new Response(null, { status: 204 });
      }),
    );
    const user = renderPage();
    await openMenu(user, "Podcast · city parks.mp3");
    await user.click(await screen.findByRole("menuitem", { name: "Xoá" }));

    const dialog = await screen.findByRole("dialog", {
      name: "Xoá Podcast · city parks.mp3?",
    });
    expect(dialog).toHaveTextContent("Chưa câu hỏi nào dùng tệp này.");
    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));

    expect(await screen.findByText("Đã xoá tệp")).toBeVisible();
    expect(deleted).toBe(PODCAST.id);
  });

  it("warns that the questions using a file will show it missing", async () => {
    const user = renderPage();
    await openMenu(user, "Unit 4 · Airport announcements.mp3");
    await user.click(await screen.findByRole("menuitem", { name: "Xoá" }));

    expect(await screen.findByRole("dialog")).toHaveTextContent(
      "Đang dùng trong 3 câu hỏi. Các câu hỏi đó sẽ báo thiếu tệp.",
    );
  });
});

describe("the upload dialog", () => {
  it("uploads audio with the default play limit and adds it to the library", async () => {
    let sent: URLSearchParams | null = null;
    server.use(
      http.post(`${BASE}/teacher/media`, ({ request }) => {
        sent = new URL(request.url).searchParams;
        return contractJson("/teacher/media", "post", 201, {
          id: "018f0000-0000-7000-8000-0000000000c1",
          kind: "audio",
          url: "https://example.test/new.mp3",
          mimeType: "audio/mpeg",
          bytes: 1024,
          durationMs: 30_000,
          originalFilename: "unit-5.mp3",
          createdAt: "2026-10-08T00:00:00Z",
        });
      }),
    );
    const user = renderPage();
    await screen.findByText("Directions map.png");
    await user.click(screen.getByRole("button", { name: "Tải lên" }));

    const dialog = await screen.findByRole("dialog", { name: "Tải lên media" });
    expect(
      within(dialog).getByRole("button", { name: "Âm thanh", pressed: true }),
    ).toBeVisible();
    expect(dialog).toHaveTextContent("Giới hạn lượt nghe");
    const before = lists.length;
    await user.upload(
      dialog.querySelector<HTMLInputElement>("input[type=file]")!,
      new File([new Uint8Array(16)], "unit-5.mp3"),
    );
    await user.click(within(dialog).getByRole("button", { name: "Tải lên" }));

    expect(await screen.findByText("Đã tải lên unit-5.mp3")).toBeVisible();
    expect(sent!.get("defaultMaxPlays")).toBe("0");
    await waitFor(() => expect(lists.length).toBeGreaterThan(before));
  });

  it("takes an image without a play limit, under the image's rules", async () => {
    let uploads = 0;
    server.use(
      http.post(`${BASE}/teacher/media`, () => {
        uploads += 1;
        return new Response(null, { status: 500 });
      }),
    );
    const user = renderPage("/teacher/media", false);
    await screen.findByText("Directions map.png");
    await user.click(screen.getByRole("button", { name: "Tải lên" }));
    const dialog = await screen.findByRole("dialog", { name: "Tải lên media" });
    await user.click(within(dialog).getByRole("button", { name: "Hình ảnh" }));

    expect(dialog).not.toHaveTextContent("Giới hạn lượt nghe");
    expect(dialog).toHaveTextContent("JPG, PNG hoặc WebP · tối đa 10 MB");
    await user.upload(
      dialog.querySelector<HTMLInputElement>("input[type=file]")!,
      new File([new Uint8Array(16)], "bieu-do.gif"),
    );
    await user.click(within(dialog).getByRole("button", { name: "Tải lên" }));

    expect(
      await within(dialog).findByText(/không phải JPG, PNG hoặc WebP/),
    ).toBeVisible();
    expect(uploads).toBe(0);
  });
});
