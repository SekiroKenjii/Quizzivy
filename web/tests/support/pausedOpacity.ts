import ts from "typescript";

export interface PausedOpacityUse {
  line: number;
  text: string;
}

const ARIA_DISABLED = /aria-(?:disabled|\[disabled(?:=[^\]]*)?\])|\[&\[aria-disabled/;
const ENDS_IN_OPACITY = /(?:^|[:\]])!?(?:opacity-[^\s:]*|\[opacity:[^\]]*\])$/;
const FULL_OPACITY = /(?:^|[:\]])!?opacity-100!?$/;
const ON_DESCENDANT = /(?:^|:)\*{1,2}:|\[&[^\]>_~+\s]*[>_~+\s]/;

interface Literal {
  text: string;
  start: number;
}

function literals(node: ts.Node, file: ts.SourceFile): Literal[] {
  const found: Literal[] = [];
  const walk = (n: ts.Node) => {
    if (
      ts.isStringLiteralLike(n) ||
      ts.isTemplateHead(n) ||
      ts.isTemplateMiddle(n) ||
      ts.isTemplateTail(n)
    )
      found.push({ text: n.text, start: n.getStart(file) + 1 });
    ts.forEachChild(n, walk);
  };
  walk(node);
  return found;
}

function tokens(literal: Literal): { token: string; at: number }[] {
  return [...literal.text.matchAll(/\S+/g)].map((match) => ({
    token: match[0],
    at: literal.start + match.index,
  }));
}

function dimsTheElement(token: string): boolean {
  return (
    ENDS_IN_OPACITY.test(token) &&
    !FULL_OPACITY.test(token) &&
    !ON_DESCENDANT.test(token)
  );
}

function lineAt(file: ts.SourceFile, position: number): number {
  return file.getLineAndCharacterOfPosition(position).line + 1;
}

function inTokens(file: ts.SourceFile): PausedOpacityUse[] {
  return literals(file, file).flatMap((literal) =>
    tokens(literal).flatMap(({ token, at }) =>
      ARIA_DISABLED.test(token) && dimsTheElement(token)
        ? [{ line: lineAt(file, at), text: token }]
        : [],
    ),
  );
}

function attribute(
  element: ts.JsxOpeningLikeElement,
  name: string,
): ts.JsxAttribute | undefined {
  return element.attributes.properties.find(
    (p): p is ts.JsxAttribute => ts.isJsxAttribute(p) && p.name.getText() === name,
  );
}

function constants(file: ts.SourceFile): Map<string, ts.Expression> {
  const found = new Map<string, ts.Expression>();
  const walk = (n: ts.Node) => {
    if (ts.isVariableDeclarationList(n) && (n.flags & ts.NodeFlags.Const) !== 0)
      for (const declaration of n.declarations)
        if (ts.isIdentifier(declaration.name) && declaration.initializer)
          found.set(declaration.name.text, declaration.initializer);
    ts.forEachChild(n, walk);
  };
  walk(file);
  return found;
}

function referenced(node: ts.Node, known: Map<string, ts.Expression>): ts.Expression[] {
  const found: ts.Expression[] = [];
  const walk = (n: ts.Node) => {
    if (ts.isIdentifier(n)) {
      const value = known.get(n.text);
      if (value) found.push(value);
    }
    ts.forEachChild(n, walk);
  };
  walk(node);
  return found;
}

function plainOpacity(token: string): boolean {
  return /^!?opacity-/.test(token) && dimsTheElement(token);
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

function onPausedElements(file: ts.SourceFile): PausedOpacityUse[] {
  const known = constants(file);
  const found: PausedOpacityUse[] = [];
  const visit = (node: ts.Node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      if (attribute(node, "aria-disabled")) found.push(...opacityOn(node, file, known));
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return found;
}

function classOpacity(
  className: ts.JsxAttribute,
  file: ts.SourceFile,
  known: Map<string, ts.Expression>,
): PausedOpacityUse[] {
  if (!className.initializer) return [];
  const line = lineAt(file, className.getStart(file));
  const sources = [className.initializer, ...referenced(className.initializer, known)];
  return sources.flatMap((source) =>
    literals(source, file).flatMap((literal) =>
      tokens(literal)
        .filter(({ token }) => plainOpacity(token))
        .map(({ token }) => ({ line, text: token })),
    ),
  );
}

function opacityOn(
  element: ts.JsxOpeningLikeElement,
  file: ts.SourceFile,
  known: Map<string, ts.Expression>,
): PausedOpacityUse[] {
  const className = attribute(element, "className");
  const out = className ? classOpacity(className, file, known) : [];
  const style = attribute(element, "style");
  if (style && styleSetsOpacity(style))
    out.push({ line: lineAt(file, style.getStart(file)), text: "style opacity" });
  return out;
}

/**
 * pausedOpacity lists the places in a TSX source that dim an aria-disabled
 * control with opacity: a class token in a string that holds aria-disabled and
 * ends in an opacity utility, in any spelling, and an opacity utility or style
 * on an element that declares aria-disabled, its class read through one hop of
 * a constant in the same file. Comments, `opacity-100` and an opacity aimed at
 * the control's children are not uses. A class built in another module is out
 * of reach.
 */
export function pausedOpacity(
  source: string,
  fileName = "file.tsx",
): PausedOpacityUse[] {
  const file = ts.createSourceFile(fileName, source, ts.ScriptTarget.Latest, true);
  return [...inTokens(file), ...onPausedElements(file)].sort((a, b) => a.line - b.line);
}
