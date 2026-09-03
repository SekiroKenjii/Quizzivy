import { createRef } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import {
  UploadPanel,
  type UploadHandle,
} from "@/features/media/components/UploadPanel";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

// jsdom has no media pipeline, so <audio> never fires loadedmetadata. The
// duration each test wants is stubbed on the element prototype, which is what
// the widget reads.
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

let uploadCalls = 0;

beforeEach(() => {
  uploadCalls = 0;
  server.use(
    http.post(`${BASE}/admin/media`, () => {
      uploadCalls += 1;
      return new Response(null, { status: 500 });
    }),
  );
});

async function choose(file: File) {
  const user = userEvent.setup();
  render(<UploadPanel onUploaded={vi.fn()} />);
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

  it("rejects a file over 10 MB without reading its duration", async () => {
    stubDuration(10);
    await choose(audioFile("qua-lon.mp3", 11 * 1024 * 1024));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/qua-lon\.mp3/);
    expect(alert).toHaveTextContent(/10 MB/);
    expect(uploadCalls).toBe(0);
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
    const handle = createRef<UploadHandle>();
    render(<UploadPanel ref={handle} onUploaded={vi.fn()} />);

    act(() => handle.current?.dropped([]));

    expect(await screen.findByRole("alert")).toHaveTextContent(/thư mục/);
    expect(uploadCalls).toBe(0);
  });

  it("refuses a multi-file drop rather than uploading one and dropping the rest", async () => {
    const handle = createRef<UploadHandle>();
    render(<UploadPanel ref={handle} onUploaded={vi.fn()} />);

    act(() =>
      handle.current?.dropped([audioFile("a.mp3", 16), audioFile("b.mp3", 16)]),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(/một tệp/);
    expect(uploadCalls, "a partial upload reads as the others failing").toBe(0);
  });
});
