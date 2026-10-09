import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { QuestionMediaField } from "@/features/question-bank/components/QuestionMediaField";
import { MaterialAssetDialog } from "@/features/question-groups/components/MaterialAssetDialog";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

/**
 * The two editors that hosted UploadPanel upload through useMediaUpload now,
 * each drawing the hook's state itself. These pin that each still checks the
 * file before sending it, hands on what the server stored, says so when the
 * server fails, and cancels on request.
 */
const BASE = "http://localhost:8080";
const STORED = {
  id: "018f0000-0000-7000-8000-0000000000d4",
  kind: "audio",
  url: "https://example.test/hop-le.mp3",
  mimeType: "audio/mpeg",
  bytes: 1024,
  durationMs: 30_000,
  originalFilename: "hop-le.mp3",
  createdAt: "2026-10-08T00:00:00Z",
};

let answer: "stored" | "failed" | "pending" = "stored";
let uploads = 0;

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
  answer = "stored";
  uploads = 0;
  stubDuration(30);
  server.use(
    http.post(`${BASE}/teacher/media`, async () => {
      uploads += 1;
      if (answer === "pending") await new Promise(() => undefined);
      if (answer === "failed") return new Response(null, { status: 500 });
      return contractJson("/teacher/media", "post", 201, STORED);
    }),
    http.get(`${BASE}/teacher/media`, () =>
      contractJson("/teacher/media", "get", 200, {
        items: [],
        page: 1,
        pageSize: 20,
        total: 0,
        totalBytes: 0,
        facets: { all: 0, audio: 0, image: 0, unused: 0 },
        usage: { audioBytes: 0, imageBytes: 0, quotaBytes: 5_368_709_120 },
      }),
    ),
  );
});

afterEach(() => vi.restoreAllMocks());

function audio(name: string): File {
  return new File([new Uint8Array(1024)], name, { type: "audio/mpeg" });
}

const hosts = {
  "the question editor's media field": (onUploaded: (asset: unknown) => void) =>
    render(<QuestionMediaField value={null} onChange={onUploaded} />),
  "the group editor's audio dialog": (onUploaded: (asset: unknown) => void) => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return render(
      <QueryClientProvider client={client}>
        <MaterialAssetDialog
          kind="audio"
          insertError={null}
          onClose={vi.fn()}
          onInsert={(asset) => onUploaded(asset)}
        />
      </QueryClientProvider>,
    );
  },
};

for (const [host, mount] of Object.entries(hosts)) {
  describe(host, () => {
    it("uploads a file within the limits and takes what the server stored", async () => {
      const onUploaded = vi.fn();
      const user = userEvent.setup();
      mount(onUploaded);
      await user.upload(screen.getByLabelText("Chọn tệp từ máy"), audio("hop-le.mp3"));

      await waitFor(() => expect(uploads).toBe(1));
      if (host.startsWith("the question")) {
        await waitFor(() =>
          expect(onUploaded).toHaveBeenCalledWith(
            expect.objectContaining({ originalFilename: "hop-le.mp3" }),
          ),
        );
      } else {
        expect(await screen.findByText(/hop-le\.mp3/)).toBeInTheDocument();
      }
      expect(screen.queryByRole("alert")).toBeNull();
    });

    it("refuses a file over five minutes without sending it", async () => {
      stubDuration(6 * 60);
      const user = userEvent.setup();
      mount(vi.fn());
      await user.upload(screen.getByLabelText("Chọn tệp từ máy"), audio("dai.mp3"));

      expect(await screen.findByRole("alert")).toHaveTextContent(/dai\.mp3.*5 phút/);
      expect(uploads).toBe(0);
    });

    it("says so when the server fails", async () => {
      answer = "failed";
      const user = userEvent.setup();
      mount(vi.fn());
      await user.upload(screen.getByLabelText("Chọn tệp từ máy"), audio("loi.mp3"));

      expect(await screen.findByRole("alert")).toHaveTextContent(
        "Không dùng được tệp này",
      );
    });

    it("shows the upload in progress and aborts it on cancel", async () => {
      answer = "pending";
      const abort = vi.spyOn(XMLHttpRequest.prototype, "abort");
      const user = userEvent.setup();
      mount(vi.fn());
      await user.upload(screen.getByLabelText("Chọn tệp từ máy"), audio("cham.mp3"));

      expect(
        await screen.findByRole("progressbar", { name: "Đang tải lên" }),
      ).toBeVisible();
      await user.click(screen.getByRole("button", { name: "Huỷ tải lên" }));

      expect(abort).toHaveBeenCalled();
    });
  });
}
