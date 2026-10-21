import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");
const WRITES_THE_SEARCH = /(?:setFilter|change)\("q",|params\.get\("q"\)/;
const HOOK = /const \[(\w+), (\w+)\] = useDebouncedInput\(/;

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx$/.test(name) ? [path] : [];
  });
}

const searchesInTheUrl = sources(SRC)
  .filter((path) => WRITES_THE_SEARCH.test(readFileSync(path, "utf8")))
  .map((path) => relative(SRC, path));

describe("a search box whose text lives in the URL", () => {
  it("is found on every list that has one", () => {
    expect(searchesInTheUrl).toEqual(
      expect.arrayContaining([
        "features/assignments/pages/teacher/AssignmentsListPage.tsx",
        "features/attempts/components/Monitor.tsx",
        "features/audit/pages/admin/AuditLogPage.tsx",
        "features/classes/components/ClassMembersCard.tsx",
        "features/imports/pages/teacher/ImportsListPage.tsx",
        "features/media/pages/teacher/MediaPage.tsx",
        "features/question-bank/pages/teacher/QuestionBankPage.tsx",
        "features/question-groups/pages/teacher/GroupsListPage.tsx",
        "features/students/pages/teacher/StudentsPage.tsx",
        "features/tests/pages/teacher/TestsListPage.tsx",
        "features/users/pages/admin/UsersPage.tsx",
      ]),
    );
  });

  it.each(searchesInTheUrl)(
    "%s binds its field to useDebouncedInput, so a slow navigation cannot take a keystroke back",
    (path) => {
      const text = readFileSync(join(SRC, path), "utf8");
      const hook = HOOK.exec(text);
      expect(hook, "a useDebouncedInput call").not.toBeNull();
      const [, value, setter] = hook!;
      expect(text).toMatch(new RegExp(`=\\{${value}\\}`));
      expect(text).toMatch(new RegExp(`=\\{${setter}\\}`));
    },
  );
});
