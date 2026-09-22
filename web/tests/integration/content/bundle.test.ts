import { resolve } from "node:path";
import { gzipSync } from "node:zlib";
import { build } from "vite";

type Chunk = {
  type: "chunk";
  name: string;
  fileName: string;
  imports: string[];
  dynamicImports: string[];
  moduleIds: string[];
  code: string;
};
type Output = { output: (Chunk | { type: "asset" })[] };

function closure(entry: Chunk, chunks: Map<string, Chunk>): Chunk[] {
  const seen = new Set<string>();
  const pending = [entry];
  const result: Chunk[] = [];
  while (pending.length) {
    const chunk = pending.pop()!;
    if (seen.has(chunk.fileName)) continue;
    seen.add(chunk.fileName);
    result.push(chunk);
    for (const name of [...chunk.imports, ...chunk.dynamicImports]) {
      const dependency = chunks.get(name);
      if (dependency) pending.push(dependency);
    }
  }
  return result;
}

test("keeps editor modules out of the reader and pins prototype transfer budgets", async () => {
  const result = (await build({
    configFile: resolve(
      import.meta.dirname,
      "../../support/content-editor/vite.config.ts",
    ),
    logLevel: "silent",
    define: { "process.env.NODE_ENV": JSON.stringify("production") },
    build: { write: false },
  })) as unknown as Output;
  const chunks = result.output.filter((item): item is Chunk => item.type === "chunk");
  const byName = new Map(chunks.map((chunk) => [chunk.fileName, chunk]));
  const reader = closure(
    chunks.find((chunk) => chunk.name === "reader")!,
    byName,
  );
  const editor = closure(
    chunks.find((chunk) => chunk.name === "editor")!,
    byName,
  );
  expect(
    reader
      .flatMap((chunk) => chunk.moduleIds)
      .filter((id) => /@tiptap|prosemirror/.test(id)),
  ).toEqual([]);
  expect(
    editor.flatMap((chunk) => chunk.moduleIds).some((id) => id.includes("@tiptap")),
  ).toBe(true);
  const shared = new Set(reader.map((chunk) => chunk.fileName));
  expect(
    editor
      .filter((chunk) => !shared.has(chunk.fileName))
      .reduce((total, chunk) => total + gzipSync(chunk.code).byteLength, 0),
  ).toBeLessThan(160 * 1024);
  expect(
    reader.reduce((total, chunk) => total + gzipSync(chunk.code).byteLength, 0),
  ).toBeLessThan(220 * 1024);
}, 60_000);
