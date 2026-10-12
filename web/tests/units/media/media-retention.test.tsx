import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import MediaPage from "@/features/media/pages/teacher/MediaPage";
import type { LibraryAsset, MediaUsage } from "@/features/media/api";
import { Toaster } from "@/components/ui/sonner";
import i18n from "@/lib/i18n";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";

const BASE = "http://localhost:8080";
const MIB = 1024 * 1024;
const QUOTA = 5 * 1024 * MIB;
const NEW_FILE = "018f0000-0000-7000-8000-0000000000b1";
const TEST_ONE = "018f0000-0000-7000-8000-0000000000d1";
const TEST_TWO = "018f0000-0000-7000-8000-0000000000d2";
const GOES_ON = "2026-10-19T03:00:00Z";

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

const FREE = asset({
  id: "018f0000-0000-7000-8000-0000000000a1",
  displayName: "Free listening.mp3",
});
const HELD = asset({
  id: "018f0000-0000-7000-8000-0000000000a2",
  displayName: "Held listening.mp3",
  questionCount: 2,
  usageCount: 3,
  usedIn: [
    { id: TEST_ONE, title: "Đề 1", version: 1 },
    { id: TEST_ONE, title: "Đề 1", version: 2 },
    { id: TEST_TWO, title: "Đề 2", version: 1 },
  ],
});

let usage: MediaUsage;
let items: LibraryAsset[];

function libraryBody() {
  return {
    items,
    page: 1,
    pageSize: 24,
    total: items.length,
    totalBytes: items.reduce((sum, row) => sum + row.bytes, 0),
    facets: { all: items.length, audio: items.length, image: 0, unused: 0 },
    usage,
  };
}

function durationUnreadable() {
  Object.defineProperty(HTMLMediaElement.prototype, "src", {
    configurable: true,
    set(this: HTMLMediaElement) {
      setTimeout(() => this.onerror?.(new Event("error")), 0);
    },
  });
}

beforeEach(() => {
  usage = { audioBytes: 100 * MIB, imageBytes: 20 * MIB, quotaBytes: QUOTA };
  items = [FREE, HELD];
  durationUnreadable();
  server.use(
    http.get(`${BASE}/teacher/media`, () =>
      contractJson("/teacher/media", "get", 200, libraryBody()),
    ),
  );
});

