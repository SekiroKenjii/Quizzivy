const RAW = ["style", "script", "title"];

function rawElement(html: string, open: number): string | undefined {
  return RAW.find(
    (name) =>
      html.slice(open + 1, open + 1 + name.length).toLowerCase() === name &&
      /[\s/>]/.test(html[open + 1 + name.length] ?? ""),
  );
}

function closingTag(html: string, name: string, from: number): number {
  const pattern = new RegExp(`</${name}`, "gi");
  pattern.lastIndex = from;
  return pattern.exec(html)?.index ?? -1;
}

function markupEnd(html: string, open: number): number {
  if (html.startsWith("<!--", open)) {
    const close = html.indexOf("-->", open + 4);
    return close < 0 ? -1 : close + 3;
  }
  const raw = rawElement(html, open);
  const close = raw ? closingTag(html, raw, open + 1) : open;
  const end = close < 0 ? -1 : html.indexOf(">", close + 1);
  return end < 0 ? -1 : end + 1;
}

function outsideMarkup(html: string): string {
  let text = "";
  let from = 0;
  while (from < html.length) {
    const open = html.indexOf("<", from);
    if (open < 0) return text + html.slice(from);
    text += html.slice(from, open);
    from = markupEnd(html, open);
    if (from < 0) return text;
  }
  return text;
}

/** clipboardHasText reports whether copied HTML shows any text outside its tags, comments and its style, script and title elements, without parsing it; a copied image alone shows none. */
export function clipboardHasText(html: string): boolean {
  return (
    outsideMarkup(html)
      .replace(/&(?:nbsp|#(?:160|xa0));/gi, " ")
      .trim().length > 0
  );
}
