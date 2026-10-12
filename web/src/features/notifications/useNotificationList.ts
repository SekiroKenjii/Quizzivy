import { useInfiniteQuery } from "@tanstack/react-query";
import { listNotifications, notificationKeys, type NotificationAudience } from "./api";

/** useNotificationList appends twenty-item cursor pages of `audience`'s kinds without polling or repeating a loaded cursor. */
export function useNotificationList(audience: NotificationAudience, enabled = true) {
  const query = useInfiniteQuery({
    queryKey: notificationKeys.list(audience),
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      listNotifications(
        pageParam
          ? { audience, before: pageParam, limit: 20 }
          : { audience, limit: 20 },
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
