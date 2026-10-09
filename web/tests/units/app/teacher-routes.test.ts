import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const ROUTER = readFileSync(
  resolve(import.meta.dirname, "../../../src/app/router.tsx"),
  "utf8",
);
const ROUTE =
  /(?:path: "([^"]+)"|index: true),(?:(?!path: "|index: true)[^])*?import\("@\/([^"]+)"\)/g;

const TEACHER_TREE = [
  ["/teacher", "layouts/TeacherShell"],
  ["/teacher (index)", "features/dashboard/pages/teacher/TeacherDashboardPage"],
  ["/teacher/tests", "features/tests/pages/teacher/TestsListPage"],
  ["/teacher/tests/:id", "features/tests/pages/teacher/TestDetailPage"],
  ["/teacher/tests/:id/edit", "features/tests/pages/teacher/TestBuilderPage"],
  ["/teacher/imports", "features/imports/pages/teacher/ImportsGate"],
  ["/teacher/imports (index)", "features/imports/pages/teacher/ImportsListPage"],
  ["/teacher/imports/new", "features/imports/pages/teacher/NewImportPage"],
  ["/teacher/imports/:id", "features/imports/pages/teacher/ImportDetailPage"],
  ["/teacher/imports/:id/review", "features/imports/pages/teacher/ImportReviewPage"],
  ["/teacher/question-bank", "features/question-bank/pages/teacher/QuestionBankPage"],
  [
    "/teacher/question-bank/groups",
    "features/question-groups/pages/teacher/GroupsListPage",
  ],
  [
    "/teacher/question-bank/groups/:id",
    "features/question-groups/pages/teacher/GroupEditorPage",
  ],
  [
    "/teacher/question-bank/new",
    "features/question-bank/pages/teacher/QuestionEditorPage",
  ],
  [
    "/teacher/question-bank/:id",
    "features/question-bank/pages/teacher/QuestionEditorPage",
  ],
  ["/teacher/media", "features/media/pages/teacher/MediaPage"],
  ["/teacher/assignments", "features/assignments/pages/teacher/AssignmentsListPage"],
  ["/teacher/assignments/new", "features/assignments/pages/teacher/AssignmentFormPage"],
  [
    "/teacher/assignments/:id",
    "features/assignments/pages/teacher/AssignmentDetailPage",
  ],
  [
    "/teacher/assignments/:id/edit",
    "features/assignments/pages/teacher/AssignmentFormPage",
  ],
  ["/teacher/attempts/:id", "features/attempts/pages/teacher/AttemptReviewPage"],
  ["/teacher/grading", "features/attempts/pages/teacher/GradingQueuePage"],
  ["/teacher/students", "features/students/pages/teacher/StudentsListPage"],
  ["/teacher/classes", "features/classes/pages/teacher/ClassesListPage"],
  ["/teacher/classes/:id", "features/classes/pages/teacher/ClassDetailPage"],
  ["/teacher/settings/:section?", "features/auth/pages/AdminSettingsPage"],
];

function treeOf(source: string, name: string): string {
  const start = source.indexOf(`const ${name}: RouteObject = {`);
  const end = source.indexOf("\n};", start);
  if (start < 0 || end < 0) throw new Error(`no ${name} in the router`);
  return source.slice(source.indexOf("{", start), end);
}

function depthAt(tree: string, index: number): number {
  const before = tree.slice(0, index);
  return before.split("{").length - before.split("}").length;
}

function routesOf(source: string): string[][] {
  const tree = treeOf(source, "teacherTree");
  const open: { depth: number; path: string }[] = [];
  return [...tree.matchAll(ROUTE)].map((match) => {
    const depth = depthAt(tree, match.index);
    while (open.length > 0 && open.at(-1)!.depth >= depth) open.pop();
    const parent = open.map((route) => `/${route.path}`).join("");
    if (match[1] === undefined) return [`${parent} (index)`, match[2]!];
    open.push({ depth, path: match[1] });
    return [`${parent}/${match[1]}`, match[2]!];
  });
}

function depthsOf(source: string): number[] {
  const tree = treeOf(source, "teacherTree");
  return [...tree.matchAll(ROUTE)].map((match) => depthAt(tree, match.index));
}

function swapped(source: string, one: string, other: string): string {
  return source
    .replace(`import("@/${one}")`, "import(#)")
    .replace(`import("@/${other}")`, `import("@/${one}")`)
    .replace("import(#)", `import("@/${other}")`);
}

describe("the teacher's route table", () => {
  it("loads, at every path under /teacher, the page that belongs there", () => {
    expect(routesOf(ROUTER)).toEqual(TEACHER_TREE);
  });

  it("is the whole tree: every module the tree imports is paired with a path", () => {
    const imports = [
      ...treeOf(ROUTER, "teacherTree").matchAll(/import\("@\/([^"]+)"\)/g),
    ];
    expect(imports.map((match) => match[1])).toEqual(
      TEACHER_TREE.map(([, module]) => module),
    );
  });

  it("keeps every page inside the layout, and the import pages inside their gate", () => {
    const gate = TEACHER_TREE.findIndex(([path]) => path === "/teacher/imports");
    expect(depthsOf(ROUTER)).toEqual(
      TEACHER_TREE.map(([path], row) => {
        if (row === 0) return 1;
        return row > gate && path!.startsWith("/teacher/imports") ? 4 : 3;
      }),
    );
  });

  it("would be caught: two pages behind each other's paths", () => {
    const crossed = routesOf(
      swapped(
        ROUTER,
        "features/assignments/pages/teacher/AssignmentsListPage",
        "features/classes/pages/teacher/ClassesListPage",
      ),
    );
    expect(crossed).not.toEqual(TEACHER_TREE);
    expect(crossed).toContainEqual([
      "/teacher/assignments",
      "features/classes/pages/teacher/ClassesListPage",
    ]);
    expect(crossed).toContainEqual([
      "/teacher/classes",
      "features/assignments/pages/teacher/AssignmentsListPage",
    ]);
  });

  it("would be caught: a page moved out from under its parent", () => {
    const sample = `const teacherTree: RouteObject = {
  path: "teacher",
  children: [
    {
      lazy: page(() => import("@/layouts/Layout")),
      children: [
        {
          path: "imports",
          handle: { crumb: { key: "nav.imports" } },
          lazy: page(() => import("@/pages/Gate")),
          children: [
            { index: true, lazy: page(() => import("@/pages/List")) },
            { path: ":id", lazy: page(() => import("@/pages/Detail")) },
          ],
        },
        { path: "new", lazy: page(() => import("@/pages/New")) },
        { index: true, lazy: page(() => import("@/pages/Home")) },
      ],
    },
  ],
};`;
    expect(routesOf(sample)).toEqual([
      ["/teacher", "layouts/Layout"],
      ["/teacher/imports", "pages/Gate"],
      ["/teacher/imports (index)", "pages/List"],
      ["/teacher/imports/:id", "pages/Detail"],
      ["/teacher/new", "pages/New"],
      ["/teacher (index)", "pages/Home"],
    ]);
  });

  it("still reads a route that declares a handle between its path and its page", () => {
    const withHandles = ROUTER.replace(
      /(path: "[^"]+"|index: true),/g,
      '$1,\n          handle: { crumb: { key: "nav.tests" }, width: 1080 },',
    );
    expect(withHandles).not.toBe(ROUTER);
    expect(routesOf(withHandles)).toEqual(TEACHER_TREE);
    expect(depthsOf(withHandles)).toEqual(depthsOf(ROUTER));
  });
});
