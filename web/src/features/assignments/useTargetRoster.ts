import { useQuery } from "@tanstack/react-query";
import { fetchMembers } from "@/features/classes/api";
import { getStudent } from "@/features/students/api";

/** useTargetRoster counts enabled students once across all selected classes and individuals. */
export function useTargetRoster(classIds: string[], studentIds: string[]) {
  const classes = [...new Set(classIds)].sort((a, b) => a.localeCompare(b));
  const students = [...new Set(studentIds)].sort((a, b) => a.localeCompare(b));
  return useQuery({
    queryKey: ["assignment-target-roster", classes, students],
    staleTime: 10_000,
    queryFn: async ({ signal }) => {
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
            return [...first.items, ...pages.flatMap((page) => page.items)];
          }),
        ),
        Promise.all(students.map((id) => getStudent(id, signal))),
      ]);
      const names = new Map<string, string>();
      const overlaps = new Set<string>();
      const add = (id: string, name: string) => {
        if (names.has(id)) overlaps.add(id);
        names.set(id, name);
      };
      for (const member of members.flat()) add(member.userId, member.fullName);
      for (const student of individual) {
        if (student.disabledAt === null) add(student.id, student.fullName);
      }
      return { total: names.size, overlaps: [...overlaps].map((id) => names.get(id)!) };
    },
  });
}
