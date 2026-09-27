import { api } from "@/lib/api/client";
import type { MaintenanceWindow } from "@/stores/appState";

/** MaintenanceStatus is what `GET /public/status` says now. */
export type MaintenanceStatus =
  { kind: "open" } | { kind: "maintenance"; window: MaintenanceWindow };

/**
 * readStatus asks `GET /public/status` past the browser's cache, which the
 * route allows for thirty seconds. A window that has not started yet is open.
 */
export async function readStatus(signal?: AbortSignal): Promise<MaintenanceStatus> {
  const { maintenance } = await api("get", "/public/status", {
    cache: "no-store",
    ...(signal ? { signal } : {}),
  });
  if (maintenance === null || !maintenance.active) return { kind: "open" };
  return {
    kind: "maintenance",
    window: { startsAt: maintenance.startsAt, endsAt: maintenance.endsAt },
  };
}
