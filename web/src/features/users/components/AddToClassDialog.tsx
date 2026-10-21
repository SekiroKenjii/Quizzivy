import { useState } from "react";
import { useTranslation } from "react-i18next";
import { GraduationCap, LoaderCircle } from "lucide-react";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { Button } from "@/components/ui/button";
import { addMember } from "@/features/classes/api";
import type { AdminUser, ClassChoice } from "@/features/users/api";
import { ClassPicker } from "@/features/users/components/ClassPicker";
import type { BulkFailure } from "@/features/users/components/CredentialResult";
import { ApiError } from "@/lib/api/errors";

const PARALLEL = 4;

const PAUSED = "aria-disabled:bg-primary/55 aria-disabled:hover:bg-primary/55";

function codeOf(cause: unknown): string {
  if (!(cause instanceof ApiError)) return "INTERNAL";
  return cause.status === 404 ? "NOT_A_STUDENT" : cause.code;
}

async function addAll(
  classId: string,
  users: readonly AdminUser[],
  onStep: () => void,
): Promise<BulkFailure[]> {
  const failures: BulkFailure[] = [];
  const queue = [...users];
  async function next(): Promise<void> {
    const user = queue.shift();
    if (user === undefined) return;
    try {
      await addMember(classId, user.id);
    } catch (cause) {
      failures.push({ userId: user.id, fullName: user.fullName, code: codeOf(cause) });
    }
    onStep();
    return next();
  }
  await Promise.all(Array.from({ length: Math.min(PARALLEL, users.length) }, next));
  return failures;
}

/**
 * AddToClassDialog adds the `selected` accounts to one class the Admin picks
 * ("Thêm {n} người vào một lớp"), one `addClassMember` each, a few at a time.
 * An account already in the class stays as it is. The server refuses every
 * account that is not an active student. While it runs it counts the accounts
 * done, and Cancel, Esc and the close button wait for it. `onFinished` receives the class
 * and the accounts it did not take, each with its reason.
 */
export function AddToClassDialog({
  open,
  selected,
  onClose,
  onFinished,
}: Readonly<{
  open: boolean;
  selected: readonly AdminUser[];
  onClose: () => void;
  onFinished: (choice: ClassChoice, failures: BulkFailure[]) => void;
}>) {
  const { t } = useTranslation();
  const [choice, setChoice] = useState<ClassChoice | null>(null);
  const [pending, setPending] = useState(false);
  const [done, setDone] = useState(0);
  const [error, setError] = useState<string | null>(null);

  function finish() {
    if (pending) return;
    setChoice(null);
    setError(null);
    onClose();
  }

  async function add() {
    if (pending) return;
    if (choice === null) {
      setError(t("users.bulk.addToClassPick"));
      return;
    }
    setPending(true);
    setDone(0);
    setError(null);
    const failures = await addAll(choice.id, selected, () =>
      setDone((steps) => steps + 1),
    );
    setPending(false);
    setChoice(null);
    onFinished(choice, failures);
  }

  return (
    <DialogShell
      open={open}
      width={480}
      onOpenChange={(next) => {
        if (!next) finish();
      }}
    >
      <DialogShellHeader
        icon={GraduationCap}
        title={t("users.bulk.addToClassTitle", { count: selected.length })}
        description={t("users.bulk.addToClassBody")}
      />
      <DialogShellBody>
        {open ? (
          <ClassPicker
            label={t("users.classPicker.label")}
            value={choice}
            onChange={(next) => {
              setChoice(next);
              setError(null);
            }}
          />
        ) : null}
        <p aria-live="polite" className="text-danger-ink text-xs empty:hidden">
          {error}
        </p>
        <p role="status" className="text-muted-fg text-meta empty:hidden">
          {pending ? t("users.bulk.adding", { done, total: selected.length }) : null}
        </p>
      </DialogShellBody>
      <DialogShellFooter>
        <Button
          type="button"
          variant="outline"
          aria-disabled={pending || undefined}
          className="aria-disabled:text-muted-fg aria-disabled:cursor-default"
          onClick={finish}
        >
          {t("common.cancel")}
        </Button>
        <Button
          type="button"
          aria-disabled={pending || choice === null || undefined}
          className={PAUSED}
          onClick={() => void add()}
        >
          {pending ? (
            <LoaderCircle
              aria-hidden="true"
              className="animate-spin motion-reduce:animate-none"
            />
          ) : null}
          {t("users.bulk.addToClassSubmit")}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
