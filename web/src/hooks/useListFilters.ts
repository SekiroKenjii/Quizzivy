import { useCallback, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useLocation, useSearchParams } from "react-router";
import { useAuthStore } from "@/stores/auth";

/** useListFilters keeps filters in the URL and restores them on return within the current session. */
export function useListFilters() {
  const client = useQueryClient();
  const { pathname, search } = useLocation();
  const userId = useAuthStore((state) => state.user?.id ?? "anonymous");
  const key = ["list-filters", userId, pathname];
  const [initial] = useState(() =>
    search === "" ? (client.getQueryData<string>(key) ?? "") : "",
  );
  const [params, setParams] = useSearchParams(initial);
  const first = useRef(true);
  const serialized = params.toString();

  useEffect(() => {
    client.setQueryDefaults(["list-filters", userId, pathname], { gcTime: Infinity });
    client.setQueryData(["list-filters", userId, pathname], serialized);
    if (first.current) {
      first.current = false;
      if (search === "" && serialized !== "") {
        setParams(serialized, { replace: true });
      }
    }
  }, [client, userId, pathname, serialized, search, setParams]);

  const setFilter = useCallback(
    (name: string, value: string | readonly string[] | null) => {
      setParams(
        (previous) => {
          const next = new URLSearchParams(previous);
          next.delete(name);
          next.delete("page");
          if (typeof value === "string" && value !== "") next.set(name, value);
          else if (Array.isArray(value)) {
            for (const item of value) next.append(name, item);
          }
          return next;
        },
        { replace: true },
      );
    },
    [setParams],
  );

  return { params, setParams, setFilter };
}
