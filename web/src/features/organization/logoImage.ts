/** LOGO_TYPES are the file types the logo dialog takes: a PNG, or an SVG it draws to one. */
export const LOGO_TYPES = ["image/png", "image/svg+xml"] as const;

/** LOGO_MAX_BYTES is the largest PNG `POST /admin/settings/logo` accepts: 1 MiB. */
export const LOGO_MAX_BYTES = 1024 * 1024;

/** LOGO_CANVAS is the side, in pixels, of the PNG an SVG is drawn to (D-019). */
export const LOGO_CANVAS = 1024;

const SQUARE_WITHIN = 0.02;
const SVG_NAMESPACE = "http://www.w3.org/2000/svg";

/**
 * LogoProblem names why a logo file is refused before it is sent: its type
 * is neither PNG nor SVG, a PNG is over 1 MiB, an SVG does not draw, or an
 * SVG is not square within 2%, as the server asks of a PNG.
 */
export type LogoProblem = "type" | "tooLarge" | "unreadable" | "notSquare";

/** LogoRefused is thrown by prepareLogo with the problem it found. */
export class LogoRefused extends Error {
  readonly problem: LogoProblem;

  constructor(problem: LogoProblem) {
    super(problem);
    this.name = "LogoRefused";
    this.problem = problem;
  }
}

/** isSvg reports whether a file is an SVG by its type, else by its name. */
export function isSvg(file: File): boolean {
  return file.type === "image/svg+xml" || /\.svg$/i.test(file.name);
}

function isPng(file: File): boolean {
  return file.type === "image/png" || /\.png$/i.test(file.name);
}

/**
 * logoProblem is what is wrong with `file` that can be told without reading
 * it: its type, and the size of a PNG. It is null for a file worth reading.
 */
export function logoProblem(file: File): LogoProblem | null {
  if (isSvg(file)) return null;
  if (!isPng(file)) return "type";
  return file.size > LOGO_MAX_BYTES ? "tooLarge" : null;
}

function measure(value: string | null): number | null {
  if (value === null || value.trim().endsWith("%")) return null;
  const parsed = Number.parseFloat(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function aspectOf(root: Element): number | null {
  const box = root
    .getAttribute("viewBox")
    ?.trim()
    .split(/[\s,]+/);
  const [wide, high] =
    box?.length === 4
      ? [box[2], box[3]]
      : [root.getAttribute("width"), root.getAttribute("height")];
  const width = measure(wide ?? null);
  const height = measure(high ?? null);
  return width === null || height === null ? null : width / height;
}

function squareSvg(text: string): string {
  const document = new DOMParser().parseFromString(text, "image/svg+xml");
  const root = document.documentElement;
  if (
    document.querySelector("parsererror") !== null ||
    root.localName !== "svg" ||
    root.namespaceURI !== SVG_NAMESPACE
  )
    throw new LogoRefused("unreadable");
  const aspect = aspectOf(root);
  if (aspect === null) throw new LogoRefused("unreadable");
  if (Math.abs(aspect - 1) > SQUARE_WITHIN) throw new LogoRefused("notSquare");
  root.setAttribute("width", String(LOGO_CANVAS));
  root.setAttribute("height", String(LOGO_CANVAS));
  return new XMLSerializer().serializeToString(root);
}

function loaded(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new LogoRefused("unreadable"));
    image.src = url;
  });
}

async function drawn(svg: string): Promise<Blob> {
  const url = URL.createObjectURL(new Blob([svg], { type: "image/svg+xml" }));
  try {
    const image = await loaded(url);
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = LOGO_CANVAS;
    const context = canvas.getContext("2d");
    if (context === null) throw new LogoRefused("unreadable");
    context.drawImage(image, 0, 0, LOGO_CANVAS, LOGO_CANVAS);
    return await new Promise<Blob>((resolve, reject) => {
      try {
        canvas.toBlob(
          (png) =>
            png === null ? reject(new LogoRefused("unreadable")) : resolve(png),
          "image/png",
        );
      } catch {
        reject(new LogoRefused("unreadable"));
      }
    });
  } finally {
    URL.revokeObjectURL(url);
  }
}

/**
 * prepareLogo makes `file` something `POST /admin/settings/logo` accepts. A
 * PNG goes as it is, for the server to judge its size and shape. An SVG is
 * drawn, in this browser, on a 1024 x 1024 canvas and sent as the PNG that
 * canvas exports, because the server reads no SVG (D-019); it must parse,
 * draw, and be square within 2%. It throws a LogoRefused otherwise.
 */
export async function prepareLogo(file: File): Promise<File> {
  const problem = logoProblem(file);
  if (problem !== null) throw new LogoRefused(problem);
  if (!isSvg(file)) return file;
  const png = await drawn(squareSvg(await file.text()));
  const name = file.name.replace(/\.svg$/i, "") || "logo";
  return new File([png], `${name}.png`, { type: "image/png" });
}
