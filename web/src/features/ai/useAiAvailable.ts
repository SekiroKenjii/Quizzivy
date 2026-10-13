import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useCan } from "@/features/auth/permissions";
import { useAuthStore } from "@/stores/auth";
import {
  AI_STATUS_KEY,
  AI_USAGE_KEY,
  getAiUsage,
  readAiStatus,
  type AiLanguage,
  type AiMonthUsage,
} from "./api";

/**
 * AiBlock is why a teacher cannot use the assistant now although it is
 * offered: paused by an Admin, out of their own allowance, or the centre's
 * pool used up.
 */
export type AiBlock = "paused" | "teacher" | "pool";

/** AI_BLOCK_KEYS names the sentence that gives each AiBlock as a reason. */
export const AI_BLOCK_KEYS: Readonly<Record<AiBlock, string>> = {
  paused: "ai.blocked.paused",
  teacher: "ai.blocked.teacher",
  pool: "ai.blocked.pool",
};

/**
 * useAiAvailable reports whether AI entry points are drawn at all: the server
 * has an assistant (`PublicStatus.ai`) and the signed-in user holds `ai.use`.
 * Without `ai.use` the status is not read.
 */
export function useAiAvailable(): boolean {
  const allowed = useCan("ai.use");
  const status = useQuery({
    queryKey: AI_STATUS_KEY,
    queryFn: ({ signal }) => readAiStatus(signal),
    enabled: allowed,
    staleTime: 60_000,
  });
  return allowed && status.data === true;
}

/**
 * blockOf reads a month of usage as the reason the assistant cannot be used,
 * or null when it can. A teacher with nothing left is out of their own
 * allowance when they used all of it, and otherwise the pool ran out first.
 */
export function blockOf(usage: AiMonthUsage): AiBlock | null {
  if (usage.paused) return "paused";
  if (usage.left > 0) return null;
  return usage.used >= usage.allowance ? "teacher" : "pool";
}

/**
 * useAiBlock subscribes to why the assistant cannot be used now, read from
 * the teacher's month of usage while `available` is true. Until the read
 * answers, and when it fails, nothing is blocked: the server still refuses a
 * call it cannot take.
 */
export function useAiBlock(available: boolean): AiBlock | null {
  const usage = useQuery({
    queryKey: AI_USAGE_KEY,
    queryFn: ({ signal }) => getAiUsage(signal),
    enabled: available,
    staleTime: 30_000,
    select: blockOf,
  });
  return available ? (usage.data ?? null) : null;
}

/** useAiLeft subscribes to the credits the teacher has left this month, or null until read. */
export function useAiLeft(): number | null {
  const usage = useQuery({
    queryKey: AI_USAGE_KEY,
    queryFn: ({ signal }) => getAiUsage(signal),
    staleTime: 30_000,
    select: (month: AiMonthUsage) => month.left,
  });
  return usage.data ?? null;
}

/**
 * useAiLanguage is the language the assistant writes in for the signed-in
 * teacher: their preference, else the language of the interface.
 */
export function useAiLanguage(): AiLanguage {
  const { i18n } = useTranslation();
  const stored = useAuthStore((s) => s.user?.preferences?.ai?.language);
  if (stored) return stored;
  return i18n.resolvedLanguage?.startsWith("en") ? "en" : "vi";
}

/** useAiTagsDrafts reports whether what the assistant drafts is tagged, which is on unless turned off. */
export function useAiTagsDrafts(): boolean {
  return useAuthStore((s) => s.user?.preferences?.ai?.tagDrafts !== false);
}
