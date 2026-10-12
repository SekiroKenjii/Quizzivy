import { useState, type Dispatch, type SetStateAction } from "react";
import { useQuery } from "@tanstack/react-query";
import type { AssignmentDraft } from "@/features/assignments/draft";
import { fetchClass } from "@/features/classes/api";
import { getTest, listVersions } from "@/features/tests/api";

type SetDraft = Dispatch<SetStateAction<AssignmentDraft>>;

/**
 * usePickFromQuery picks the current version of the test named by `testId`
 * once, when it loads, unless the draft already has a test. A null id does
 * nothing.
 */
export function usePickFromQuery(testId: string | null, setDraft: SetDraft) {
  const test = useQuery({
    queryKey: ["admin-test", testId],
    queryFn: ({ signal }) => getTest(testId ?? "", signal),
    enabled: testId !== null,
  });
  const versions = useQuery({
    queryKey: ["admin-test-versions", testId],
    queryFn: ({ signal }) => listVersions(testId ?? "", signal),
    enabled: testId !== null,
  });
  const latest = versions.data?.items.find(
    (version) => version.version === test.data?.currentVersion,
  );
  const [pickedFor, setPickedFor] = useState<string | null>(null);
  if (test.data && latest && pickedFor !== test.data.id) {
    setPickedFor(test.data.id);
    const picked = {
      testId: test.data.id,
      testTitle: test.data.title,
      version: latest,
    };
    setDraft((current) => (current.picked === null ? { ...current, picked } : current));
  }
}

/**
 * useClassFromQuery adds the class named by `classId` to the draft's targets
 * once, when it loads. A null id does nothing.
 */
export function useClassFromQuery(classId: string | null, setDraft: SetDraft) {
  const klass = useQuery({
    queryKey: ["admin-class", classId],
    queryFn: ({ signal }) => fetchClass(classId ?? "", signal),
    enabled: classId !== null,
  });
  const [appliedFor, setAppliedFor] = useState<string | null>(null);
  const found = klass.data;
  if (found && appliedFor !== found.id) {
    setAppliedFor(found.id);
    const token = { id: found.id, label: found.name, hint: String(found.studentCount) };
    setDraft((current) =>
      current.classes.some((c) => c.id === token.id)
        ? current
        : { ...current, classes: [...current.classes, token] },
    );
  }
}
