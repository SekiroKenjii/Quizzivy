function outsideMarkup(html: string): string {
  let text = "";
  let from = 0;
  while (from < html.length) {
    const open = html.indexOf("<", from);
    if (open < 0) return text + html.slice(from);
    text += html.slice(from, open);
    const comment = html.startsWith("<!--", open);
    const close = comment ? html.indexOf("-->", open + 4) : html.indexOf(">", open + 1);
    if (close < 0) return text;
    from = close + (comment ? 3 : 1);
  }
  return text;
}

/** clipboardHasText reports whether copied HTML holds any text outside its tags and comments, without parsing it; a copied image alone holds none. */
export function clipboardHasText(html: string): boolean {
  return (
    outsideMarkup(html)
      .replace(/&(?:nbsp|#(?:160|xa0));/gi, " ")
      .trim().length > 0
  );
}
