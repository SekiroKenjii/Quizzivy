import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useBeforeUnload } from "react-router";
import { useTranslation } from "react-i18next";
import {
  skipToken,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { toast } from "@/components/ui/sonner";
import {
  USER_IMPORTS_KEY,
  USERS_KEY,
  createUserImport,
  previewUserImport,
  userImportQuery,
  type UserImportDefaultRole,
  type UserImportDelivery,
} from "@/features/users/api";
import { failureReason, isJobRunning, isTooLarge } from "@/features/users/importText";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";

/** IMPORT_POLL_MS is how often a running job is read while the Admin is there. */
export const IMPORT_POLL_MS = 2_000;

const PREVIEW_KEPT_MS = 5 * 60_000;

type Source = Readonly<{ file: File; pick: number }>;

function newRequestId() {
  return crypto.randomUUID();
}

/**
 * useUserImport holds one run of the Import users dialog: the file and the
 * choices, the preview they come to, and, once the Admin starts it, the job.
 * The preview is read again when the file, the role or the hand-out changes,
 * and the latest read wins. The `requestId` is made with the dialog and held
 * while the file stays the same, so a retry of a start whose answer was lost
 * returns the job it made; choosing another file takes a new one, because the
 * same id would return the first file's job whatever the new file holds. A
 * running job is read every two seconds while the
 * Admin is there, whether or not the dialog is open, until a read fails; when
 * it settles, the Users list is read again and, if the dialog is closed, a
 * toast says how it ended. While a sheet job runs, or its sheet waits to be
 * taken, closing or reloading the tab asks first, because the sheet is offered
 * only here. `dismiss` is called when the dialog closes: it forgets the draft,
 * and a settled job whose sheet can no longer be taken.
 */
export function useUserImport(open: boolean) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [source, setSource] = useState<Source | null>(null);
  const [defaultRole, setDefaultRole] = useState<UserImportDefaultRole>("student");
  const [delivery, setDelivery] = useState<UserImportDelivery>("email_link");
  const [requestId, setRequestId] = useState(newRequestId);
  const [jobId, setJobId] = useState<string | null>(null);

  const tooLarge = source !== null && isTooLarge(source.file);
  const asked = jobId === null && !tooLarge ? source : null;
  const preview = useQuery({
    queryKey: [
      ...USER_IMPORTS_KEY,
      "preview",
      asked?.pick ?? null,
      defaultRole,
      delivery,
    ] as const,
    queryFn:
      asked === null
        ? skipToken
        : ({ signal }) =>
            previewUserImport(asked.file, { defaultRole, delivery }, signal),
    retry: false,
    staleTime: Infinity,
    gcTime: PREVIEW_KEPT_MS,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });

  const starting = useRef(false);
  const start = useMutation({
    mutationFn: (input: { file: File }) =>
      createUserImport(input.file, { defaultRole, delivery }, requestId),
    onSuccess: (job) => {
      queryClient.setQueryData(userImportQuery(job.id).queryKey, job);
      setJobId(job.id);
    },
    onSettled: () => {
      starting.current = false;
    },
  });

  const interval = useIdlePolling(IMPORT_POLL_MS, jobId !== null);
  const job = useQuery({
    ...userImportQuery(jobId ?? ""),
    enabled: jobId !== null,
    refetchInterval: (query) =>
      query.state.status !== "error" && isJobRunning(query.state.data)
        ? interval
        : false,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(job.refetch, jobId !== null && isJobRunning(job.data));

  const settled = job.data !== undefined && !isJobRunning(job.data) ? job.data : null;
  const holdsSheet =
    job.data?.delivery === "sheet" &&
    (isJobRunning(job.data) ||
      (job.data.sheet?.available === true && job.data.sheet.downloadedAt === null));
  useBeforeUnload((event) => {
    if (holdsSheet) event.preventDefault();
  });
  const settledId = settled?.id ?? null;
  const announce = useEffectEvent(() => {
    void queryClient.invalidateQueries({ queryKey: USERS_KEY });
    if (open || settled === null) return;
    toast(
      settled.status === "failed"
        ? failureReason(settled, t)
        : t("users.import.summary", {
            created: t("users.import.created", { count: settled.created }),
            skipped: t("users.import.skipped", { count: settled.skipped }),
          }),
    );
  });
  useEffect(() => {
    if (settledId !== null) announce();
  }, [settledId]);

  function reset() {
    setSource(null);
    setDefaultRole("student");
    setDelivery("email_link");
    setRequestId(newRequestId());
    setJobId(null);
    start.reset();
  }

  return {
    file: source?.file ?? null,
    tooLarge,
    defaultRole,
    delivery,
    preview,
    start,
    jobId,
    job,
    choose(file: File | null) {
      start.reset();
      setRequestId(newRequestId());
      setSource((previous) =>
        file === null ? null : { file, pick: (previous?.pick ?? 0) + 1 },
      );
    },
    setDefaultRole(role: UserImportDefaultRole) {
      start.reset();
      setDefaultRole(role);
    },
    setDelivery(next: UserImportDelivery) {
      start.reset();
      setDelivery(next);
    },
    begin() {
      if (source === null || tooLarge || starting.current) return;
      starting.current = true;
      start.mutate({ file: source.file });
    },
    again: reset,
    dismiss() {
      const keepsSheet = settled !== null && settled.sheet?.available === true;
      if (jobId === null || (settled !== null && !keepsSheet)) reset();
    },
  };
}

export type UserImport = ReturnType<typeof useUserImport>;
