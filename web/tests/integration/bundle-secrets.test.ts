import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { build } from "vite";
import { beforeAll, describe, expect, it } from "vitest";

type OutputChunk = { type: "chunk"; fileName: string; code: string };
type OutputAsset = { type: "asset"; fileName: string; source: string | Uint8Array };
type BuildOutput = { output: (OutputChunk | OutputAsset)[] };

/**
 * Vite's `envDir` points at the repository root, so the frontend build reads a
 * directory whose `.env` holds the database superuser password, the JWT signing
 * key, the Google client secret and two sets of object-storage credentials.
 */

const ROOT = resolve(import.meta.dirname, "../..");
const REPO = resolve(ROOT, "..");

/** Names that must never be inlined, whatever they are set to. */
const SERVER_ONLY = [
  "POSTGRES_SUPERUSER_PASSWORD",
  "QUIZZIVY_MIGRATE_PASSWORD",
  "QUIZZIVY_APP_PASSWORD",
  "DATABASE_URL",
  "NEON_ADMIN_URL",
  "NEON_MIGRATE_URL",
  "NEON_APP_URL",
  "JWT_SIGNING_KEY",
  "GOOGLE_CLIENT_SECRET",
  "S3_SECRET_ACCESS_KEY",
  "R2_SECRET_ACCESS_KEY",
];

/** Values from the developer's real .env, when there is one to read. */
function secretValues(): { name: string; value: string }[] {
  let raw: string;
  try {
    raw = readFileSync(resolve(REPO, ".env"), "utf8");
  } catch {
    return [];
  }

  const out: { name: string; value: string }[] = [];
  for (const line of raw.split("\n")) {
    const match = /^([A-Z_][A-Z0-9_]*)=(.*)$/.exec(line.trim());
    if (!match) continue;
    const [, name, rawValue] = match;
    if (!name || !SERVER_ONLY.includes(name)) continue;
    const value = (rawValue ?? "").trim().replace(/^["']|["']$/g, "");
    if (value.length >= 8) out.push({ name, value });
  }
  return out;
}

let bundle = "";

beforeAll(async () => {
  const result = (await build({
    root: ROOT,
    configFile: resolve(ROOT, "vite.config.ts"),
    logLevel: "silent",
    build: { write: false },
  })) as unknown as BuildOutput | BuildOutput[];
  const first = Array.isArray(result) ? result[0]! : result;

  bundle = first.output
    .map((o) =>
      o.type === "chunk" ? o.code : typeof o.source === "string" ? o.source : "",
    )
    .join("\n");
}, 120_000);

describe("the built bundle", () => {
  it("contains no server-only variable NAME", () => {
    const leaked = SERVER_ONLY.filter((name) => bundle.includes(name));
    expect(
      leaked,
      `these server-only names reached dist/: ${leaked.join(", ")}`,
    ).toEqual([]);
  });

  it("contains no server-only VALUE from the local .env", () => {
    const values = secretValues();
    if (values.length === 0) {
      // No .env on this machine (CI). The name check above still applies.
      return;
    }
    const leaked = values
      .filter(({ value }) => bundle.includes(value))
      .map(({ name }) => name);
    expect(leaked, `the VALUES of these reached dist/: ${leaked.join(", ")}`).toEqual(
      [],
    );
  });

  it("does contain the values that are public by design", () => {
    expect(bundle.length).toBeGreaterThan(10_000);
    expect(bundle).toContain("VITE_API_BASE_URL");
  });
});
