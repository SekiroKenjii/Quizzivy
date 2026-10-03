import { useQuery } from "@tanstack/react-query";
import {
  listMyAssignments,
  type StudentAssignmentCard,
} from "@/features/assignments/api";

const WINDOW_MS = 7 * 24 * 60 * 60 * 1000;

/**
 * dueSoonCount is the number on the student's Home badge (DG-82): the papers
 * still to do that close within seven days of `now`. It counts what the
 * server lists as due now or upcoming, so a paper with attempts left counts
 * and a finished one does not. A paper with a live attempt closes when that
 * attempt's deadline does, whatever its window says.
 */
export function dueSoonCount(
  lists: Readonly<{
    dueNow: readonly StudentAssignmentCard[];
    upcoming: readonly StudentAssignmentCard[];
  }>,
  now: number,
): number {
  return [...lists.dueNow, ...lists.upcoming].filter((card) => {
    const closes = Date.parse(
      card.hasLiveAttempt && card.liveDeadlineAt ? card.liveDeadlineAt : card.closesAt,
    );
    return closes > now && closes <= now + WINDOW_MS;
  }).length;
}

/**
 * useDueSoonCount returns the Home badge's number from the assignment lists
 * the student pages share, and 0 while they are loading or could not be
 * read. It counts from the moment the lists were fetched. The lists refresh
 * when the shell mounts and when the tab becomes visible again, never on a
 * timer.
 */
export function useDueSoonCount(): number {
  const { data, dataUpdatedAt } = useQuery({
    queryKey: ["my-assignments"],
    queryFn: ({ signal }) => listMyAssignments(signal),
    refetchOnMount: "always",
    refetchOnWindowFocus: true,
  });
  return data ? dueSoonCount(data, dataUpdatedAt) : 0;
}
