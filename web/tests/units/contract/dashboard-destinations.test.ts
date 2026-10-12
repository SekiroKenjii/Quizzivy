import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import type { components } from "@/lib/api/schema";
import { loadSpec } from "@tests/support/openapi";

const spec = loadSpec();
const ajv = new Ajv({ strict: false, validateFormats: true });
addFormats(ajv);
const shape = ajv.compile({
  $ref: "#/components/schemas/Dashboard",
  components: spec.components,
});
const id = "019535d9-3df7-79fb-b466-fa907fa17f9e";
function fixture(): components["schemas"]["Dashboard"] {
  return {
    openAssignments: 0,
    awaitingGrading: 0,
    activeStudents: 0,
    flaggedAttempts: 0,
    recentAttempts: [],
    newestFlaggedAttempt: null,
    takingNow: { students: 0, assignments: 0, assignmentId: null },
    submissions: {
      days: Array.from({ length: 14 }, (_, i) => ({
        date: new Date(Date.UTC(2026, 8, 22 + i)).toISOString().slice(0, 10),
        count: 0,
      })),
      total: 0,
      averagePercent: null,
    },
    today: [],
    recentActivity: [],
  };
}
test("dashboard destinations require explicit nulls or a complete UUID pair", () => {
  const body = fixture();
  expect(shape(body), JSON.stringify(shape.errors)).toBe(true);
  expect(
    shape({
      ...body,
      newestFlaggedAttempt: { assignmentId: id, attemptId: id },
      takingNow: { students: 2, assignments: 1, assignmentId: id },
    }),
    JSON.stringify(shape.errors),
  ).toBe(true);
});
test.each([
  ["absent flagged destination", { ...fixture(), newestFlaggedAttempt: undefined }],
  [
    "absent taking destination",
    { ...fixture(), takingNow: { students: 0, assignments: 0 } },
  ],
  ["missing attempt", { ...fixture(), newestFlaggedAttempt: { assignmentId: id } }],
  ["missing assignment", { ...fixture(), newestFlaggedAttempt: { attemptId: id } }],
  [
    "null member",
    { ...fixture(), newestFlaggedAttempt: { assignmentId: id, attemptId: null } },
  ],
  [
    "extra identity",
    {
      ...fixture(),
      newestFlaggedAttempt: { assignmentId: id, attemptId: id, studentId: id },
    },
  ],
  [
    "invalid flagged UUID",
    { ...fixture(), newestFlaggedAttempt: { assignmentId: id, attemptId: "invalid" } },
  ],
  [
    "invalid taking UUID",
    {
      ...fixture(),
      takingNow: { students: 1, assignments: 1, assignmentId: "invalid" },
    },
  ],
])("rejects %s", (_, body) => {
  expect(shape(body)).toBe(false);
});