afterEach(async () => {
  cleanup();
  await i18n.changeLanguage("vi");
});

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/media", element: <MediaPage /> },
      { path: "/teacher/tests/:id", element: <p>test</p> },
    ],
    { initialEntries: ["/teacher/media"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

type User = ReturnType<typeof userEvent.setup>;

async function openAction(user: User, file: string, action: string) {
  await user.click(
    await screen.findByRole("button", { name: `Thao tác với tệp ${file}` }),
  );
  await user.click(await screen.findByRole("menuitem", { name: action }));
}

async function chooseReplacement(user: User, file: string) {
  await openAction(user, file, "Thay tệp");
  const dialog = await screen.findByRole("dialog", { name: "Thay tệp" });
  await user.upload(
    dialog.querySelector<HTMLInputElement>("input[type=file]")!,
    new File([new Uint8Array(16)], "moi.mp3"),
  );
  return dialog;
}

function replaced(retired?: {
  usedIn: LibraryAsset["usedIn"];
  deleteAt: string | null;
}) {
  return contractJson("/teacher/media/{id}/replace", "post", 201, {
    asset: { ...FREE, id: NEW_FILE },
    repointed: { questions: 0, groups: 0 },
    left: { questions: 0, groups: 0 },
    ...(retired === undefined
      ? {}
      : {
          previous: {
            id: "018f0000-0000-7000-8000-0000000000c9",
            bytes: 2_516_582,
            url: "https://example.test/old.mp3",
            ...retired,
          },
        }),
  });
}

describe("the replace dialog says what becomes of the old file", () => {
  it("says an unused file is deleted for good seven days later, with its download", async () => {
    const user = renderPage();
    await openAction(user, "Free listening.mp3", "Thay tệp");

    const dialog = await screen.findByRole("dialog", { name: "Thay tệp" });
    expect(dialog).toHaveTextContent(
      "Chưa có đề đã xuất bản nào dùng tệp này, nên tệp cũ bị xoá hẳn 7 ngày sau khi thay và không khôi phục được. Hãy tải về ngay nếu bạn muốn giữ một bản sao.",
    );
    const download = within(dialog).getByRole("link", {
      name: "Tải xuống Free listening.mp3",
    });
    expect(download).toHaveAttribute("href", FREE.url);
    expect(download).toHaveAttribute("target", "_blank");
    expect(download).toHaveAttribute("rel", "noopener noreferrer");
    expect(download).toHaveTextContent("Tải xuống");
  });

  it("counts a test once however many of its versions hold the file", async () => {
    const user = renderPage();
    await openAction(user, "Held listening.mp3", "Thay tệp");

    const dialog = await screen.findByRole("dialog", { name: "Thay tệp" });
    expect(dialog).toHaveTextContent(
      "2 đề đã xuất bản vẫn dùng tệp này. Tệp được giữ lại cho các đề đó và bị xoá hẳn 7 ngày sau khi không còn đề nào dùng. Hãy tải về ngay nếu bạn muốn giữ một bản sao.",
    );
    expect(
      within(dialog).getByRole("link", { name: "Tải xuống Held listening.mp3" }),
    ).toHaveAttribute("href", HELD.url);
  });

  it("says it in English", async () => {
    await i18n.changeLanguage("en");
    const user = renderPage();
    await user.click(
      await screen.findByRole("button", {
        name: "File actions for Held listening.mp3",
      }),
    );
    await user.click(await screen.findByRole("menuitem", { name: "Replace file" }));

    const dialog = await screen.findByRole("dialog", { name: "Replace file" });
    expect(dialog).toHaveTextContent(
      "2 published tests still use this file. It is kept for them and deleted 7 days after no test uses it. Download it now if you want a copy.",
    );
    expect(
      within(dialog).getByRole("link", { name: "Download Held listening.mp3" }),
    ).toBeVisible();
  });

  it("states the date the old file goes once it is replaced, with no way back", async () => {
    server.use(
      http.post(`${BASE}/teacher/media/:id/replace`, () =>
        replaced({ usedIn: [], deleteAt: GOES_ON }),
      ),
    );
    const user = renderPage();
    const dialog = await chooseReplacement(user, "Free listening.mp3");
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(
      await screen.findByText("Đã thay tệp. Tệp cũ bị xoá hẳn vào ngày 19/10/2026."),
    ).toBeVisible();
    expect(screen.queryByRole("button", { name: /Khôi phục|Hoàn tác/ })).toBeNull();
  });

  it("states the date in English", async () => {
    await i18n.changeLanguage("en");
    server.use(
      http.post(`${BASE}/teacher/media/:id/replace`, () =>
        replaced({ usedIn: [], deleteAt: GOES_ON }),
      ),
    );
    const user = renderPage();
    await user.click(
      await screen.findByRole("button", {
        name: "File actions for Free listening.mp3",
      }),
    );
    await user.click(await screen.findByRole("menuitem", { name: "Replace file" }));
    const dialog = await screen.findByRole("dialog", { name: "Replace file" });
    await user.upload(
      dialog.querySelector<HTMLInputElement>("input[type=file]")!,
      new File([new Uint8Array(16)], "new.mp3"),
    );
    await user.click(within(dialog).getByRole("button", { name: "Replace" }));

    expect(
      await screen.findByText(
        "File replaced. The old file is deleted for good on 19 Oct 2026.",
      ),
    ).toBeVisible();
  });

  it("says how many published tests keep the old file while a date is not known", async () => {
    server.use(
      http.post(`${BASE}/teacher/media/:id/replace`, () =>
        replaced({ usedIn: HELD.usedIn, deleteAt: null }),
      ),
    );
    const user = renderPage();
    const dialog = await chooseReplacement(user, "Held listening.mp3");
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(
      await screen.findByText("Đã thay tệp. Tệp cũ được giữ lại cho 2 đề đã xuất bản."),
    ).toBeVisible();
  });

  it("keeps the plain line for an answer that says nothing of the old file", async () => {
    server.use(http.post(`${BASE}/teacher/media/:id/replace`, () => replaced()));
    const user = renderPage();
    const dialog = await chooseReplacement(user, "Free listening.mp3");
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(await screen.findByText("Đã thay tệp")).toBeVisible();
  });
});

describe("the replace dialog's refusals", () => {
  const quotaFull = () =>
    contractJson("/teacher/media/{id}/replace", "post", 409, {
      error: {
        code: "MEDIA_QUOTA_EXCEEDED",
        message: "Thư viện đã hết dung lượng. Hãy xoá bớt tệp rồi tải lại.",
        requestId: "018f0000-0000-7000-8000-0000000000f1",
      },
    });

  it("says a held file needs room for both files when the library is full", async () => {
    server.use(http.post(`${BASE}/teacher/media/:id/replace`, quotaFull));
    const user = renderPage();
    const dialog = await chooseReplacement(user, "Held listening.mp3");
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Không đủ dung lượng để thay tệp. Tệp cũ vẫn được giữ lại cho đề đã xuất bản và vẫn tính vào dung lượng, nên thư viện cần đủ chỗ cho cả hai tệp. Hãy xoá bớt tệp khác rồi thử lại.",
    );
  });

  it("leaves the server's words for a file no published test holds", async () => {
    server.use(http.post(`${BASE}/teacher/media/:id/replace`, quotaFull));
    const user = renderPage();
    const dialog = await chooseReplacement(user, "Free listening.mp3");
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Thư viện đã hết dung lượng. Hãy xoá bớt tệp rồi tải lại.",
    );
  });

  for (const [retryAfter, wording] of [
    ["1", "Bạn đang thay tệp quá nhanh. Hãy thử lại sau 1 giây."],
    ["37", "Bạn đang thay tệp quá nhanh. Hãy thử lại sau 37 giây."],
    ["60", "Bạn đang thay tệp quá nhanh. Hãy thử lại sau 1 phút."],
    ["130", "Bạn đang thay tệp quá nhanh. Hãy thử lại sau 3 phút."],
  ] as const) {
    it(`says how long to wait when the server asks for ${retryAfter} s`, async () => {
      server.use(
        http.post(`${BASE}/teacher/media/:id/replace`, () => {
          const response = contractJson("/teacher/media/{id}/replace", "post", 429, {
            error: {
              code: "RATE_LIMITED",
              message: "Too many requests.",
              requestId: "018f0000-0000-7000-8000-0000000000f3",
            },
          });
          response.headers.set("Retry-After", retryAfter);
          return response;
        }),
      );
      const user = renderPage();
      const dialog = await chooseReplacement(user, "Free listening.mp3");
      await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

      expect(await within(dialog).findByRole("alert")).toHaveTextContent(wording);
    });
  }

  it("leaves the server's words for a rate limit that names no wait", async () => {
    server.use(
      http.post(`${BASE}/teacher/media/:id/replace`, () => {
        return contractJson("/teacher/media/{id}/replace", "post", 429, {
          error: {
            code: "RATE_LIMITED",
            message: "Bạn thao tác quá nhanh.",
            requestId: "018f0000-0000-7000-8000-0000000000f4",
          },
        });
      }),
    );
    const user = renderPage();
    const dialog = await chooseReplacement(user, "Free listening.mp3");
    await user.click(within(dialog).getByRole("button", { name: "Thay tệp" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Bạn thao tác quá nhanh.",
    );
  });
});

describe("the delete dialog says what becomes of the file", () => {
  const gone = (deleteAt: string | null) =>
    contractJson("/teacher/media/{id}", "delete", 200, {
      id: FREE.id,
      bytes: FREE.bytes,
      url: FREE.url,
      usedIn: [],
      deleteAt,
    });

  it("says the file is deleted for good seven days later, with its download", async () => {
    const user = renderPage();
    await openAction(user, "Free listening.mp3", "Xoá");

    const dialog = await screen.findByRole("dialog", {
      name: "Xoá Free listening.mp3?",
    });
    expect(dialog).toHaveTextContent(
      "Tệp bị xoá hẳn 7 ngày sau khi bạn xoá và không khôi phục được. Hãy tải về ngay nếu bạn muốn giữ một bản sao.",
    );
    const download = within(dialog).getByRole("link", {
      name: "Tải xuống Free listening.mp3",
    });
    expect(download).toHaveAttribute("href", FREE.url);
    expect(download).toHaveAttribute("target", "_blank");
  });

  it("offers no download in the notice that a published test needs the file", async () => {
    const user = renderPage();
    await openAction(user, "Held listening.mp3", "Xoá");

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("Tệp đang được một phiên bản đã xuất bản sử dụng");
    expect(within(dialog).queryByRole("link", { name: /Tải xuống/ })).toBeNull();
    expect(dialog).not.toHaveTextContent("xoá hẳn");
  });

  it("states the date the file goes once it is deleted", async () => {
    server.use(http.delete(`${BASE}/teacher/media/:id`, () => gone(GOES_ON)));
    const user = renderPage();
    await openAction(user, "Free listening.mp3", "Xoá");
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));

    expect(
      await screen.findByText("Đã xoá tệp. Tệp bị xoá hẳn vào ngày 19/10/2026."),
    ).toBeVisible();
    expect(screen.queryByRole("button", { name: /Khôi phục|Hoàn tác/ })).toBeNull();
  });

  it("says a file something still holds is kept until nothing uses it", async () => {
    server.use(http.delete(`${BASE}/teacher/media/:id`, () => gone(null)));
    const user = renderPage();
    await openAction(user, "Free listening.mp3", "Xoá");
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));

    expect(
      await screen.findByText(
        "Đã xoá tệp. Tệp được giữ lại cho đến khi không còn gì dùng nó.",
      ),
    ).toBeVisible();
  });

  it("keeps the plain line for an answer with no body", async () => {
    server.use(
      http.delete(
        `${BASE}/teacher/media/:id`,
        () => new Response(null, { status: 204 }),
      ),
    );
    const user = renderPage();
    await openAction(user, "Free listening.mp3", "Xoá");
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));

    expect(await screen.findByText("Đã xoá tệp")).toBeVisible();
  });
});

