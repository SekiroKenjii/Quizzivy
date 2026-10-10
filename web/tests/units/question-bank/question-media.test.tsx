import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { Toaster, toast } from "@/components/ui/sonner";
import type { MediaAsset } from "@/features/media/api";
import { QuestionMediaField } from "@/features/question-bank/components/QuestionMediaField";
import {
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

const AUDIO: MediaAsset = {
  id: "018f0000-0000-7000-8000-0000000000a1",
  kind: "audio",
  originalFilename: "unit5-listening.mp3",
  bytes: 2_400_000,
  durationMs: 110_000,
  mimeType: "audio/mpeg",
  createdAt: "2026-01-01T00:00:00Z",
  url: "https://example.test/unit5-listening.mp3",
};

const IMAGE: MediaAsset = {
  id: "018f0000-0000-7000-8000-0000000000a2",
  kind: "image",
  originalFilename: "ban-do.png",
  bytes: 24_000,
  mimeType: "image/png",
  createdAt: "2026-01-01T00:00:00Z",
  url: "https://example.test/ban-do.png",
};

let stored: MediaAsset = AUDIO;
let uploads = 0;
let pending = false;

function stubDuration(seconds: number) {
  Object.defineProperty(HTMLMediaElement.prototype, "duration", {
    configurable: true,
    get: () => seconds,
  });
  Object.defineProperty(HTMLMediaElement.prototype, "src", {
    configurable: true,
    set(this: HTMLMediaElement) {
      setTimeout(() => this.onloadedmetadata?.(new Event("loadedmetadata")), 0);
    },
  });
}

beforeEach(() => {
  stored = AUDIO;
  uploads = 0;
  pending = false;
  stubDuration(30);
  server.use(
    http.post(`${BASE}/teacher/media`, async () => {
      uploads += 1;
      if (pending) await new Promise(() => undefined);
      return contractJson("/teacher/media", "post", 201, stored);
    }),
  );
});

afterEach(() => {
  toast.dismiss();
  vi.restoreAllMocks();
});

function file(name: string, bytes = 1024, type = ""): File {
  const made = new File([new Uint8Array(16)], name, { type });
  Object.defineProperty(made, "size", { value: bytes });
  return made;
}

function renderBlock(
  overrides: Partial<QuestionValues> = {},
  asset: MediaAsset | null = null,
) {
  const seen: QuestionValues[] = [];
  function Harness() {
    const [value, setValue] = useState<QuestionValues>({
      ...emptyQuestion(),
      ...overrides,
    });
    const [current, setCurrent] = useState(asset);
    return (
      <QuestionMediaField
        value={value}
        asset={current}
        onChange={(next) => {
          seen.push(next);
          setValue(next);
        }}
        onAssetChange={setCurrent}
      />
    );
  }
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <Harness />
      <Toaster />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup({ applyAccept: false }), seen };
}

