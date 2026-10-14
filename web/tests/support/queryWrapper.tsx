import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

/**
 * queryWrapper is the `wrapper` of a render with a query client of its own,
 * for a page that reads the front door's organization and is rendered
 * outside the app's providers. Nothing retries, so a failing read settles at once.
 */
export function queryWrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Wrapper({ children }: Readonly<{ children: ReactNode }>) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}
