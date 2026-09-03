import { useInfiniteQuery } from "@tanstack/react-query";

/** The paging half of every list envelope (O-20). */
export interface PagedResult<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

/**
 * A list that grows as the reader reaches its end: the pickers' answer to
 * paging, where a numbered control would be in the way of choosing.
 */
export function useLazyList<T>({
  queryKey,
  fetchPage,
  enabled = true,
}: {
  queryKey: readonly unknown[];
  fetchPage: (page: number, signal: AbortSignal) => Promise<PagedResult<T>>;
  enabled?: boolean;
}) {
  const query = useInfiniteQuery({
    queryKey,
    initialPageParam: 1,
    queryFn: ({ pageParam, signal }) => fetchPage(pageParam, signal),
    getNextPageParam: (last) =>
      last.page * last.pageSize < last.total ? last.page + 1 : undefined,
    enabled,
  });

  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  return {
    items,
    total: query.data?.pages[0]?.total ?? 0,
    isPending: query.isPending,
    isError: query.isError,
    hasMore: query.hasNextPage,
    loadingMore: query.isFetchingNextPage,
    loadMore: () => {
      if (query.hasNextPage) void query.fetchNextPage({ cancelRefetch: false });
    },
  };
}
