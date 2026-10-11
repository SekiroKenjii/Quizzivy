/** QR_PAPER is the QR's background, the paper token, resolved when it is saved. */
export const QR_PAPER = "var(--paper)";
/** QR_INK is the QR's modules, the paper's ink token, resolved when it is saved. */
export const QR_INK = "var(--paper-fg)";

/**
 * downloadJoinQr saves the QR drawn in `host` as `quizzivy-{code}.png`, 512
 * pixels square on paper white, for slides or a Zalo group, with the tokens
 * the on-screen QR reads resolved to colours. Where the browser has no
 * canvas it saves the SVG instead.
 */
export function downloadJoinQr(host: HTMLElement | null, code: string) {
  const svg = host?.querySelector("svg");
  if (!svg) return;
  const style = getComputedStyle(document.documentElement);
  const paper = style.getPropertyValue("--paper").trim();
  const ink = style.getPropertyValue("--paper-fg").trim();
  const markup = new XMLSerializer()
    .serializeToString(svg)
    .replaceAll(QR_PAPER, paper)
    .replaceAll(QR_INK, ink);
  const svgBlob = new Blob([markup], { type: "image/svg+xml" });
  const filename = `quizzivy-${code}.png`;
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 512;
  const context = canvas.getContext("2d");
  if (!context) {
    save(svgBlob, filename.replace(/\.png$/, ".svg"));
    return;
  }
  const image = new Image();
  const url = URL.createObjectURL(svgBlob);
  image.onload = () => {
    context.fillStyle = paper;
    context.fillRect(0, 0, canvas.width, canvas.height);
    context.drawImage(image, 32, 32, 448, 448);
    URL.revokeObjectURL(url);
    canvas.toBlob((png) => png && save(png, filename), "image/png");
  };
  image.src = url;
}

function save(blob: Blob, filename: string) {
  const anchor = document.createElement("a");
  anchor.href = URL.createObjectURL(blob);
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(anchor.href);
}
