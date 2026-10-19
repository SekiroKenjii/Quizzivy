import { afterEach, describe, expect, it, vi } from "vitest";
import {
  AVATAR_MAX_SIDE,
  fitWithin,
  prepareAvatar,
} from "@/features/settings/avatarImage";

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function decoded(width: number, height: number) {
  const close = vi.fn();
  vi.stubGlobal(
    "createImageBitmap",
    vi.fn().mockResolvedValue({ width, height, close }),
  );
  return close;
}

function canvasThatDraws(blob: Blob | null) {
  const drawImage = vi.fn();
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({
    drawImage,
  } as unknown as CanvasRenderingContext2D);
  const toBlob = vi
    .spyOn(HTMLCanvasElement.prototype, "toBlob")
    .mockImplementation((callback) => callback(blob));
  return { drawImage, toBlob };
}

const photo = (type = "image/jpeg") =>
  new File([new Uint8Array(3 * 1024 * 1024)], "me.jpg", { type });

describe("fitWithin", () => {
  it.each([
    [4032, 3024, { width: 2048, height: 1536 }],
    [3024, 4032, { width: 1536, height: 2048 }],
    [5000, 100, { width: 2048, height: 41 }],
  ])("scales %i by %i to fit the longest side", (width, height, expected) => {
    expect(fitWithin(width, height, AVATAR_MAX_SIDE)).toEqual(expected);
  });

  it.each([
    [2048, 2048],
    [200, 200],
    [2048, 300],
  ])("leaves %i by %i alone", (width, height) => {
    expect(fitWithin(width, height, AVATAR_MAX_SIDE)).toBeNull();
  });
});

describe("prepareAvatar", () => {
  it("draws a phone photo at 2048 by 1536 in its own type", async () => {
    const close = decoded(4032, 3024);
    const { drawImage, toBlob } = canvasThatDraws(
      new Blob([new Uint8Array(900)], { type: "image/jpeg" }),
    );
    const file = photo();

    const prepared = await prepareAvatar(file);

    expect(drawImage).toHaveBeenCalledWith(expect.anything(), 0, 0, 2048, 1536);
    expect(toBlob).toHaveBeenCalledWith(expect.any(Function), "image/jpeg", 0.92);
    expect(prepared).not.toBe(file);
    expect(prepared.name).toBe("me.jpg");
    expect(prepared.type).toBe("image/jpeg");
    expect(prepared.size).toBe(900);
    expect(close).toHaveBeenCalledTimes(1);
  });

  it("returns a photo that already fits, without drawing it", async () => {
    const close = decoded(1200, 900);
    const { drawImage } = canvasThatDraws(null);
    const file = photo();

    expect(await prepareAvatar(file)).toBe(file);
    expect(drawImage).not.toHaveBeenCalled();
    expect(close).toHaveBeenCalledTimes(1);
  });

  it("returns the file when the browser cannot decode it", async () => {
    vi.stubGlobal("createImageBitmap", vi.fn().mockRejectedValue(new Error("bad")));
    const file = photo();

    expect(await prepareAvatar(file)).toBe(file);
  });

  it("returns the file when there is no createImageBitmap or no canvas", async () => {
    vi.stubGlobal("createImageBitmap", undefined);
    const file = photo();
    expect(await prepareAvatar(file)).toBe(file);

    const close = decoded(4032, 3024);
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
    expect(await prepareAvatar(file)).toBe(file);
    expect(close).toHaveBeenCalledTimes(1);
  });

  it("returns the file when the canvas cannot encode it", async () => {
    decoded(4032, 3024);
    canvasThatDraws(null);
    const file = photo();

    expect(await prepareAvatar(file)).toBe(file);
  });
});
