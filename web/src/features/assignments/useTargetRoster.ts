import { useQuery } from "@tanstack/react-query";
import { fetchMembers } from "@/features/classes/api";
import { getStudent } from "@/features/students/api";

/**
 * TargetRoster is who an assignment reaches: `total` enabled students counted
 * once, `fromClasses` of them through a class, the names of those picked more
 * than once in `overlaps`, and `classesOf`, for each individually picked student
 * who is also in a picked class, the ids of those classes.
 */
export interface TargetRoster {
  total: number;
  fromClasses: number;
  overlaps: string[];
  classesOf: Record<string, string[]>;
}

/** useTargetRoster counts enabled students once across all selected classes and individuals. */
export function useTargetRoster(classIds: string[], studentIds: string[]) {
  const classes = [...new Set(classIds)].sort((a, b) => a.localeCompare(b));
  const students = [...new Set(studentIds)].sort((a, b) => a.localeCompare(b));
  return useQuery({
    queryKey: ["assignment-target-roster", classes, students],
    staleTime: 10_000,
    queryFn: async ({ signal }): Promise<TargetRoster> => {
      const [members, individual] = await Promise.all([
        Promise.all(
          classes.map(async (id) => {
            const first = await fetchMembers(id, { limit: 100 }, signal);
            const pages = await Promise.all(
              Array.from(
                { length: Math.max(0, Math.ceil(first.total / first.pageSize) - 1) },
                (_, i) => fetchMembers(id, { limit: 100, page: i + 2 }, signal),
              ),
            );
            return {
              id,
              items: [...first.items, ...pages.flatMap((page) => page.items)],
            };
          }),
        ),
        Promise.all(students.map((id) => getStudent(id, signal))),
      ]);
      const names = new Map<string, string>();
      const overlaps = new Set<string>();
      const memberClasses = new Map<string, string[]>();
      const add = (id: string, name: string) => {
        if (names.has(id)) overlaps.add(id);
        names.set(id, name);
      };
      for (const klass of members) {
        for (const member of klass.items) {
          add(member.userId, member.fullName);
          memberClasses.set(member.userId, [
            ...(memberClasses.get(member.userId) ?? []),
            klass.id,
          ]);
        }
      }
      const fromClasses = names.size;
      const classesOf: Record<string, string[]> = {};
      for (const student of individual) {
        if (student.disabledAt !== null) continue;
        const through = memberClasses.get(student.id);
        if (through) classesOf[student.id] = through;
        add(student.id, student.fullName);
      }
      return {
        total: names.size,
        fromClasses,
        overlaps: [...overlaps].map((id) => names.get(id) ?? id),
        classesOf,
      };
    },
  });
}
