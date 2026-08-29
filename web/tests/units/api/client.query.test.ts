import { describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { api } from "@/lib/api/client";
import { server } from "@tests/support/server";

const BASE = "http://localhost:8080";

/**
 * A-06's rail sends several types and several tags at once. OpenAPI declares
 * them `style: form, explode: true`, which is one repeated key per value —
 * String(array) would send "a,b" as a single value and the server would look
 * for a question type spelled "a,b".
 */
describe("array query parameters", () => {
  it("repeats the key once per value", async () => {
    let seen: string | null = null;
    server.use(
      http.get(`${BASE}/admin/questions`, ({ request }) => {
        seen = new URL(request.url).search;
        return HttpResponse.json({
          items: [],
          nextCursor: null,
          facets: {
            all: 0,
            single_choice: 0,
            multiple_choice: 0,
            true_false: 0,
            fill_blank: 0,
            short_answer: 0,
          },
        });
      }),
    );

    await api("get", "/admin/questions", {
      query: { type: ["short_answer", "fill_blank"], tag: ["unit-5"], hasAudio: true },
    });

    const params = new URLSearchParams(seen ?? "");
    expect(params.getAll("type")).toEqual(["short_answer", "fill_blank"]);
    expect(params.getAll("tag")).toEqual(["unit-5"]);
    expect(params.get("hasAudio")).toBe("true");
  });
});