describe("the question media block", () => {
  it("starts empty with the drop zone, its limits and both ways in", () => {
    renderBlock();

    expect(screen.getByText("Media của câu hỏi")).toBeInTheDocument();
    expect(
      screen.getByText("Thả tệp âm thanh hoặc hình ảnh vào đây"),
    ).toBeInTheDocument();
    expect(screen.getByText(/50 MB và 5 phút .* 10 MB/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Chọn tệp" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Từ Media" })).toBeVisible();
  });

  it("attaches an uploaded image, shows its preview and saves its alt text", async () => {
    stored = IMAGE;
    const { user, seen } = renderBlock();

    await user.upload(screen.getByLabelText("Chọn tệp từ máy"), file("ban-do.png"));

    const preview = await screen.findByRole("img", { name: "ban-do.png" });
    expect(
      await screen.findByText("Đã lưu vào Media · ban-do.png"),
    ).toBeInTheDocument();
    expect(seen.at(-1)).toMatchObject({
      mediaAssetId: IMAGE.id,
      audio: null,
      transcript: null,
      mediaAlt: null,
    });

    Object.defineProperty(preview, "naturalWidth", { value: 1200 });
    Object.defineProperty(preview, "naturalHeight", { value: 800 });
    fireEvent.load(preview);
    expect(screen.getByText(/1200 × 800 · /)).toBeInTheDocument();

    const alt = screen.getByRole("textbox", { name: "Văn bản thay thế" });
    expect(alt).toHaveAccessibleDescription(/trình đọc màn hình/);
    await user.type(alt, "Bản đồ thị trấn");

    expect(seen.at(-1)).toMatchObject({ mediaAlt: "Bản đồ thị trấn" });
    expect(screen.getByRole("img", { name: "Bản đồ thị trấn" })).toBeInTheDocument();

    await user.clear(alt);
    expect(seen.at(-1)).toMatchObject({ mediaAlt: null });
  });

  it("names the attached audio with its length and size, and plays it", () => {
    renderBlock(
      {
        mediaAssetId: AUDIO.id,
        audio: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: true },
      },
      AUDIO,
    );

    const name = screen.getByText("unit5-listening.mp3");
    expect(name.parentElement).toHaveTextContent("1:50 · 2.3 MB");
    const play = screen.getByRole("button", { name: "Phát" });
    expect(play.parentElement).toHaveTextContent("0:00 / 1:50");
    expect(
      screen.getByRole("button", { name: "Thay media của câu hỏi này" }),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Gỡ media" })).toBeVisible();
  });

  it("saves Plays and both switches for audio", async () => {
    const { user, seen } = renderBlock(
      {
        mediaAssetId: AUDIO.id,
        audio: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: true },
      },
      AUDIO,
    );
    const plays = screen.getByRole("radiogroup", { name: "Số lần nghe" });
    expect(
      within(plays)
        .getAllByRole("radio")
        .map((radio) => radio.parentElement?.textContent),
    ).toEqual(["Một lần", "Hai lần", "Không giới hạn"]);

    await user.click(within(plays).getByRole("radio", { name: "Một lần" }));
    expect(seen.at(-1)?.audio).toMatchObject({ maxPlays: 1 });
    await user.click(within(plays).getByRole("radio", { name: "Không giới hạn" }));
    expect(seen.at(-1)?.audio).toMatchObject({ maxPlays: null });

    await user.click(screen.getByRole("switch", { name: "Cho tua tới" }));
    expect(seen.at(-1)?.audio).toMatchObject({ allowSeek: true });
    await user.click(
      screen.getByRole("switch", { name: "Hiện lời thoại sau khi nộp" }),
    );
    expect(seen.at(-1)?.audio).toMatchObject({ showTranscriptAfterSubmit: false });

    await user.type(screen.getByRole("textbox", { name: /Lời thoại/ }), "Hi");
    expect(seen.at(-1)).toMatchObject({ transcript: "Hi" });
  });

  it("shows a stored count outside the three as a fourth option while it is the value", async () => {
    const { user } = renderBlock(
      {
        mediaAssetId: AUDIO.id,
        audio: { maxPlays: 3, allowSeek: false, showTranscriptAfterSubmit: true },
      },
      AUDIO,
    );
    const plays = screen.getByRole("radiogroup", { name: "Số lần nghe" });

    expect(within(plays).getAllByRole("radio")).toHaveLength(4);
    expect(within(plays).getByRole("radio", { name: "3 lần" })).toBeChecked();

    await user.click(within(plays).getByRole("radio", { name: "Hai lần" }));
    expect(within(plays).getAllByRole("radio")).toHaveLength(3);
  });

  it("removes the media with everything that hangs on it", async () => {
    const { user, seen } = renderBlock(
      {
        mediaAssetId: AUDIO.id,
        audio: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: true },
        transcript: "Hello",
      },
      AUDIO,
    );

    await user.click(screen.getByRole("button", { name: "Gỡ media" }));

    expect(seen.at(-1)).toMatchObject({
      mediaAssetId: null,
      audio: null,
      transcript: null,
      mediaAlt: null,
    });
    expect(
      screen.getByText("Thả tệp âm thanh hoặc hình ảnh vào đây"),
    ).toBeInTheDocument();
  });

  it("opens the device's picker for the attached kind on Replace", async () => {
    const { user } = renderBlock({ mediaAssetId: IMAGE.id }, IMAGE);
    const input = screen.getByLabelText<HTMLInputElement>("Chọn tệp từ máy");
    const click = vi.spyOn(input, "click").mockImplementation(() => undefined);

    await user.click(
      screen.getByRole("button", { name: "Thay media của câu hỏi này" }),
    );

    expect(click).toHaveBeenCalledOnce();
    expect(input.accept).not.toContain(".mp3");
    expect(input.accept).toContain(".png");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("says it is checking an audio file's length before it uploads", async () => {
    pending = true;
    let release: () => void = () => undefined;
    Object.defineProperty(HTMLMediaElement.prototype, "src", {
      configurable: true,
      set(this: HTMLMediaElement) {
        release = () => this.onloadedmetadata?.(new Event("loadedmetadata"));
      },
    });
    const { user } = renderBlock();

    await user.upload(
      screen.getByLabelText("Chọn tệp từ máy"),
      file("nghe.mp3", 2_400_000),
    );

    expect(
      await screen.findByText("2.3 MB · đang kiểm tra độ dài…"),
    ).toBeInTheDocument();
    expect(uploads).toBe(0);
    act(() => release());
    expect(
      await screen.findByRole("progressbar", { name: "Đang tải lên" }),
    ).toBeVisible();
  });

  describe("refuses a file before sending it, naming the file", () => {
    const cases = [
      {
        what: "a file of neither kind",
        file: () => file("ghi-chu.txt"),
        title: "Hãy chọn tệp âm thanh hoặc hình ảnh",
        body: "ghi-chu.txt không phải tệp MP3, M4A, JPG, PNG hoặc WebP.",
      },
      {
        what: "an image over 10 MB",
        file: () => file("to.png", 11 * 1024 * 1024),
        title: "Hình ảnh tối đa 10 MB",
        body: /to\.png nặng .*10 MB/,
      },
      {
        what: "audio over 50 MB",
        file: () => file("to.mp3", 51 * 1024 * 1024),
        title: "Âm thanh tối đa 50 MB",
        body: /to\.mp3 nặng .*50 MB/,
      },
      {
        what: "audio over five minutes",
        file: () => file("dai.mp3"),
        duration: 6 * 60,
        title: "Âm thanh dài tối đa 5 phút",
        body: /dai\.mp3 dài 6:00/,
      },
    ];
    for (const refusal of cases) {
      it(refusal.what, async () => {
        if (refusal.duration) stubDuration(refusal.duration);
        const { user } = renderBlock();

        await user.upload(screen.getByLabelText("Chọn tệp từ máy"), refusal.file());

        expect(await screen.findByRole("alert")).toHaveTextContent(refusal.title);
        expect(screen.getByText(refusal.body)).toBeInTheDocument();
        expect(uploads).toBe(0);
        expect(
          screen.getByText("Thả tệp âm thanh hoặc hình ảnh vào đây"),
        ).toBeInTheDocument();
      });
    }
  });

  it("starts an attached recording's Plays from its default in Media", async () => {
    server.use(
      http.get(`${BASE}/teacher/media`, () =>
        contractJson("/teacher/media", "get", 200, {
          items: [
            {
              ...AUDIO,
              displayName: AUDIO.originalFilename,
              defaultMaxPlays: 0,
              width: null,
              height: null,
              questionCount: 2,
            },
          ],
          page: 1,
          pageSize: 24,
          total: 1,
          totalBytes: AUDIO.bytes,
          facets: { all: 1, audio: 1, image: 0, unused: 0 },
          usage: { audioBytes: AUDIO.bytes, imageBytes: 0, quotaBytes: 5_368_709_120 },
        }),
      ),
    );
    const { user, seen } = renderBlock();

    await user.click(screen.getByRole("button", { name: "Từ Media" }));
    await user.dblClick(
      await screen.findByRole("button", { name: /unit5-listening\.mp3/ }),
    );

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(seen.at(-1)).toMatchObject({
      mediaAssetId: AUDIO.id,
      audio: { maxPlays: null, allowSeek: false, showTranscriptAfterSubmit: true },
    });
  });
});
