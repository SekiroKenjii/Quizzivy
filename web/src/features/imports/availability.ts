import { useQuery, type QueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api/errors";
import type { ImportCapabilities } from "./api";
import { importCapabilitiesQuery } from "./queries";

/**
 * ImportAvailability is what this deployment can do with Word imports:
 * nothing ("off"); everything except processing ("reviewOnly": no worker is
 * deployed, so imports already processed can be reviewed and committed but no
 * new one can start); or all of it ("on"). It is "unknown" until the server
 * answers.
 */
export type ImportAvailability = "unknown" | "off" | "reviewOnly" | "on";

function availabilityOf(capabilities: ImportCapabilities): ImportAvailability {
  if (!capabilities.intakeEnabled) return "off";
  return capabilities.processingEnabled ? "on" : "reviewOnly";
}

/** CAPABILITIES_POLL_MS is how often a screen waiting on the worker re-reads the availability. */
export const CAPABILITIES_POLL_MS = 30_000;

/**
 * useImportAvailability subscribes to this deployment's Word import
 * availability. With a refetchInterval it is also re-read at that pace, so a
 * screen waiting on the worker notices processing being switched off.
 */
export function useImportAvailability(
  refetchInterval: number | false = false,
): ImportAvailability {
  const { data } = useQuery({
    ...importCapabilitiesQuery(),
    select: availabilityOf,
    refetchInterval,
  });
  return data ?? "unknown";
}

/** refreshAvailability re-reads the capabilities when the server refused processing as switched off. */
export function refreshAvailability(client: QueryClient, cause: unknown) {
  if (cause instanceof ApiError && cause.code === "IMPORT_PROCESSING_UNAVAILABLE")
    void client.invalidateQueries({ queryKey: importCapabilitiesQuery().queryKey });
}
