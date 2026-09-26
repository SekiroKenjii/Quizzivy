import type { ImportRun, ImportStatus, WordImport } from "./api";

/** IMPORT_STATUSES lists every import status in the order the history filter offers them. */
export const IMPORT_STATUSES: readonly ImportStatus[] = [
  "awaiting_sources",
  "queued",
  "processing",
  "needs_review",
  "committing",
  "committed",
  "failed",
  "cancelled",
];

/** IMPORT_POLL_MS is how often a screen re-reads an import the server is still working on, while its tab is visible. */
export const IMPORT_POLL_MS = 2000;

const ACTIVE: ReadonlySet<ImportStatus> = new Set([
  "queued",
  "processing",
  "committing",
]);

/** isActiveStatus reports whether the server is still working on the import. */
export function isActiveStatus(status: ImportStatus): boolean {
  return ACTIVE.has(status);
}

/** isImportStatus narrows an untrusted URL value to a known status. */
export function isImportStatus(value: string | null): value is ImportStatus {
  return value !== null && (IMPORT_STATUSES as readonly string[]).includes(value);
}

/** PROCESSING_STAGES names the teacher-facing stages and the run stages each one covers. */
export const PROCESSING_STAGES = [
  { key: "validate", runStages: ["source_validation"] },
  { key: "read", runStages: ["normalization", "extraction"] },
  { key: "structure", runStages: ["recognition"] },
  { key: "answers", runStages: ["recognition"] },
  { key: "check", runStages: ["validation"] },
] as const satisfies readonly {
  key: string;
  runStages: readonly ImportRun["stage"][];
}[];

export type StageState = "done" | "current" | "waiting";

const RUN_ORDER: readonly ImportRun["stage"][] = [
  "queued",
  "source_validation",
  "normalization",
  "extraction",
  "recognition",
  "validation",
  "ready",
];

/**
 * stageStates places a run's stage on the teacher-facing stages. A run that is
 * waiting to be retried has no current stage.
 */
export function stageStates(
  stage: ImportRun["stage"] | undefined,
  waiting = false,
): StageState[] {
  const position = RUN_ORDER.indexOf(stage ?? "queued");
  return PROCESSING_STAGES.map(({ runStages }) => {
    const covered = runStages.map((value) => RUN_ORDER.indexOf(value));
    if (covered.includes(position)) return waiting ? "waiting" : "current";
    return Math.max(...covered) < position ? "done" : "waiting";
  });
}

/** isWaitingToRetry reports whether a run failed transiently and is queued for another attempt. */
export function isWaitingToRetry(run: ImportRun | undefined): boolean {
  return run?.status === "queued" && run.errorCode !== undefined;
}

const RUN_ERROR_GROUPS: Readonly<Record<string, readonly string[]>> = {
  SOURCE_INVALID: ["SOURCE_INVALID"],
  SOURCE_UNSUPPORTED: ["SOURCE_UNSUPPORTED"],
  SOURCE_TOO_LARGE: ["SOURCE_TOO_LARGE"],
  LEGACY_CONVERSION_UNAVAILABLE: ["LEGACY_CONVERSION_UNAVAILABLE"],
  PDF_NO_TEXT: ["PDF_NO_TEXT"],
  PDF_PROTECTED: ["PDF_PROTECTED"],
  PDF_INVALID: ["PDF_INVALID"],
  PDF_TOO_LARGE: ["PDF_TOO_LARGE"],
  CONVERSION_FAILED: ["CONVERSION_FAILED"],
  STORAGE_INTEGRITY_FAILED: ["STORAGE_INTEGRITY_FAILED"],
  STOPPED: ["PRIVATE_ANSWER_EVIDENCE"],
  STORAGE_QUOTA_EXCEEDED: ["STORAGE_QUOTA_EXCEEDED"],
  PIPELINE_RETIRED: ["PIPELINE_RETIRED"],
  TEMPORARY: [
    "STORAGE_UNAVAILABLE",
    "CONVERSION_BUSY",
    "CONVERSION_TIMEOUT",
    "PROCESSING_TIMEOUT",
    "WORKER_INTERRUPTED",
    "WORKER_LEASE_LOST",
    "WORKER_LEASE_EXPIRED",
    "PROVIDER_UNAVAILABLE",
    "PROVIDER_RATE_LIMITED",
  ],
  INTERNAL: [
    "CONVERSION_CLEANUP_FAILED",
    "PROCESSOR_OUTPUT_INVALID",
    "PROVIDER_OUTPUT_INVALID",
    "PROCESSING_FAILED",
  ],
};

/** RUN_ERROR_KEYS lists every message key under imports.runError, UNKNOWN included. */
export const RUN_ERROR_KEYS: readonly string[] = [
  ...Object.keys(RUN_ERROR_GROUPS),
  "UNKNOWN",
];

const RUN_ERROR_KEY = new Map(
  Object.entries(RUN_ERROR_GROUPS).flatMap(([key, codes]) =>
    codes.map((code) => [code, key] as const),
  ),
);

const NEEDS_NEW_FILE: ReadonlySet<string> = new Set([
  "SOURCE_INVALID",
  "SOURCE_UNSUPPORTED",
  "SOURCE_TOO_LARGE",
  "LEGACY_CONVERSION_UNAVAILABLE",
  "PDF_NO_TEXT",
  "PDF_PROTECTED",
  "PDF_INVALID",
  "PDF_TOO_LARGE",
  "CONVERSION_FAILED",
  "STORAGE_INTEGRITY_FAILED",
  "PRIVATE_ANSWER_EVIDENCE",
]);

/** runErrorKey is the i18n key under imports.runError that explains a failed run's error code. */
export function runErrorKey(errorCode: string | undefined): string {
  return (
    (errorCode === undefined ? undefined : RUN_ERROR_KEY.get(errorCode)) ?? "UNKNOWN"
  );
}

/** isRetryable reports whether processing the same files again can succeed; otherwise the teacher needs another file. */
export function isRetryable(errorCode: string | undefined): boolean {
  return errorCode === undefined || !NEEDS_NEW_FILE.has(errorCode);
}

/** hasDraft reports whether processing has already produced a reviewable draft for the import. */
export function hasDraft(value: WordImport): boolean {
  return (value.draftRevision ?? 0) > 0;
}

/** reprocessOutcome is how the latest reprocess of an import under review ended, when it did not finish. */
export function reprocessOutcome(value: WordImport): "failed" | "cancelled" | null {
  const status = value.run?.status;
  if (value.status !== "needs_review" || !hasDraft(value)) return null;
  return status === "failed" || status === "cancelled" ? status : null;
}

/** importHref is the import's own page, whatever its status. */
export function importHref(value: WordImport): string {
  return `/admin/imports/${value.id}`;
}

/** importActionHref is where a history row's action leads for the import's current state. */
export function importActionHref(value: WordImport): string {
  if (value.status === "committed" && value.testId !== undefined)
    return `/admin/tests/${value.testId}/edit`;
  if (value.status === "needs_review") return `/admin/imports/${value.id}/review`;
  return importHref(value);
}
