import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api/errors";

/**
 * §2: "All API reads/writes go through query/mutation hooks."
 *
 * The retry policy matters more than it looks. Retrying a 4xx is pointless and
 * actively harmful for two of ours: a 429 retried immediately makes the rate
 * limit worse (§6.5), and a 401 is already handled inside the client by
 * refresh-and-retry, so a Query-level retry would multiply attempts against a
 * session that is already being repaired.
 */
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