describe("the storage card counts the files kept for published tests", () => {
  it("adds what is held to what is used, and says what is held", async () => {
    usage = {
      audioBytes: 100 * MIB,
      imageBytes: 20 * MIB,
      quotaBytes: QUOTA,
      heldBytes: 30 * MIB,
      pendingBytes: 0,
      pendingDeleteAt: null,
    };
    renderPage();

    expect(
      await screen.findByText("0.15 GB trên 5 GB · âm thanh 100 MB · hình ảnh 20 MB"),
    ).toBeVisible();
    expect(screen.getByText("Giữ lại cho đề đã xuất bản · 30 MB")).toBeVisible();
    expect(screen.queryByText(/Chờ xoá hẳn/)).toBeNull();
  });

  it("says what waits for deletion and from which day", async () => {
    usage = {
      audioBytes: 100 * MIB,
      imageBytes: 20 * MIB,
      quotaBytes: QUOTA,
      heldBytes: 0,
      pendingBytes: 12 * MIB,
      pendingDeleteAt: GOES_ON,
    };
    renderPage();

    expect(
      await screen.findByText("Chờ xoá hẳn · 12 MB · từ ngày 19/10/2026"),
    ).toBeVisible();
    expect(
      screen.getByText("0.12 GB trên 5 GB · âm thanh 100 MB · hình ảnh 20 MB"),
    ).toBeVisible();
    expect(screen.queryByText(/Giữ lại cho đề/)).toBeNull();
  });

  it("says both in English", async () => {
    await i18n.changeLanguage("en");
    usage = {
      audioBytes: 100 * MIB,
      imageBytes: 20 * MIB,
      quotaBytes: QUOTA,
      heldBytes: 30 * MIB,
      pendingBytes: 12 * MIB,
      pendingDeleteAt: GOES_ON,
    };
    renderPage();

    expect(await screen.findByText("Kept for published tests · 30 MB")).toBeVisible();
    expect(
      screen.getByText("Pending deletion · 12 MB · from 19 Oct 2026"),
    ).toBeVisible();
    expect(
      screen.getByText("0.15 GB of 5 GB · audio 100 MB · images 20 MB"),
    ).toBeVisible();
  });

  it("draws neither line while nothing is held or waiting", async () => {
    renderPage();

    expect(
      await screen.findByText("0.12 GB trên 5 GB · âm thanh 100 MB · hình ảnh 20 MB"),
    ).toBeVisible();
    await waitFor(() => expect(screen.queryByText(/Giữ lại cho đề/)).toBeNull());
    expect(screen.queryByText(/Chờ xoá hẳn/)).toBeNull();
  });
});
