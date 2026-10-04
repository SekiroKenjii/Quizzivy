import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { LoaderCircle } from "lucide-react";

import {
  DeckDialog,
  DeckDialogActions,
  DeckDialogCancel,
} from "@/components/shared/DeckDialog";
import { Button } from "@/components/ui/button";
import { learnsOnly } from "@/features/auth/permissions";
import { myClassesQuery } from "@/features/classes/api";
import { ApiError } from "@/lib/api/errors";
import { notify } from "@/lib/toast";
import { useAuthStore } from "@/stores/auth";
import { joinClass } from "../api";
import { toneFor, useJoinLookup } from "../useJoinLookup";
import { ClassPreviewCard } from "./ClassPreviewCard";
import { JoinCodeField } from "./JoinCodeField";

const ERROR_ID = "join-dialog-error";

const LIST_WAIT_MS = 3000;

/**
 * JoinDialog is how a signed-in student joins a class by its code (§6.2). A
 * complete code is looked up once it rests, and the class it finds is shown
 * before anything is joined; every refusal reads as the one line that names
 * no class. A student already in the class is told so and cannot join again.
 * Joining refreshes the class and assignment lists, says "You joined {name}"
 * and closes the dialog, with focus back on what opened it, or on the page
 * when that control has gone. It waits for the class list so the new class is
 * on screen under the toast, and for no more than three seconds. The typed
 * code and the state of a join are forgotten when the dialog closes; a join
 * still on its way then finishes without closing a dialog opened since.
 */
export function JoinDialog({
  open,
  onOpenChange,
}: Readonly<{ open: boolean; onOpenChange: (open: boolean) => void }>) {
  const { t } = useTranslation();
  return (
    <DeckDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("student.joinClass")}
      description={t("join.subtitle")}
      placement="top"
    >
      <JoinForm onClose={() => onOpenChange(false)} />
    </DeckDialog>
  );
}

function JoinForm({ onClose }: Readonly<{ onClose: () => void }>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const student = useAuthStore((s) => learnsOnly(s.user));
  const join = useMutation({
    mutationFn: (code: string) => joinClass(code),
    onSuccess: async (joined) => {
      void queryClient.invalidateQueries({ queryKey: ["my-assignments"] });
      await Promise.race([
        queryClient.invalidateQueries({ queryKey: myClassesQuery.queryKey }),
        new Promise((resolve) => setTimeout(resolve, LIST_WAIT_MS)),
      ]);
      notify.success(t("join.dialog.joined", { className: joined.name }));
    },
    onError: (cause, code) => {
      if (cause instanceof ApiError && cause.status === 404)
        queryClient.setQueryData(["join-preview", code], null);
    },
  });
  const [code, setCode] = useState("");
  const lookup = useJoinLookup(code);
  const classes = useQuery(myClassesQuery);
  const sent = (join.variables ?? null) === lookup.code;
  const joining = (join.isPending || join.isSuccess) && sent;
  const waiting = lookup.found !== undefined && classes.isPending;
  const member =
    !joining &&
    lookup.found !== undefined &&
    classes.data?.items.some((c) => c.id === lookup.found?.classId) === true;
  const found = waiting || member ? undefined : lookup.found;
  const failed = sent && !join.isPending ? join.error : null;

  let message = lookup.message;
  let retryAfter = lookup.retryAfter;
  if (member) message = t("join.dialog.alreadyMember");
  else if (failed instanceof ApiError && failed.isRateLimited) {
    message = failed.message;
    retryAfter = failed.retryAfterSeconds ?? null;
  } else if (failed !== null && message === null) message = t("join.failed");
  const ready = found !== undefined && student;

  let status = "";
  if (lookup.checking || waiting) status = t("join.checking");
  else if (found)
    status = t("join.dialog.found", {
      className: found.className,
      teacherName: found.teacherName,
    });

  return (
    <form
      noValidate
      className="flex flex-col gap-3.5"
      onSubmit={(event) => {
        event.preventDefault();
        if (ready && !joining && lookup.code !== null)
          join.mutate(lookup.code, { onSuccess: onClose });
      }}
    >
      <JoinCodeField
        size="dialog"
        value={code}
        onChange={setCode}
        tone={toneFor(message, found !== undefined)}
        errorId={message ? ERROR_ID : undefined}
      />
      {message && (
        <p id={ERROR_ID} role="alert" className="text-danger-ink -mt-1.5 text-sm">
          {message}
          {retryAfter !== null && ` ${t("join.dialog.retryIn", { count: retryAfter })}`}
        </p>
      )}
      <p role="status" className="sr-only">
        {status}
      </p>
      {found && (
        <ClassPreviewCard
          variant="row"
          name={found.className}
          teacherName={found.teacherName}
        />
      )}
      {found && !student && (
        <p className="text-muted-fg -mt-1.5 text-sm">{t("join.studentsOnly")}</p>
      )}
      <DeckDialogActions>
        <DeckDialogCancel onClick={onClose}>{t("common.cancel")}</DeckDialogCancel>
        <Button
          type="submit"
          size="lg"
          aria-disabled={!ready || undefined}
          aria-busy={joining || undefined}
          className="aria-disabled:opacity-50"
        >
          {joining && (
            <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
          )}
          {t("join.dialog.submit")}
        </Button>
      </DeckDialogActions>
    </form>
  );
}
