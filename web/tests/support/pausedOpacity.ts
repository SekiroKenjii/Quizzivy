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

function spreadObject(p: ts.JsxSpreadAttribute): ts.ObjectLiteralExpression | null {
  let e: ts.Expression = p.expression;
  while (ts.isParenthesizedExpression(e)) e = e.expression;
  return ts.isObjectLiteralExpression(e) ? e : null;
}

function attributes(element: ts.JsxOpeningLikeElement): Entries {
  const out: Entries = new Map();
  for (const p of element.attributes.properties) {
    if (ts.isJsxSpreadAttribute(p)) {
      const spread = spreadObject(p);
      if (spread) for (const [k, v] of properties(spread)) out.set(k, v);
      continue;
    }
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
  for (const p of object.properties) {
    if (ts.isPropertyAssignment(p)) out.set(entryName(p.name), p.initializer);
    if (ts.isShorthandPropertyAssignment(p)) out.set(p.name.text, p.name);
  }
  return out;
}

function spreadInto(object: ts.Node): boolean {
  let up = object.parent;
  while (ts.isParenthesizedExpression(up)) up = up.parent;
  return ts.isJsxSpreadAttribute(up);
}

function entries(node: ts.Node): Entries | null {
  if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node))
    return attributes(node);
  return ts.isObjectLiteralExpression(node) && !spreadInto(node)
    ? properties(node)
    : null;
}

const isUndefined = (e: ts.Expression) => ts.isIdentifier(e) && e.text === "undefined";
const isFalse = (e: ts.Expression) =>
  e.kind === ts.SyntaxKind.FalseKeyword || /^["']false["']$/.test(e.getText());
const isTrue = (e: ts.Expression) =>
  e.kind === ts.SyntaxKind.TrueKeyword || /^["']true["']$/.test(e.getText());

function condition(value: ts.Expression): string | null {
  if (isFalse(value) || isUndefined(value)) return null;
  if (isTrue(value)) return "true";
  if (
    ts.isBinaryExpression(value) &&
    value.operatorToken.kind === ts.SyntaxKind.BarBarToken &&
    isUndefined(value.right)
  )
    return condition(value.left);
  if (ts.isConditionalExpression(value)) {
    const off = (e: ts.Expression) => isUndefined(e) || isFalse(e);
    if (isTrue(value.whenTrue) && off(value.whenFalse))
      return value.condition.getText();
    if (off(value.whenTrue) && isTrue(value.whenFalse))
      return negated(value.condition.getText());
  }
  return value.getText();
}

function negated(text: string): string {
  if (text.startsWith("!(") && text.endsWith(")")) return text.slice(2, -1);
  return text.startsWith("!") ? text.slice(1) : `!${text}`;
}

function pausedBy(found: Entries): string | null {
  if (!found.has("aria-disabled")) return null;
  const value = found.get("aria-disabled");
  return value === undefined ? "true" : condition(value);
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

function shadows(scope: ts.Node, name: string): boolean {
  return (
    ts.isFunctionLike(scope) &&
    scope.parameters.some((p) => ts.isIdentifier(p.name) && p.name.text === name)
  );
}

function constantNamed(name: string, from: ts.Node): ts.Expression | undefined {
  for (let scope: ts.Node | undefined = from.parent; scope; scope = scope.parent) {
    if (shadows(scope, name)) return undefined;
    if (ts.isBlock(scope) || ts.isSourceFile(scope) || ts.isModuleBlock(scope)) {
      const value = declared(scope.statements, name);
      if (value) return value;
    }
  }
  return undefined;
}

function pausedBranch(n: ts.Node, paused: string): ts.Node | null | undefined {
  const off = new Set([negated(paused), `!(${paused})`]);
  if (ts.isConditionalExpression(n)) {
    const test = n.condition.getText();
    if (test === paused) return n.whenTrue;
    if (off.has(test)) return n.whenFalse;
  }
  if (
    ts.isBinaryExpression(n) &&
    n.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken &&
    off.has(n.left.getText())
  )
    return null;
  return undefined;
}

function reachable(node: ts.Node, paused: string): ts.Node[] {
  const found: ts.Node[] = [];
  const walk = (n: ts.Node, hops: number) => {
    const branch = pausedBranch(n, paused);
    if (branch === null) return;
    if (branch) return walk(branch, hops);
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

const UNCONDITIONAL =
  /^(?:sm|md|lg|xl|2xl|dark|print|engine-lg|motion-safe|motion-reduce|(?:min|max)-\[[^\]]*\])$/;

function variants(token: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < token.length; i++) {
    if (token[i] === "[") depth++;
    else if (token[i] === "]") depth--;
    else if (token[i] === ":" && depth === 0) {
      parts.push(token.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(token.slice(start));
  return parts;
}

function plainOpacity(token: string): boolean {
  const parts = variants(token);
  const utility = parts.pop() ?? "";
  if (!parts.every((variant) => UNCONDITIONAL.test(variant))) return false;
  return /^!?(?:opacity-|\[opacity:)/.test(utility) && dimsTheElement(token);
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
 * (bare or behind a breakpoint or theme variant) on an element, its spread or
 * a props object that declares aria-disabled (not `false` or `undefined`), its
 * class and style read through one hop of a constant in scope, skipping the
 * unpaused branch of a ternary or `&&` on the same condition. Comments,
 * `opacity-100` and an opacity aimed at the control's children are not uses. A class built in
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
