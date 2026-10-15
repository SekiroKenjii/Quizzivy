import { vi } from "vitest";

type Options = Readonly<{
  imageFails?: boolean;
  noContext?: boolean;
  noBlob?: boolean;
}>;

/** Drawing is what a stubbed browser recorded of the logos drawn through it. */
export type Drawing = {
  sources: string[];
  canvases: { width: number; height: number }[];
  drawn: { width: number; height: number }[];
  created: string[];
  revoked: string[];
  restore: () => void;
};

/**
 * stubDrawing gives jsdom the parts of a browser that draw an SVG to a PNG:
 * object URLs, an Image that loads (or fails) the moment it has a source, and
 * a canvas whose context records its drawing and whose toBlob answers a PNG.
 * It records the SVG sources it was handed.
 */
export function stubDrawing(options: Options = {}): Drawing {
  const record: Drawing = {
    sources: [],
    canvases: [],
    drawn: [],
    created: [],
    revoked: [],
    restore: () => undefined,
  };
  const blobs = new Map<string, Blob>();
  let serial = 0;
  const before = {
    create: URL.createObjectURL,
    revoke: URL.revokeObjectURL,
  };
  Object.assign(URL, {
    createObjectURL: vi.fn((blob: Blob) => {
      serial += 1;
      const url = `blob:logo-${serial}`;
      blobs.set(url, blob);
      record.created.push(url);
      return url;
    }),
    revokeObjectURL: vi.fn((url: string) => {
      record.revoked.push(url);
    }),
  });

  class DrawnImage {
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    set src(url: string) {
      const blob = blobs.get(url);
      void blob?.text().then((text) => record.sources.push(text));
      queueMicrotask(() => (options.imageFails ? this.onerror?.() : this.onload?.()));
    }
  }
  vi.stubGlobal("Image", DrawnImage);

  const context = {
    drawImage: (
      _image: unknown,
      _x: number,
      _y: number,
      width: number,
      height: number,
    ) => record.drawn.push({ width, height }),
  };
  const getContext = vi
    .spyOn(HTMLCanvasElement.prototype, "getContext")
    .mockImplementation(function (this: HTMLCanvasElement) {
      record.canvases.push({ width: this.width, height: this.height });
      return options.noContext
        ? null
        : (context as unknown as CanvasRenderingContext2D);
    } as never);
  const toBlob = vi
    .spyOn(HTMLCanvasElement.prototype, "toBlob")
    .mockImplementation((callback: BlobCallback, type?: string) => {
      callback(
        options.noBlob ? null : new Blob(["png-bytes"], { type: type ?? "image/png" }),
      );
    });

  record.restore = () => {
    Object.assign(URL, {
      createObjectURL: before.create,
      revokeObjectURL: before.revoke,
    });
    vi.unstubAllGlobals();
    getContext.mockRestore();
    toBlob.mockRestore();
  };
  return record;
}

/** SQUARE_SVG is a logo that draws: 200 x 200 units, a circle. */
export const SQUARE_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200"><circle cx="100" cy="100" r="80"/></svg>';

/** svgFile is an SVG logo file with `body` as its text. */
export function svgFile(body = SQUARE_SVG, name = "logo.svg"): File {
  return new File([body], name, { type: "image/svg+xml" });
}

/** pngFile is a PNG logo file; its bytes are not a real picture, since the server judges those. */
export function pngFile(name = "logo.png", size = 16): File {
  return new File([new Uint8Array(size)], name, { type: "image/png" });
}
