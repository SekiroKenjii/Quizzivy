import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentWizardPage from "@/features/assignments/pages/teacher/AssignmentWizardPage";
import { Toaster } from "@/components/ui/sonner";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";

type QuestionSkill = components["schemas"]["QuestionSkill"];

export const BASE = "http://localhost:8080";
export const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
export const OTHER_TEST_ID = "018f0000-0000-7000-8000-0000000000a2";
export const VERSION_ID = "018f0000-0000-7000-8000-0000000000f1";
export const OTHER_VERSION_ID = "018f0000-0000-7000-8000-0000000000f2";
export const CLASS_A = "018f0000-0000-7000-8000-0000000000c1";
export const CLASS_B = "018f0000-0000-7000-8000-0000000000c2";
export const AN = "018f0000-0000-7000-8000-0000000000e1";
export const BINH = "018f0000-0000-7000-8000-0000000000e2";
export const CHI = "018f0000-0000-7000-8000-0000000000e3";
export const CREATED = "018f0000-0000-7000-8000-0000000000d1";
export const NOW = "2026-09-01T00:00:00Z";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

type Assignment = components["schemas"]["Assignment"];

function publishedTest(id: string, title: string, skills: string[]) {
  return {
    id,
    title,
    description: null,
    status: "published",
    currentVersion: 3,
    totalPoints: 30,
    questionCount: 99,
    audioCount: 0,
    skills,
    assignments: { live: 0, scheduled: 0, closed: 0 },
    unpublishedChanges: null,
    sections: [],
    createdAt: NOW,
    updatedAt: NOW,
  };
}

export function version(id: string, manualCount: number, skills: QuestionSkill[] = []) {
  return {
    id,
    version: 3,
    totalPoints: 30,
    questionCount: 24,
    audioCount: 0,
    manualCount,
    skills,
    assignmentCount: 0,
    changeNote: null,
    publishedAt: NOW,
    publishedBy: "Thương",
  };
}

function klass(id: string, name: string, studentCount: number) {
  return {
    id,
    name,
    description: null,
    studentCount,
    openAssignmentCount: 0,
    selfJoinEnabled: false,
    archivedAt: null,
    createdAt: NOW,
  };
}

const stats = { submittedCount: 0, flaggedCount: 0, activity: { live: false } };

function member(userId: string, fullName: string) {
  return {
    userId,
    fullName,
    email: `${userId}@example.com`,
    joinedVia: "admin",
    joinedAt: NOW,
    joinCodeHint: null,
    stats,
  };
}

function student(id: string, fullName: string, classIds: string[]) {
  return {
    id,
    email: `${id}@example.com`,
    fullName,
    hasPassword: true,
    linkedProviders: [],
    mustChangePassword: false,
    createdAt: NOW,
    disabledAt: null,
    classes: classIds.map((classId) => ({
      id: classId,
      name: classId === CLASS_A ? "IELTS Foundation" : "TOEIC 600",
      joinedVia: "admin",
      joinedAt: NOW,
    })),
    stats,
  };
}

const TESTS = [
  publishedTest(TEST_ID, "Unit 5 Reading", ["reading"]),
  publishedTest(OTHER_TEST_ID, "Mock B", ["grammar"]),
];
export const VERSIONS: Record<string, ReturnType<typeof version>> = {
  [TEST_ID]: version(VERSION_ID, 2, ["listening"]),
  [OTHER_TEST_ID]: version(OTHER_VERSION_ID, 0),
};
export const CLASSES = [
  klass(CLASS_A, "IELTS Foundation", 2),
  klass(CLASS_B, "TOEIC 600", 1),
];
const MEMBERS: Record<string, ReturnType<typeof member>[]> = {
  [CLASS_A]: [member(AN, "Nguyễn An"), member(BINH, "Trần Bình")],
  [CLASS_B]: [member(AN, "Nguyễn An")],
};
export const STUDENTS = [
  student(AN, "Nguyễn An", [CLASS_A, CLASS_B]),
  student(BINH, "Trần Bình", [CLASS_A]),
  student(CHI, "Lê Chi", []),
];

/** Requests holds the bodies the stubbed server received, newest last. */
export const requests: {
  posted: Record<string, unknown>[];
  patched: Record<string, unknown>[];
} = {
  posted: [],
  patched: [],
};

