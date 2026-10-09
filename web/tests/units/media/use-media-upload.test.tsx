import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import type { MediaAsset, MediaKind } from "@/features/media/api";
import { UploadStatus } from "@/features/media/components/UploadStatus";
import { MAX_AUDIO_BYTES, MAX_IMAGE_BYTES } from "@/features/media/limits";
import {
  useMediaUpload,
  type MediaUpload,
  type UploadSender,
} from "@/features/media/useMediaUpload";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

// jsdom has no media pipeline, so <audio> never fires loadedmetadata. The
// duration each test wants is stubbed on the element prototype, which is what
// the hook reads.
function stubDuration(seconds: number | null) {
  Object.defineProperty(HTMLMediaElement.prototype, "duration", {
    configurable: true,
    get: () => (seconds === null ? NaN : seconds),
  });
  Object.defineProperty(HTMLMediaElement.prototype, "src", {
    configurable: true,
    set(this: HTMLMediaElement) {
      // Metadata "arrives" on the next tick, as it would in a browser.
      setTimeout(() => {
        if (seconds === null) this.onerror?.(new Event("error"));
        else this.onloadedmetadata?.(new Event("loadedmetadata"));
      }, 0);
    },
  });
}

function audioFile(name: string, bytes: number): File {
  return new File([new Uint8Array(bytes)], name, { type: "audio/mpeg" });
}

function audioFileReporting(name: string, bytes: number): File {
  const file = audioFile(name, 16);
  Object.defineProperty(file, "size", { value: bytes });
  return file;
}

function imageFileReporting(name: string, bytes: number): File {
  const file = new File([new Uint8Array(16)], name, { type: "image/png" });
  Object.defineProperty(file, "size", { value: bytes });
  return file;
}

let uploadCalls = 0;
let durationReads = 0;

beforeEach(() => {
  uploadCalls = 0;
  durationReads = 0;
  server.use(
    http.post(`${BASE}/teacher/media`, () => {
      uploadCalls += 1;
      return new Response(null, { status: 500 });
    }),
  );
});

/** A host as the two editors are: a file input, the hook and its status. */
function Host({
  kind,
  send,
  handleRef,
  onUploaded = vi.fn(),
}: Readonly<{
  kind?: MediaKind;
  send?: UploadSender<MediaAsset>;
  handleRef?: { current: MediaUpload<MediaAsset> | null };
  onUploaded?: (asset: MediaAsset) => void;
}>) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const upload = useMediaUpload({
    onUploaded,
    ...(kind ? { kind } : {}),
    ...(send ? { send } : {}),
  });
  useEffect(() => {
    if (handleRef) handleRef.current = upload;
  });
  return (
    <>
      <input
        ref={input}
        type="file"
        aria-label={t("media.chooseFile")}
        onChange={(event) => {
          const file = event.target.files?.[0];
          event.target.value = "";
          if (file) void upload.start(file);
        }}
      />
      <UploadStatus
        state={upload.state}
        onCancel={upload.cancel}
        onRetry={() => input.current?.click()}
      />
    </>
  );
}

async function choose(file: File, kind?: MediaKind) {
  const user = userEvent.setup();
  render(<Host {...(kind ? { kind } : {})} />);
  await user.upload(screen.getByLabelText("Chọn tệp từ máy"), file);
}

describe("the upload panel's client-side pre-check", () => {
  it("rejects a 6-minute file in Vietnamese without contacting the server", async () => {
    stubDuration(6 * 60);
    await choose(audioFile("bai-nghe-dai.mp3", 1024));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/bai-nghe-dai\.mp3/);
    expect(alert).toHaveTextContent(/6:00/);
    expect(alert).toHaveTextContent(/5 phút/);
    expect(uploadCalls, "an over-length file must not be uploaded").toBe(0);
  });

  it("rejects a file one byte over 50 MB without reading its duration", async () => {
    stubDuration(10);
    await choose(audioFileReporting("qua-lon.mp3", MAX_AUDIO_BYTES + 1));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/qua-lon\.mp3/);
    expect(alert).toHaveTextContent(/50 MB/);
    expect(uploadCalls).toBe(0);
  });

  it("uploads a file of exactly 50 MB", async () => {
    stubDuration(10);
    await choose(audioFileReporting("vua-du.mp3", MAX_AUDIO_BYTES));

    await waitFor(() => expect(uploadCalls).toBe(1));
  });

  it("rejects a file whose extension is not mp3 or m4a", async () => {
    stubDuration(10);
    await choose(audioFile("bai-nghe.wav", 1024));

    expect(await screen.findByRole("alert")).toHaveTextContent(/mp3/);
    expect(uploadCalls).toBe(0);
  });

  it("uploads anyway when the browser cannot read the duration", async () => {
    stubDuration(null);
    await choose(audioFile("khong-doc-duoc.m4a", 1024));

    await waitFor(() => {
      expect(uploadCalls, "an unreadable duration must not block the upload").toBe(1);
    });
  });

  it("uploads a file within both limits", async () => {
    stubDuration(90);
    await choose(audioFile("hop-le.mp3", 2 * 1024 * 1024));

    await waitFor(() => expect(uploadCalls).toBe(1));
  });

  it("reports a server failure rather than going quiet", async () => {
    stubDuration(30);
    await choose(audioFile("that-bai.mp3", 1024));

    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("says why a dropped folder produced nothing, instead of swallowing it", async () => {
    const handle: { current: MediaUpload<MediaAsset> | null } = { current: null };
    render(<Host handleRef={handle} />);

    act(() => handle.current?.dropped([]));

    expect(await screen.findByRole("alert")).toHaveTextContent(/thư mục/);
    expect(uploadCalls).toBe(0);
  });

  it("refuses a multi-file drop rather than uploading one and dropping the rest", async () => {
    const handle: { current: MediaUpload<MediaAsset> | null } = { current: null };
    render(<Host handleRef={handle} />);

    act(() =>
      handle.current?.dropped([audioFile("a.mp3", 16), audioFile("b.mp3", 16)]),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(/một tệp/);
    expect(uploadCalls, "a partial upload reads as the others failing").toBe(0);
  });
});

describe("an image's pre-check (DG-63)", () => {
  beforeEach(() => {
    stubDuration(10);
    const read = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "src");
    Object.defineProperty(HTMLMediaElement.prototype, "src", {
      configurable: true,
      set(this: HTMLMediaElement, value: string) {
        durationReads += 1;
        read?.set?.call(this, value);
      },
    });
  });

  it("refuses a GIF, which no screen takes", async () => {
    await choose(imageFileReporting("bieu-do.gif", 1024), "image");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/bieu-do\.gif/);
    expect(alert).toHaveTextContent(/WebP/);
    expect(uploadCalls).toBe(0);
  });

  it("refuses an image one byte over 10 MB", async () => {
    await choose(imageFileReporting("ban-do.png", MAX_IMAGE_BYTES + 1), "image");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/ban-do\.png/);
    expect(alert).toHaveTextContent(/10 MB/);
    expect(uploadCalls).toBe(0);
  });

  it("sends an image of exactly 10 MB without reading a duration", async () => {
    await choose(imageFileReporting("vua-du.webp", MAX_IMAGE_BYTES), "image");

    await waitFor(() => expect(uploadCalls).toBe(1));
    expect(durationReads, "an image has no duration to read").toBe(0);
  });

  it("checks an mp3 chosen as an image against the image's rules", async () => {
    await choose(audioFile("bai-nghe.mp3", 1024), "image");

    expect(await screen.findByRole("alert")).toHaveTextContent(/JPG, PNG hoặc WebP/);
    expect(uploadCalls).toBe(0);
  });
});

