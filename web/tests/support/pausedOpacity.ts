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

type Entries = Map<string, ts.Expression | undefined>;

function entryName(name: ts.Node): string {
  return ts.isStringLiteral(name) || ts.isIdentifier(name) ? name.text : name.getText();
}

function attributes(element: ts.JsxOpeningLikeElement): Entries {
  const out: Entries = new Map();
  for (const p of element.attributes.properties) {
    if (!ts.isJsxAttribute(p)) continue;
    const value = p.initializer;
    out.set(
      p.name.getText(),
      value && ts.isJsxExpression(value) ? value.expression : value,
    );
  }
  return out;
}

function properties(object: ts.ObjectLiteralExpression): Entries {
  const out: Entries = new Map();
  for (const p of object.properties)
    if (ts.isPropertyAssignment(p)) out.set(entryName(p.name), p.initializer);
  return out;
}

function entries(node: ts.Node): Entries | null {
  if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node))
    return attributes(node);
  return ts.isObjectLiteralExpression(node) ? properties(node) : null;
}

function pausedBy(found: Entries): string | null {
  if (!found.has("aria-disabled")) return null;
  const value = found.get("aria-disabled");
  if (value === undefined) return "true";
  if (
    value.kind === ts.SyntaxKind.FalseKeyword ||
    /^["']false["']$/.test(value.getText())
  )
    return null;
  const text = value.getText();
  const bare = text.endsWith("undefined")
    ? text.slice(0, text.lastIndexOf("||")).trimEnd()
    : text;
  return /^["']true["']$/.test(bare) ? "true" : bare;
}

function declared(statements: ts.NodeArray<ts.Statement>, name: string) {
  for (const statement of statements) {
    if (!ts.isVariableStatement(statement)) continue;
    if ((statement.declarationList.flags & ts.NodeFlags.Const) === 0) continue;
    const declaration = statement.declarationList.declarations.find(
      (d) => ts.isIdentifier(d.name) && d.name.text === name,
    );
    if (declaration) return declaration.initializer;
  }
  return undefined;
}

function constantNamed(name: string, from: ts.Node): ts.Expression | undefined {
  for (let scope: ts.Node | undefined = from.parent; scope; scope = scope.parent) {
    if (ts.isBlock(scope) || ts.isSourceFile(scope) || ts.isModuleBlock(scope)) {
      const value = declared(scope.statements, name);
      if (value) return value;
    }
  }
  return undefined;
}

function reachable(node: ts.Node, paused: string): ts.Node[] {
  const found: ts.Node[] = [];
  const walk = (n: ts.Node, hops: number) => {
    if (ts.isConditionalExpression(n)) {
      const condition = n.condition.getText();
      if (condition === paused) return walk(n.whenTrue, hops);
      if (condition === `!${paused}` || condition === `!(${paused})`)
        return walk(n.whenFalse, hops);
    }
    found.push(n);
    if (ts.isIdentifier(n) && hops > 0) {
      const value = constantNamed(n.text, n);
      if (value) walk(value, hops - 1);
    }
    ts.forEachChild(n, (child) => walk(child, hops));
  };
  walk(node, 1);
  return found;
}

function classOpacity(value: ts.Node, paused: string, file: ts.SourceFile): string[] {
  return reachable(value, paused).flatMap((n) =>
    ts.isStringLiteralLike(n) ||
    ts.isTemplateHead(n) ||
    ts.isTemplateMiddle(n) ||
    ts.isTemplateTail(n)
      ? tokens({ text: n.text, start: n.getStart(file) })
          .map(({ token }) => token)
          .filter(plainOpacity)
      : [],
  );
}

function styleSetsOpacity(value: ts.Node, paused: string): boolean {
  return reachable(value, paused).some(
    (n) =>
      ((ts.isPropertyAssignment(n) || ts.isShorthandPropertyAssignment(n)) &&
        entryName(n.name) === "opacity") ||
      (ts.isElementAccessExpression(n) &&
        /^["']opacity["']$/.test(n.argumentExpression.getText())),
  );
}

function plainOpacity(token: string): boolean {
  return /^!?opacity-/.test(token) && dimsTheElement(token);
}

function onPausedElements(file: ts.SourceFile): PausedOpacityUse[] {
  const found: PausedOpacityUse[] = [];
  const visit = (node: ts.Node) => {
    const carried = entries(node);
    const paused = carried && pausedBy(carried);
    if (carried && paused !== null)
      found.push(...opacityOn(node, carried, paused, file));
    ts.forEachChild(node, visit);
  };
  visit(file);
  return found;
}

function opacityOn(
  carrier: ts.Node,
  carried: Entries,
  paused: string,
  file: ts.SourceFile,
): PausedOpacityUse[] {
  const line = lineAt(file, carrier.getStart(file));
  const className = carried.get("className");
  const out = className
    ? classOpacity(className, paused, file).map((text) => ({ line, text }))
    : [];
  const style = carried.get("style");
  if (style && styleSetsOpacity(style, paused))
    out.push({ line, text: "style opacity" });
  return out;
}

/**
 * pausedOpacity lists the places in a TSX source that dim an aria-disabled
 * control with opacity: a class token in a string that holds aria-disabled and
 * ends in an opacity utility, in any spelling, and an opacity utility or style
 * on an element or a props object that declares aria-disabled (not `false`),
 * its class and style read through one hop of a constant in scope and only the
 * paused branch of a ternary on the same condition. Comments, `opacity-100` and
 * an opacity aimed at the control's children are not uses. A class built in
 * another module, two hops of constants and an ancestor's opacity are out of
 * reach.
 */
export function pausedOpacity(
  source: string,
  fileName = "file.tsx",
): PausedOpacityUse[] {
  const file = ts.createSourceFile(fileName, source, ts.ScriptTarget.Latest, true);
  return [...inTokens(file), ...onPausedElements(file)].sort((a, b) => a.line - b.line);
}
