import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api/errors";

/** §2: "All API reads/writes go through query/mutation hooks." */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (failureCount, error) => {
        if (error instanceof ApiError) {
          // 4xx is a statement about the request, not a transient fault.
          if (error.status >= 400 && error.status < 500) return false;
        }
        return failureCount < 2;
      },
      refetchOnWindowFocus: false,
    },
    mutations: {
      // A mutation may not be idempotent. Never retry one automatically.
      retry: false,
    },
  },
});