describe("the upload itself", () => {
  it("hands the host what the server stored and clears its state", async () => {
    stubDuration(30);
    server.use(
      http.post(`${BASE}/teacher/media`, () => {
        uploadCalls += 1;
        return Response.json(
          {
            id: "018f0000-0000-7000-8000-0000000000e3",
            kind: "audio",
            url: "https://example.test/hop-le.mp3",
            mimeType: "audio/mpeg",
            bytes: 1024,
            durationMs: 30_000,
            originalFilename: "hop-le.mp3",
            createdAt: "2026-10-08T00:00:00Z",
          },
          { status: 201 },
        );
      }),
    );
    const onUploaded = vi.fn();
    const user = userEvent.setup();
    render(<Host onUploaded={onUploaded} />);
    await user.upload(
      screen.getByLabelText("Chọn tệp từ máy"),
      audioFile("hop-le.mp3", 1024),
    );

    await waitFor(() => expect(onUploaded).toHaveBeenCalledTimes(1));
    expect(onUploaded.mock.calls[0]?.[0]).toMatchObject({
      originalFilename: "hop-le.mp3",
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows the progress and cancels on request, saying so", async () => {
    stubDuration(30);
    const sent = vi.fn<UploadSender<MediaAsset>>(
      (_file, { signal, onProgress }) =>
        new Promise<MediaAsset>((_resolve, reject) => {
          onProgress?.(0.4);
          signal?.addEventListener("abort", () =>
            reject(new DOMException("Aborted", "AbortError")),
          );
        }),
    );
    const onUploaded = vi.fn();
    const user = userEvent.setup();
    render(<Host onUploaded={onUploaded} send={sent} />);
    await user.upload(
      screen.getByLabelText("Chọn tệp từ máy"),
      audioFile("dang-tai.mp3", 1024),
    );

    expect(
      await screen.findByRole("progressbar", { name: "Đang tải lên" }),
    ).toHaveValue(0.4);
    expect(screen.getByText(/Đang tải lên · 40/)).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Huỷ tải lên" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Đã huỷ tải lên.");
    expect(onUploaded).not.toHaveBeenCalled();
  });

  it("cancels while the duration is still being read, which may never end", async () => {
    Object.defineProperty(HTMLMediaElement.prototype, "src", {
      configurable: true,
      set() {},
    });
    const sent = vi.fn<UploadSender<MediaAsset>>();
    const user = userEvent.setup();
    render(<Host send={sent} />);
    await user.upload(
      screen.getByLabelText("Chọn tệp từ máy"),
      audioFile("treo.mp3", 1024),
    );
    expect(await screen.findByRole("status")).toHaveTextContent("treo.mp3");

    await user.click(screen.getByRole("button", { name: "Huỷ tải lên" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Đã huỷ tải lên.");
    expect(sent).not.toHaveBeenCalled();
  });

  it("aborts the upload in flight when its host unmounts", async () => {
    stubDuration(30);
    let signal: AbortSignal | undefined;
    const sent = vi.fn<UploadSender<MediaAsset>>(
      (_file, options) =>
        new Promise<MediaAsset>(() => {
          signal = options.signal;
        }),
    );
    const onUploaded = vi.fn();
    const user = userEvent.setup();
    const { unmount } = render(<Host onUploaded={onUploaded} send={sent} />);
    await user.upload(
      screen.getByLabelText("Chọn tệp từ máy"),
      audioFile("roi-di.mp3", 1024),
    );
    await screen.findByRole("progressbar");

    unmount();

    expect(signal?.aborted).toBe(true);
    expect(onUploaded).not.toHaveBeenCalled();
  });
});
