import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { http } from "msw";
import AiPanel from "@/features/ai/components/AiPanel";
import { api } from "@/lib/api/client";
import vi_ from "@/lib/i18n/locales/vi.json";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { aiOn, BASE, renderWithApp } from "./support";

vi.mock("@/lib/api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/client")>();
  return { ...actual, api: vi.fn(actual.api) };
});

const ai = vi_.ai;
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const SOURCE = "Cities around the world are rethinking how they use open land.";
const FEATURE = resolve(import.meta.dirname, "../../../src/features/ai");

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    return statSync(path).isDirectory() ? files(path) : [path];
  });
}

function reached() {
  return vi.mocked(api).mock.calls.map(([method, path]) => `${method} ${path}`);
}

function allowed(call: string) {
  return (
    /^post \/teacher\/ai\//.test(call) ||
    /^get \/(teacher\/ai\/usage|public\/status)$/.test(call)
  );
}

beforeEach(() => {
  vi.mocked(api).mockClear();
  aiOn();
  server.use(
    http.post(`${BASE}/teacher/ai/questions/generate`, () =>
      contractJson("/teacher/ai/questions/generate", "post", 200, {
        items: [
          {
            type: "short_answer",
            prompt: "What do cities rethink?",
            answerKey: ["open land"],
            explanation: "",
          },
        ],
        usage: { credits: 500, left: 1000 },
      }),
    ),
    http.post(`${BASE}/teacher/ai/tests/:id/variant`, () =>
      contractJson("/teacher/ai/tests/{id}/variant", "post", 200, {
        changes: [
          {
            questionId: QUESTION_ID,
            number: 1,
            kind: "reword",
            title: "What do cities rethink?",
            before: "What do cities rethink?",
            after: "What are cities rethinking?",
            payload: { prompt: "What are cities rethinking?" },
          },
        ],
        usage: { credits: 300, left: 300 },
      }),
    ),
    http.post(`${BASE}/teacher/ai/bank/cleanup`, () =>
      contractJson("/teacher/ai/bank/cleanup", "post", 200, {
        scanned: 1,
        truncated: false,
        duplicates: [],
        tags: [
          {
            questionId: QUESTION_ID,
            prompt: "What do cities rethink?",
            tags: ["cities"],
          },
        ],
        usage: { credits: 200, left: 100 },
      }),
    ),
    http.post(`${BASE}/teacher/ai/tests/:id/check`, () =>
      contractJson("/teacher/ai/tests/{id}/check", "post", 200, {
        findings: [
          {
            severity: "minor",
            questionId: QUESTION_ID,
            number: 1,
            type: "short_answer",
            title: "What do cities rethink?",
            issue: "The explanation is missing.",
            fix: {
              kind: "explanation",
              description: "Add one.",
              label: "Add explanation",
              payload: { text: "The first sentence says so." },
            },
          },
        ],
        usage: { credits: 400, left: 600 },
      }),
    ),
  );
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

describe("features/ai", () => {
  it("calls only /teacher/ai/* through every kind, while each primary and a fix save through the host", async () => {
    const save = vi.fn(() => Promise.resolve());
    const generate = renderWithApp(
      <AiPanel
        request={{
          kind: "generate",
          context: {
            passages: [],
            destination: { kind: "bank" },
            save,
          },
        }}
        onClose={() => undefined}
      />,
    );
    const dialog = await screen.findByRole("dialog");
    await generate.user.type(within(dialog).getByLabelText(ai.generate.topic), SOURCE);
    await generate.user.click(
      within(dialog).getByRole("button", { name: ai.generate.run }),
    );
    await generate.user.click(
      await within(dialog).findByRole("button", { name: "Lưu 1 câu vào ngân hàng" }),
    );
    expect(save).toHaveBeenCalledTimes(1);
    generate.unmount();

    const fix = vi.fn(() => Promise.resolve());
    const check = renderWithApp(
      <AiPanel
        request={{
          kind: "check",
          context: {
            testId: TEST_ID,
            title: "Unit 5",
            numberOf: () => 1,
            fix,
            open: () => undefined,
          },
        }}
        onClose={() => undefined}
      />,
    );
    const checkDialog = await screen.findByRole("dialog");
    await check.user.click(
      within(checkDialog).getByRole("button", { name: ai.check.run }),
    );
    await check.user.click(
      await within(checkDialog).findByRole("button", { name: "Add explanation" }),
    );
    expect(fix).toHaveBeenCalledTimes(1);
    check.unmount();

    const saveVariant = vi.fn(() => Promise.resolve(TEST_ID));
    const variant = renderWithApp(
      <AiPanel
        request={{
          kind: "variant",
          context: { testId: TEST_ID, title: "Unit 5", save: saveVariant },
        }}
        onClose={() => undefined}
      />,
    );
    const variantDialog = await screen.findByRole("dialog");
    await variant.user.click(
      within(variantDialog).getByRole("button", { name: ai.variant.run }),
    );
    await variant.user.click(
      await within(variantDialog).findByRole("button", {
        name: "Lưu 1 thay đổi thành bản nháp",
      }),
    );
    expect(saveVariant).toHaveBeenCalledTimes(1);
    variant.unmount();

    const tag = vi.fn(() => Promise.resolve(1));
    const cleanup = renderWithApp(
      <AiPanel
        request={{ kind: "cleanup", context: { href: (id) => `/q/${id}`, tag } }}
        onClose={() => undefined}
      />,
    );
    const cleanupDialog = await screen.findByRole("dialog");
    await cleanup.user.click(
      within(cleanupDialog).getByRole("button", { name: ai.cleanup.run }),
    );
    await cleanup.user.click(
      await within(cleanupDialog).findByRole("button", { name: "Thêm thẻ cho 1 câu" }),
    );
    expect(tag).toHaveBeenCalledTimes(1);

    const calls = reached();
    expect(calls).toContain("post /teacher/ai/questions/generate");
    expect(calls).toContain("post /teacher/ai/tests/{id}/check");
    expect(calls).toContain("post /teacher/ai/tests/{id}/variant");
    expect(calls).toContain("post /teacher/ai/bank/cleanup");
    expect(calls.filter((call) => !allowed(call))).toEqual([]);
  });

  it("reaches the API client from api.ts alone, and api.ts names only AI paths", () => {
    const sources = files(FEATURE).filter((path) => /\.tsx?$/.test(path));
    const clients = sources.filter((path) =>
      /from "@\/lib\/api\/client"/.test(readFileSync(path, "utf8")),
    );
    expect(clients.map((path) => relative(FEATURE, path))).toEqual(["api.ts"]);
    const paths = [
      ...readFileSync(join(FEATURE, "api.ts"), "utf8").matchAll(/"(\/[^"]+)"/g),
    ].map((match) => match[1]);
    expect(paths.length).toBeGreaterThan(0);
    expect(
      paths.filter((path) => !/^\/(teacher\/ai\/|public\/status$)/.test(path!)),
    ).toEqual([]);
    const writes = sources.filter((path) =>
      /^import (?!type )[^;]*from "@\/features\/(?!ai\/)[^"]+\/api"/m.test(
        readFileSync(path, "utf8"),
      ),
    );
    expect(writes.map((path) => relative(FEATURE, path))).toEqual([]);
  });

  it("is loaded lazily by every page that opens it", () => {
    const stack = readFileSync(resolve(FEATURE, "components/AiPanelStack.tsx"), "utf8");
    expect(stack).toMatch(/lazy\(\(\) => import\("\.\/AiPanel"\)\)/);
    expect(stack).not.toMatch(/^import AiPanel/m);
    const hosts = [
      "features/tests/components/BuilderAiActions.tsx",
      "features/tests/components/VariantAction.tsx",
      "features/question-bank/components/BankAiActions.tsx",
      "features/assignments/components/AiInsightsPanel.tsx",
    ].map((path) => readFileSync(resolve(FEATURE, "..", "..", path), "utf8"));
    for (const source of hosts) {
      expect(source).toMatch(
        /import \{ AiPanelStack \} from "@\/features\/ai\/components\/AiPanelStack"/,
      );
      expect(source).not.toMatch(
        /^import (?!type )[^;]*from "@\/features\/ai\/components\/AiPanel"/m,
      );
      expect(source).not.toMatch(
        /^import \{[^}]*\b(generateKind|checkKind|variantKind|cleanupKind)\b/m,
      );
    }
  });
});