/** installWizardServer stubs every read and write the wizard makes and empties `requests`. */
export function installWizardServer() {
  requests.posted = [];
  requests.patched = [];
  server.use(
    http.get(`${BASE}/teacher/tests`, ({ request }) => {
      const q = new URL(request.url).searchParams.get("q") ?? "";
      const items = TESTS.filter((test) =>
        test.title.toLowerCase().includes(q.toLowerCase()),
      );
      return contractJson("/teacher/tests", "get", 200, {
        items,
        page: 1,
        pageSize: 20,
        total: items.length,
        facets: { all: 2, draft: 0, published: 2, archived: 0 },
        tags: [],
      });
    }),
    http.get(`${BASE}/teacher/tests/:id`, ({ params }) =>
      contractJson(
        "/teacher/tests/{id}",
        "get",
        200,
        TESTS.find((test) => test.id === params["id"]),
      ),
    ),
    http.get(`${BASE}/teacher/tests/:id/versions`, ({ params }) =>
      contractJson("/teacher/tests/{id}/versions", "get", 200, {
        items: [VERSIONS[String(params["id"])]],
      }),
    ),
    http.get(`${BASE}/teacher/classes`, () =>
      contractJson("/teacher/classes", "get", 200, {
        items: CLASSES,
        page: 1,
        pageSize: 100,
        total: CLASSES.length,
        facets: { all: 2, joinable: 0, archived: 0, students: 3 },
      }),
    ),
    http.get(`${BASE}/teacher/classes/:id`, ({ params }) =>
      contractJson(
        "/teacher/classes/{id}",
        "get",
        200,
        CLASSES.find((item) => item.id === params["id"]),
      ),
    ),
    http.get(`${BASE}/teacher/classes/:id/members`, ({ params }) => {
      const items = MEMBERS[String(params["id"])] ?? [];
      return contractJson("/teacher/classes/{id}/members", "get", 200, {
        items,
        page: 1,
        pageSize: 100,
        total: items.length,
      });
    }),
    http.get(`${BASE}/teacher/students`, () =>
      contractJson("/teacher/students", "get", 200, {
        items: STUDENTS,
        page: 1,
        pageSize: 100,
        total: STUDENTS.length,
        facets: { total: STUDENTS.length, activeLast7Days: 0 },
      }),
    ),
    http.get(`${BASE}/teacher/students/:id`, ({ params }) =>
      contractJson(
        "/teacher/students/{id}",
        "get",
        200,
        STUDENTS.find((item) => item.id === params["id"]),
      ),
    ),
    http.post(`${BASE}/teacher/assignments`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      requests.posted.push(body);
      return contractJson("/teacher/assignments", "post", 201, stored(body));
    }),
    http.patch(`${BASE}/teacher/assignments/:id`, async ({ request, params }) => {
      const body = (await request.json()) as Record<string, unknown>;
      requests.patched.push(body);
      return contractJson(
        "/teacher/assignments/{id}",
        "patch",
        200,
        stored(body, String(params["id"])),
      );
    }),
  );
}

/** stored is the assignment the server answers for `body`. */
export function stored(body: Record<string, unknown>, id = CREATED): Assignment {
  const input = body as unknown as components["schemas"]["AssignmentInput"];
  return {
    id,
    testId: TEST_ID,
    testVersionId: input.testVersionId,
    testVersion: 3,
    testTitle: "Unit 5 Reading",
    targets: {
      classes: CLASSES.filter((item) => input.targets.classIds.includes(item.id)).map(
        (item) => ({ id: item.id, name: item.name, studentCount: item.studentCount }),
      ),
      students: STUDENTS.filter((item) =>
        input.targets.studentIds.includes(item.id),
      ).map((item) => ({ id: item.id, name: item.fullName })),
    },
    publishedAt: body["draft"] === true ? null : NOW,
    updatedAt: NOW,
    window: { ...input.window, closedAt: null },
    durationMinutes: input.durationMinutes,
    maxAttempts: input.maxAttempts,
    shuffleQuestions: input.shuffleQuestions,
    shuffleOptions: input.shuffleOptions,
    review: { release: "on_submit", showClassAverage: false, ...input.review },
    studentNote: input.studentNote ?? null,
    integrity: input.integrity,
    status: body["draft"] === true ? "draft" : "scheduled",
  };
}

/** renderWizard mounts the wizard at `entry` beside stand-ins for the pages it leaves to. */
export function renderWizard(entry = "/teacher/assignments/new") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/assignments/new", element: <AssignmentWizardPage /> },
      { path: "/teacher/assignments/:id/edit", element: <AssignmentWizardPage /> },
      { path: "/teacher/assignments/:id", element: <p>assignment detail</p> },
      { path: "/teacher/assignments", element: <p>assignments list</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return router;
}
