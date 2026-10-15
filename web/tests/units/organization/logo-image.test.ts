import { afterEach, describe, expect, it } from "vitest";
import {
  LOGO_CANVAS,
  LOGO_MAX_BYTES,
  LogoRefused,
  isSvg,
  logoProblem,
  prepareLogo,
  type LogoProblem,
} from "@/features/organization/logoImage";
import { SQUARE_SVG, pngFile, stubDrawing, svgFile, type Drawing } from "./drawing";

let stub: Drawing | undefined;

afterEach(() => {
  stub?.restore();
  stub = undefined;
});

async function refusal(file: File): Promise<LogoProblem | undefined> {
  try {
    await prepareLogo(file);
  } catch (cause) {
    return cause instanceof LogoRefused ? cause.problem : undefined;
  }
  return undefined;
}

describe("logoProblem", () => {
  it("lets a PNG and an SVG be read", () => {
    expect(logoProblem(pngFile())).toBeNull();
    expect(logoProblem(svgFile())).toBeNull();
    expect(logoProblem(new File(["x"], "logo.PNG"))).toBeNull();
    expect(logoProblem(new File(["x"], "logo.SVG"))).toBeNull();
  });

  it("refuses any other type", () => {
    expect(logoProblem(new File(["x"], "logo.jpg", { type: "image/jpeg" }))).toBe(
      "type",
    );
    expect(logoProblem(new File(["x"], "logo.pdf", { type: "application/pdf" }))).toBe(
      "type",
    );
    expect(logoProblem(new File(["x"], "logo"))).toBe("type");
  });

  it("refuses a PNG over 1 MiB but not one of exactly 1 MiB, and never judges an SVG by size", () => {
    expect(logoProblem(pngFile("logo.png", LOGO_MAX_BYTES))).toBeNull();
    expect(logoProblem(pngFile("logo.png", LOGO_MAX_BYTES + 1))).toBe("tooLarge");
    expect(logoProblem(svgFile("x".repeat(LOGO_MAX_BYTES + 1)))).toBeNull();
  });

  it("knows an SVG by its type or by its name", () => {
    expect(isSvg(svgFile())).toBe(true);
    expect(isSvg(new File(["x"], "mark.svg"))).toBe(true);
    expect(isSvg(pngFile())).toBe(false);
  });
});

describe("prepareLogo", () => {
  it("hands a PNG on untouched, for the server to judge", async () => {
    stub = stubDrawing();
    const png = pngFile();

    expect(await prepareLogo(png)).toBe(png);
    expect(stub.created).toEqual([]);
  });

  it("refuses what logoProblem refuses, naming the problem", async () => {
    stub = stubDrawing();

    expect(await refusal(new File(["x"], "logo.gif", { type: "image/gif" }))).toBe(
      "type",
    );
    expect(await refusal(pngFile("big.png", LOGO_MAX_BYTES + 1))).toBe("tooLarge");
  });

  it("draws an SVG on a 1024 x 1024 canvas and answers the PNG that canvas exports", async () => {
    stub = stubDrawing();

    const png = await prepareLogo(svgFile(SQUARE_SVG, "mark.svg"));

    expect(png).toBeInstanceOf(File);
    expect(png.name).toBe("mark.png");
    expect(png.type).toBe("image/png");
    expect(LOGO_CANVAS).toBe(1024);
    expect(stub.canvases).toEqual([{ width: 1024, height: 1024 }]);
    expect(stub.drawn).toEqual([{ width: 1024, height: 1024 }]);
  });

  it("sizes the SVG to the canvas before it is drawn, and lets go of its object URL", async () => {
    stub = stubDrawing();

    await prepareLogo(svgFile());

    expect(stub.sources).toHaveLength(1);
    expect(stub.sources[0]).toContain('width="1024"');
    expect(stub.sources[0]).toContain('height="1024"');
    expect(stub.sources[0]).toContain("<circle");
    expect(stub.revoked).toEqual(stub.created);
  });

  it("takes a square from width and height when there is no viewBox", async () => {
    stub = stubDrawing();
    const svg =
      '<svg xmlns="http://www.w3.org/2000/svg" width="64px" height="64"><rect/></svg>';

    expect((await prepareLogo(svgFile(svg))).type).toBe("image/png");
  });

  it("accepts a shape off square by less than 2% and refuses one off by more", async () => {
    stub = stubDrawing();
    const box = (width: number, height: number) =>
      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}"><rect/></svg>`;

    expect(await refusal(svgFile(box(200, 198)))).toBeUndefined();
    expect(await refusal(svgFile(box(200, 190)))).toBe("notSquare");
    expect(await refusal(svgFile(box(300, 100)))).toBe("notSquare");
  });

  it.each([
    ["is not XML", "<svg"],
    ["is not an SVG", '<html xmlns="http://www.w3.org/1999/xhtml"></html>'],
    ["has no size", '<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>'],
    [
      "has a size in percent",
      '<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%"><rect/></svg>',
    ],
    ["has no namespace", '<svg viewBox="0 0 10 10"><rect/></svg>'],
  ])("refuses an SVG that %s as unreadable", async (_, body) => {
    stub = stubDrawing();

    expect(await refusal(svgFile(body))).toBe("unreadable");
    expect(stub.drawn).toEqual([]);
  });

  it("refuses an SVG the browser cannot draw", async () => {
    stub = stubDrawing({ imageFails: true });

    expect(await refusal(svgFile())).toBe("unreadable");
    expect(stub.revoked).toEqual(stub.created);
  });

  it("refuses when the canvas gives no context", async () => {
    stub = stubDrawing({ noContext: true });

    expect(await refusal(svgFile())).toBe("unreadable");
  });

  it("refuses an SVG whose drawing taints the canvas, as one holding a foreignObject does", async () => {
    stub = stubDrawing({ tainted: true });

    expect(await refusal(svgFile())).toBe("unreadable");
    expect(stub.revoked).toEqual(stub.created);
  });

  it("refuses when the canvas exports nothing", async () => {
    stub = stubDrawing({ noBlob: true });

    expect(await refusal(svgFile())).toBe("unreadable");
    expect(stub.revoked).toEqual(stub.created);
  });
});
