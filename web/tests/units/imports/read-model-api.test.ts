import { describe, expect, it } from "vitest";
import { http } from "msw";
import { listWordImports } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { BASE, wordImport } from "./fixtures";

const facets = {
  all: 8,
  processing: 4,
  needsReview: 1,
  failed: 1,
  committed: 1,
  cancelled: 1,
};

describe("import history read model", () => {
  it("sends repeated OR statuses and preserves nullable current counts above smallint", async () => {
    const queries: URLSearchParams[] = [];
    server.use(
      http.get(`${BASE}/teacher/imports`, ({ request }) => {
        queries.push(new URL(request.url).searchParams);
        return contractJson("/teacher/imports", "get", 200, {
          items: [
            { ...wordImport(), reviewCounts: { needsAction: 2, toConfirm: 32768 } },
            { ...wordImport(), reviewCounts: null },
          ],
          page: 1,
          pageSize: 20,
          total: 2,
          facets,
        });
      }),
    );
    const result = await listWordImports({
      status: ["needs_review", "failed"],
      q: "Đề %_",
      page: 1,
    });
    expect(queries[0]?.getAll("status")).toEqual(["needs_review", "failed"]);
    expect(queries[0]?.get("q")).toBe("Đề %_");
    expect(result.facets).toEqual(facets);
    expect(result.items.map((item) => item.reviewCounts)).toEqual([
      { needsAction: 2, toConfirm: 32768 },
      null,
    ]);
  });

  it("still sends a single selected status as one ordinary query value", async () => {
    const queries: string[][] = [];
    server.use(
      http.get(`${BASE}/teacher/imports`, ({ request }) => {
        queries.push(new URL(request.url).searchParams.getAll("status"));
        return contractJson("/teacher/imports", "get", 200, {
          items: [],
          page: 1,
          pageSize: 20,
          total: 0,
          facets,
        });
      }),
    );
    await listWordImports({ status: ["failed"] });
    expect(queries).toEqual([["failed"]]);
  });

  it("refuses missing or negative counts and facets instead of accepting false fixtures", () => {
    const base = {
      items: [{ ...wordImport(), reviewCounts: null }],
      page: 1,
      pageSize: 20,
      total: 1,
      facets,
    };
    expect(() => contractJson("/teacher/imports", "get", 200, base)).not.toThrow();
    expect(() =>
      contractJson("/teacher/imports", "get", 200, { ...base, items: [wordImport()] }),
    ).toThrow(/reviewCounts/);
    expect(() =>
      contractJson("/teacher/imports", "get", 200, {
        ...base,
        items: [{ ...wordImport(), reviewCounts: { needsAction: -1, toConfirm: 0 } }],
      }),
    ).toThrow(/needsAction must be >= 0/);
    expect(() =>
      contractJson("/teacher/imports", "get", 200, {
        ...base,
        facets: { ...facets, processing: -1 },
      }),
    ).toThrow(/processing must be >= 0/);
    expect(() =>
      contractJson("/teacher/imports", "get", 200, {
        items: base.items,
        page: 1,
        pageSize: 20,
        total: 1,
      }),
    ).toThrow(/facets/);
  });
});
