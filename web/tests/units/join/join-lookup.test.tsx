import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { useJoinLookup } from "@/features/join/useJoinLookup";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const CODE = "K7QM2PXA";

function wrapper({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>
  );
}

afterEach(() => vi.useRealTimers());

describe("the join lookup", () => {
  it("holds a complete code for 250 ms before it looks it up", () => {
    vi.useFakeTimers();
    server.use(
      http.post("http://localhost:8080/join/preview", () => new Response(null)),
    );
    const { result, rerender } = renderHook(({ code }) => useJoinLookup(code), {
      initialProps: { code: "" },
      wrapper,
    });
    rerender({ code: CODE });
    act(() => {
      vi.advanceTimersByTime(249);
    });
    expect(result.current.code).toBeNull();
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current.code).toBe(CODE);
  });

  it("holds a code again when it is retyped", () => {
    vi.useFakeTimers();
    server.use(
      http.post("http://localhost:8080/join/preview", () => new Response(null)),
    );
    const { result, rerender } = renderHook(({ code }) => useJoinLookup(code), {
      initialProps: { code: CODE },
      wrapper,
    });
    expect(result.current.code).toBe(CODE);
    rerender({ code: CODE.slice(0, 7) });
    rerender({ code: CODE });
    expect(result.current.code).toBeNull();
    act(() => {
      vi.advanceTimersByTime(250);
    });
    expect(result.current.code).toBe(CODE);
  });
});
