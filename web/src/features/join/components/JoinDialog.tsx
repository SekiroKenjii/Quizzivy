import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { LoaderCircle } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
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

const FRAME =
  "bg-card shadow-float top-[12%] flex max-h-[calc(100%-2rem)] w-[min(27.5rem,calc(100%-1.5rem))] max-w-none translate-y-0 flex-col gap-3.5 overflow-y-auto rounded-2xl p-5.5 min-[768px]:top-[50%] min-[768px]:translate-y-[-50%] sm:max-w-none";

type Join = Readonly<{
  code: string | null;
  error: unknown;
  busy: boolean;
  done: boolean;
  send: (code: string) => void;
}>;

/**
 * JoinDialog is how a signed-in student joins a class by its code (§6.2). A
 * complete code is looked up once it rests, and the class it finds is shown
 * before anything is joined; every refusal reads as the one line that names
 * no class. A student already in the class is told so and cannot join again.
 * Joining refreshes the class and assignment lists, says "You joined {name}"
 * and closes the dialog, with focus back on what opened it, or on the page
 * when that control has gone. The typed code is forgotten when the dialog
 * closes.
 */
export function JoinDialog({
  open,
  onOpenChange,
}: Readonly<{ open: boolean; onOpenChange: (open: boolean) => void }>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const opener = useRef<HTMLElement | null>(null);
  const page = useRef<HTMLElement | null>(null);
  const join = useMutation({
    mutationFn: (code: string) => joinClass(code),
    onSuccess: async (joined) => {
      void queryClient.invalidateQueries({ queryKey: ["my-assignments"] });
      await queryClient.invalidateQueries({ queryKey: myClassesQuery.queryKey });
      notify.success(t("join.dialog.joined", { className: joined.name }));
      onOpenChange(false);
    },
    onError: (cause, code) => {
      if (cause instanceof ApiError && cause.status === 404)
        queryClient.setQueryData(["join-preview", code], null);
    },
  });
  const { reset } = join;

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent
        showCloseButton={false}
        className={FRAME}
        onOpenAutoFocus={() => {
          const active = document.activeElement;
          opener.current =
            active instanceof HTMLElement && active !== document.body ? active : null;
          page.current = opener.current?.closest("main") ?? null;
        }}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          const target = opener.current?.isConnected ? opener.current : page.current;
          opener.current = null;
          page.current = null;
          target?.focus();
        }}
      >
        <div>
          <DialogTitle className="text-lg leading-normal font-semibold">
            {t("student.joinClass")}
          </DialogTitle>
          <DialogDescription className="text-muted-fg mt-1 text-base leading-[1.55] text-pretty">
            {t("join.subtitle")}
          </DialogDescription>
        </div>
        <JoinForm
          join={{
            code: join.variables ?? null,
            error: join.error,
            busy: join.isPending,
            done: join.isSuccess,
            send: join.mutate,
          }}
          onCancel={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}

function JoinForm({ join, onCancel }: Readonly<{ join: Join; onCancel: () => void }>) {
  const { t } = useTranslation();
  const student = useAuthStore((s) => learnsOnly(s.user));
  const [code, setCode] = useState("");
  const lookup = useJoinLookup(code);
  const classes = useQuery(myClassesQuery);
  const joining = (join.busy || join.done) && join.code === lookup.code;
  const waiting = lookup.found !== undefined && classes.isPending;
  const member =
    !joining &&
    lookup.found !== undefined &&
    classes.data?.items.some((c) => c.id === lookup.found?.classId) === true;
  const found = waiting || member ? undefined : lookup.found;
  const failed = join.code === lookup.code && !join.busy ? join.error : null;

  let message = lookup.message;
  let retryAfter = lookup.retryAfter;
  if (member) message = t("join.dialog.alreadyMember");
  else if (failed instanceof ApiError && failed.isRateLimited) {
    message = failed.message;
    retryAfter = failed.retryAfterSeconds ?? null;
  } else if (failed !== null && failed !== undefined && message === null)
    message = t("join.failed");
  const ready = found !== undefined && student;

  return (
    <form
      noValidate
      className="flex flex-col gap-3.5"
      onSubmit={(event) => {
        event.preventDefault();
        if (ready && !joining && lookup.code !== null) join.send(lookup.code);
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
        {lookup.checking || waiting ? t("join.checking") : ""}
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
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          size="lg"
          className="font-medium"
          onClick={onCancel}
        >
          {t("common.cancel")}
        </Button>
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
      </div>
    </form>
  );
}
