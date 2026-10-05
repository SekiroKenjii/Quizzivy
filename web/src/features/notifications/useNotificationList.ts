import { useInfiniteQuery } from "@tanstack/react-query";
import { listNotifications, notificationKeys } from "./api";

/** useNotificationList appends twenty-item cursor pages without polling or repeating a loaded cursor. */
export function useNotificationList(enabled = true) {
  const query = useInfiniteQuery({
    queryKey: notificationKeys.list,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      listNotifications(
        pageParam ? { before: pageParam, limit: 20 } : { limit: 20 },
        signal,
      ),
    getNextPageParam: (last, _pages, _lastParam, pageParams) =>
      last.nextBefore === null || pageParams.includes(last.nextBefore)
        ? undefined
        : last.nextBefore,
    enabled,
  });
  return {
    items: query.data?.pages.flatMap((page) => page.items) ?? [],
    hasMore: query.hasNextPage,
    loadMore: () => {
      if (enabled && query.hasNextPage)
        void query.fetchNextPage({ cancelRefetch: false });
    },
    loadingMore: query.isFetchingNextPage,
    isPending: query.isPending,
    isError: query.isError,
    refetch: query.refetch,
  };
}
