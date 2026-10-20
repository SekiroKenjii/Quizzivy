/** AVATAR_MAX_SIDE is the longest side `PUT /me/avatar` accepts, in pixels. */
export const AVATAR_MAX_SIDE = 2048;

const JPEG_QUALITY = 0.92;

/**
 * fitWithin is the size that brings an image of `width` by `height` inside a
 * square of `max` pixels, keeping its proportions, or null when it already
 * fits.
 */
export function fitWithin(
  width: number,
  height: number,
  max: number,
): { width: number; height: number } | null {
  const longest = Math.max(width, height);
  if (longest <= max) return null;
  const scale = max / longest;
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  };
}

/**
 * prepareAvatar returns `file` scaled down on a canvas so that no side is over
 * AVATAR_MAX_SIDE, in the type it came in, with the camera's rotation applied.
 * A file already inside the limit, one the browser cannot decode and a browser
 * without a canvas come back as they were, for the server to judge.
 */
export async function prepareAvatar(file: File): Promise<File> {
  if (typeof createImageBitmap !== "function") return file;
  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file);
  } catch {
    return file;
  }
  try {
    const size = fitWithin(bitmap.width, bitmap.height, AVATAR_MAX_SIDE);
    if (size === null) return file;
    const canvas = document.createElement("canvas");
    canvas.width = size.width;
    canvas.height = size.height;
    const context = canvas.getContext("2d");
    if (context === null) return file;
    context.drawImage(bitmap, 0, 0, size.width, size.height);
    const blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, file.type, JPEG_QUALITY),
    );
    return blob === null ? file : new File([blob], file.name, { type: file.type });
  } catch {
    return file;
  } finally {
    bitmap.close();
  }
}
