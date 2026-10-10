import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { MaterialAssetDialog } from "@/features/question-groups/components/MaterialAssetDialog";
import { MAX_AUDIO_BYTES, MAX_IMAGE_BYTES } from "@/features/media/limits";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

let uploads = 0;

beforeEach(() => {
  uploads = 0;
  server.use(
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
    http.post(`${BASE}/teacher/media`, () => {
      uploads += 1;
      return contractJson("/teacher/media", "post", 201, {
        id: "018f0000-0000-7000-8000-0000000000e2",
        kind: "image",
        url: "https://example.test/ban-do.png",
        mimeType: "image/png",
        bytes: 1024,
        durationMs: null,
        originalFilename: "ban-do.png",
        createdAt: "2026-10-04T00:00:00Z",
      });
    }),
  );
});

function imageReporting(bytes: number): File {
  const file = new File([new Uint8Array(16)], "ban-do.png", { type: "image/png" });
  Object.defineProperty(file, "size", { value: bytes });
  return file;
}

async function choose(file: File) {
  const user = userEvent.setup();
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MaterialAssetDialog
        kind="image"
        insertError={null}
        onClose={vi.fn()}
        onInsert={vi.fn()}
      />
    </QueryClientProvider>,
  );
  await user.upload(
    screen.getByLabelText("Chọn tệp từ máy", { selector: "input" }),
    file,
  );
}

describe("the group editor's image upload keeps the image limit", () => {
  it("refuses an image one byte over 10 MB without sending it", async () => {
    await choose(imageReporting(MAX_IMAGE_BYTES + 1));

    expect(await screen.findByRole("alert")).toHaveTextContent(/vượt quá 10 MB/);
    expect(uploads).toBe(0);
  });

  it("sends an image of exactly 10 MB", async () => {
    await choose(imageReporting(MAX_IMAGE_BYTES));

    await waitFor(() => expect(uploads).toBe(1));
    expect(await screen.findByText(/ban-do\.png/)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("does not take the audio limit for an image", async () => {
    await choose(imageReporting(MAX_AUDIO_BYTES));

    expect(await screen.findByRole("alert")).toHaveTextContent(/vượt quá 10 MB/);
    expect(uploads).toBe(0);
  });
});
