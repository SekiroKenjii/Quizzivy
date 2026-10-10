import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import TestDetailPage from "@/features/tests/pages/teacher/TestDetailPage";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

type Test = components["schemas"]["Test"];
type TestVersion = components["schemas"]["TestVersion"];
type DiffChange = components["schemas"]["DiffChange"];

export const BASE = "http://localhost:8080";
export const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
export const SECTION_ID = "018f0000-0000-7000-8000-0000000000c1";
export const FIRST_QUESTION = "018f0000-0000-7000-8000-0000000000e1";
export const SECOND_QUESTION = "018f0000-0000-7000-8000-0000000000e2";
export const PUBLISHED_PROMPT = "They ___ to the museum last weekend.";
export const ADDED_PROMPT = "Play streets are closed to traffic every day.";

/** testFixture is the draft as getTest answers it: published once, nothing pending. */
export function testFixture(overrides: Partial<Test> = {}): Test {
  return {
    id: TEST_ID,
    title: "Unit 5",
    description: null,
    status: "published",
    currentVersion: 1,
    totalPoints: 2,
    questionCount: 1,
    audioCount: 0,
    skills: [],
    assignments: { live: 0, scheduled: 0, closed: 0 },
    unpublishedChanges: 0,
    sections: [
      {
        id: SECTION_ID,
        ordinal: 0,
        title: "Ngữ pháp",
        instructions: null,
        questionIds: ["018f0000-0000-7000-8000-0000000000b1"],
      },
    ],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
    ...overrides,
  };
}

/** versionFixture is version `n` of the history, published by Cô Thương. */
export function versionFixture(
  n: number,
  overrides: Partial<TestVersion> = {},
): TestVersion {
  return {
    id: `018f0000-0000-7000-8000-0000000000f${n}`,
    version: n,
    totalPoints: n + 1,
    questionCount: n,
    skills: [],
    audioCount: 0,
    manualCount: 0,
    assignmentCount: 0,
    changeNote: null,
    publishedAt: `2026-01-0${n + 1}T03:00:00Z`,
    publishedBy: "Cô Thương",
    ...overrides,
  };
}

function question(id: string, prompt: string) {
  return {
    id,
    sectionId: SECTION_ID,
    type: "single_choice" as const,
    prompt,
    media: null,
    audio: null,
    options: [
      { id: `${id.slice(0, -2)}d1`, text: "went" },
      { id: `${id.slice(0, -2)}d2`, text: "have gone" },
    ],
    blanks: [],
    points: 1,
  };
}

/** Detail is what the stubbed API holds and what the page sent it. */
export interface Detail {
  test: Test;
  versions: TestVersion[];
  changes: DiffChange[];
  previewVersions: (number | null)[];
  diffs: string[];
  publishBodies: unknown[];
  publishFailure: Response | null;
  deleted: number[];
  current: { version: number; body: unknown }[];
  drafted: number[];
}

/** detail is the stub's state, reset by serveDetail before each test. */
export const detail: Detail = {
  test: testFixture(),
  versions: [],
  changes: [],
  previewVersions: [],
  diffs: [],
  publishBodies: [],
  publishFailure: null,
  deleted: [],
  current: [],
  drafted: [],
};

/**
 * serveDetail resets `detail` to a test with one version and serves the
 * detail's endpoints from it. Version 1 holds the first question; any later
 * version adds the second.
 */
export function serveDetail() {
  Object.assign(detail, {
    test: testFixture(),
    versions: [versionFixture(1)],
    changes: [],
    previewVersions: [],
    diffs: [],
    publishBodies: [],
    publishFailure: null,
    deleted: [],
    current: [],
    drafted: [],
  });
  server.use(
    http.get(
      `${BASE}/teacher/tests/:id/versions/:version/diff`,
      ({ request, params }) => {
        const against = new URL(request.url).searchParams.get("against") ?? "";
        detail.diffs.push(`${String(params["version"])}:${against}`);
        return contractJson("/teacher/tests/{id}/versions/{version}/diff", "get", 200, {
          from: {
            kind: "version",
            version: Number(against),
            publishedAt: "2026-01-02T03:00:00Z",
          },
          to: {
            kind: "version",
            version: Number(params["version"]),
            publishedAt: "2026-01-03T03:00:00Z",
          },
          changes: detail.changes,
        });
      },
    ),
    http.get(`${BASE}/teacher/tests/:id/versions`, () =>
      contractJson("/teacher/tests/{id}/versions", "get", 200, {
        items: [...detail.versions].sort((a, b) => b.version - a.version),
      }),
    ),
    http.get(`${BASE}/teacher/tests/:id/preview`, ({ request }) => {
      const asked = new URL(request.url).searchParams.get("version");
      detail.previewVersions.push(asked === null ? null : Number(asked));
      const version = asked === null ? detail.test.currentVersion : Number(asked);
      const questions =
        version > 1
          ? [
              question(FIRST_QUESTION, PUBLISHED_PROMPT),
              question(SECOND_QUESTION, ADDED_PROMPT),
            ]
          : [question(FIRST_QUESTION, PUBLISHED_PROMPT)];
      return contractJson("/teacher/tests/{id}/preview", "get", 200, {
        version,
        questions,
      });
    }),
    http.post(`${BASE}/teacher/tests/:id/publish`, async ({ request }) => {
      const text = await request.text();
      detail.publishBodies.push(text === "" ? null : JSON.parse(text));
      if (detail.publishFailure) return detail.publishFailure;
      const next = Math.max(...detail.versions.map((v) => v.version), 0) + 1;
      return contractJson("/teacher/tests/{id}/publish", "post", 201, {
        ...versionFixture(next),
        testUpdatedAt: "2026-01-09T00:00:00Z",
      });
    }),
    http.delete(`${BASE}/teacher/tests/:id/versions/:version`, ({ params }) => {
      const version = Number(params["version"]);
      detail.deleted.push(version);
      detail.versions = detail.versions.filter((v) => v.version !== version);
      return new HttpResponse(null, { status: 204 });
    }),
    http.post(
      `${BASE}/teacher/tests/:id/versions/:version/current`,
      async ({ request, params }) => {
        const version = Number(params["version"]);
        detail.current.push({ version, body: await request.json() });
        detail.test = { ...detail.test, currentVersion: version };
        return contractJson(
          "/teacher/tests/{id}/versions/{version}/current",
          "post",
          200,
          detail.test,
        );
      },
    ),
    http.post(`${BASE}/teacher/tests/:id/versions/:version/draft`, ({ params }) => {
      detail.drafted.push(Number(params["version"]));
      return contractJson(
        "/teacher/tests/{id}/versions/{version}/draft",
        "post",
        200,
        detail.test,
      );
    }),
    http.get(`${BASE}/teacher/tests/:id`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, detail.test),
    ),
  );
}

/** renderDetail mounts the detail at `path` beside stand-ins for the pages it links to. */
export function renderDetail(path = `/teacher/tests/${TEST_ID}`) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/tests/:id", element: <TestDetailPage /> },
      { path: "/teacher/tests/:id/edit", element: <p>builder</p> },
      { path: "/teacher/tests", element: <p>tests list</p> },
      { path: "/teacher/assignments/new", element: <p>wizard</p> },
    ],
    { initialEntries: [path] },
  );
  const user = userEvent.setup();
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user, router };
}
