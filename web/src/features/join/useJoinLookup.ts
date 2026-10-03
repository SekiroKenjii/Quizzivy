import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";

import { ApiError } from "@/lib/api/errors";
import { lookupJoinCode, type JoinPreview } from "./api";
import { CODE_LENGTH, hasExcluded } from "./code";

/** LOOKUP_DELAY_MS is how long a complete code rests before it is looked up. */
export const LOOKUP_DELAY_MS = 250;

/**
 * JoinLookup is what a typed code has come to: the class it found, or the
 * line that says why it cannot be used. `code` is the complete code once it
 * has rested, `checking` is true while its lookup is out, and `retryAfter` is
 * the wait in seconds when the server said the caller is going too fast.
 */
export interface JoinLookup {
  readonly code: string | null;
  readonly found: JoinPreview | undefined;
  readonly message: string | null;
  readonly checking: boolean;
  readonly retryAfter: number | null;
}

function useSettled(code: string | null): string | null {
  const [settled, setSettled] = useState(code);
  if (code === null && settled !== null) setSettled(null);
  useEffect(() => {
    if (code === null) return;
    const timer = setTimeout(() => setSettled(code), LOOKUP_DELAY_MS);
    return () => clearTimeout(timer);
  }, [code]);
  return code !== null && settled === code ? code : null;
}

function failureLine(error: unknown, t: TFunction): string {
  if (error instanceof ApiError && error.isRateLimited) return error.message;
  return t("join.lookupFailed");
}

/**
 * useJoinLookup looks up the class behind a cleaned join code (§6.2). Nothing
 * is sent while the code is incomplete, holds a character no code uses, or is
 * still being typed: a complete code is sent once it has rested for
 * LOOKUP_DELAY_MS, and an answer is kept for 30 seconds, a refusal as much as
 * a found class, so retyping a code costs no second request. An edit does not
 * abandon a lookup already out: the server has counted it, so it finishes and
 * is remembered. A lookup that could not be made is sent again when the code
 * is retyped, never by itself. Every refusal reads as the one line that names
 * no class.
 */
export function useJoinLookup(code: string): JoinLookup {
  const { t } = useTranslation();
  const excluded = hasExcluded(code);
  const settled = useSettled(code.length === CODE_LENGTH && !excluded ? code : null);
  const preview = useQuery({
    queryKey: ["join-preview", settled],
    queryFn: () => lookupJoinCode(settled ?? ""),
    enabled: settled !== null,
    retry: false,
    staleTime: 30_000,
    refetchOnReconnect: false,
  });

  let message: string | null = null;
  if (excluded) message = t("join.excluded");
  else if (settled !== null && preview.isError) message = failureLine(preview.error, t);
  else if (settled !== null && preview.data === null) message = t("join.notFound");
  const limited =
    settled !== null && preview.error instanceof ApiError && preview.error.isRateLimited
      ? preview.error
      : null;
  return {
    code: settled,
    found: settled !== null ? (preview.data ?? undefined) : undefined,
    message,
    checking: settled !== null && preview.isFetching,
    retryAfter: limited?.retryAfterSeconds ?? null,
  };
}

/** toneFor is the code field's tone for a lookup: danger for a message, success for a class. */
export function toneFor(message: string | null, found: boolean) {
  if (message) return "danger";
  return found ? "success" : "idle";
}
