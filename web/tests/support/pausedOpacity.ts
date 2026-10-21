import ts from "typescript";

export interface PausedOpacityUse {
  line: number;
  text: string;
}

const TOKEN = /[^\s"'`]+/g;
const ARIA_DISABLED = /aria-(?:disabled|\[disabled(?:=[^\]]*)?\])|\[&\[aria-disabled/;
const ENDS_IN_OPACITY = /(?:^|[:\]])!?(?:opacity-[^\s:]*|\[opacity:[^\]]*\])$/;

function lineOf(source: string, index: number): number {
  let line = 1;
  for (let i = 0; i < index; i++) if (source.charCodeAt(i) === 10) line++;
  return line;
}

function inTokens(source: string): PausedOpacityUse[] {
  return [...source.matchAll(TOKEN)].flatMap((match) => {
    const token = match[0];
    return ARIA_DISABLED.test(token) && ENDS_IN_OPACITY.test(token)
      ? [{ line: lineOf(source, match.index), text: token }]
      : [];
  });
}

function attribute(
  element: ts.JsxOpeningLikeElement,
  name: string,
): ts.JsxAttribute | undefined {
  return element.attributes.properties.find(
    (p): p is ts.JsxAttribute => ts.isJsxAttribute(p) && p.name.getText() === name,
  );
}

function literals(node: ts.Node): string[] {
  const found: string[] = [];
  const walk = (n: ts.Node) => {
    if (
      ts.isStringLiteralLike(n) ||
      ts.isTemplateHead(n) ||
      ts.isTemplateMiddle(n) ||
      ts.isTemplateTail(n)
    )
      found.push(n.text);
    ts.forEachChild(n, walk);
  };
  walk(node);
  return found;
}

function plainOpacity(token: string): boolean {
  return /^!?opacity-/.test(token);
}

function styleSetsOpacity(style: ts.JsxAttribute): boolean {
  let sets = false;
  const walk = (n: ts.Node) => {
    if (ts.isPropertyAssignment(n) && n.name.getText() === "opacity") sets = true;
    ts.forEachChild(n, walk);
  };
  if (style.initializer) walk(style.initializer);
  return sets;
}

function onPausedElements(source: string, fileName: string): PausedOpacityUse[] {
  const file = ts.createSourceFile(fileName, source, ts.ScriptTarget.Latest, true);
  const found: PausedOpacityUse[] = [];
  const visit = (node: ts.Node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      if (attribute(node, "aria-disabled")) found.push(...opacityOn(node, file));
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return found;
}

function opacityOn(
  element: ts.JsxOpeningLikeElement,
  file: ts.SourceFile,
): PausedOpacityUse[] {
  const out: PausedOpacityUse[] = [];
  const at = (node: ts.Node, text: string) => ({
    line: file.getLineAndCharacterOfPosition(node.getStart(file)).line + 1,
    text,
  });
  const className = attribute(element, "className");
  if (className?.initializer) {
    for (const text of literals(className.initializer))
      for (const token of text.split(/\s+/))
        if (plainOpacity(token)) out.push(at(className, token));
  }
  const style = attribute(element, "style");
  if (style && styleSetsOpacity(style)) out.push(at(style, "style opacity"));
  return out;
}

/**
 * pausedOpacity lists the places in a TSX source that dim an aria-disabled
 * control with opacity: a class token that holds aria-disabled and ends in an
 * opacity utility, in any spelling, and an opacity utility or style on an element
 * that declares aria-disabled. A class built in another module is out of reach.
 */
export function pausedOpacity(
  source: string,
  fileName = "file.tsx",
): PausedOpacityUse[] {
  return [...inTokens(source), ...onPausedElements(source, fileName)].sort(
    (a, b) => a.line - b.line,
  );
}
