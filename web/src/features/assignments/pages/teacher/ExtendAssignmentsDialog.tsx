import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Clock } from "lucide-react";
import { FormDialog } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import { extendAssignment, type Assignment } from "@/features/assignments/api";
import { ApiError } from "@/lib/api/errors";
import { nameOf } from "./assignmentWindow";

const STEPS = ["15", "30", "60", "1440"] as const;

type Failure = { id: string; title: string; message: string };

/**
 * ExtendAssignmentsDialog is "Extend deadline" for one assignment or for the
 * bulk bar's selection: Extend by 15 min, 30 min, 1 hour or 1 day (30 min for
 * one, 1 hour for several, as the deck's two forms default), and whether to
 * notify the students. It extends one assignment at a time; when some fail it
 * stays open on those alone, names each with the server's reason, and offers
 * to retry them, and its title counts those. `onExtended` hears the ids
 * that were extended. Focus returns to `returnFocus` when it closes.
 */
export function ExtendAssignmentsDialog({
  items,
  when,
  open,
  returnFocus,
  onOpenChange,
  onExtended,
}: Readonly<{
  items: readonly Assignment[];
  when?: string | undefined;
  open: boolean;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onOpenChange: (open: boolean) => void;
  onExtended: (ids: string[]) => void;
}>) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const [failures, setFailures] = useState<Failure[]>([]);
  const [opened, setOpened] = useState(open);
  if (opened !== open) {
    setOpened(open);
    if (open) setFailures([]);
  }
  const failedIds = new Set(failures.map((failure) => failure.id));
  const remaining =
    failures.length > 0 ? items.filter((item) => failedIds.has(item.id)) : items;
  const label = (minutes: string) => t(`assignments.list.by${minutes}`);

  async function extend(minutes: string, notify: boolean) {
    setPending(true);
    const done: string[] = [];
    const failed: Failure[] = [];
    const now = new Date();
    for (const item of remaining) {
      try {
        await extendAssignment(item.id, Number(minutes), notify);
        done.push(item.id);
      } catch (cause) {
        failed.push({
          id: item.id,
          title: nameOf(item, now, t),
          message: cause instanceof ApiError ? cause.message : t("common.actionFailed"),
        });
      }
    }
    setPending(false);
    setFailures(failed);
    if (done.length > 0) {
      onExtended(done);
      toast(
        done.length === 1
          ? t("assignments.list.extended", { by: label(minutes) })
          : t("assignments.list.extendedMany", {
              count: done.length,
              by: label(minutes),
            }),
      );
    }
    if (failed.length === 0) onOpenChange(false);
  }

  const single = items.length === 1 ? items[0] : undefined;
  return (
    <FormDialog
      open={open}
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next);
      }}
      icon={Clock}
      title={
        remaining.length === 1
          ? t("assignments.list.extendTitle")
          : t("assignments.list.extendTitleMany", { count: remaining.length })
      }
      description={
        single !== undefined && when !== undefined
          ? t("assignments.list.extendWhen", { title: single.testTitle, when })
          : undefined
      }
      initial={{ by: items.length > 1 ? "60" : "30", notify: true }}
      fields={[
        {
          kind: "seg",
          name: "by",
          label: t("assignments.list.extendBy"),
          options: STEPS.map((minutes) => ({ value: minutes, label: label(minutes) })),
        },
        {
          kind: "toggle",
          name: "notify",
          text: t("assignments.list.notify"),
          sub: t("assignments.list.notifySub"),
        },
      ]}
      submitLabel={
        failures.length > 0
          ? t("common.retryFailed", { count: failures.length })
          : t("assignments.list.extendSubmit")
      }
      pending={pending}
      returnFocus={returnFocus}
      onSubmit={({ by, notify }) => void extend(by, notify)}
    >
      {failures.length > 0 ? (
        <div role="alert" className="text-danger-ink flex flex-col gap-1 text-sm">
          <p>{t("assignments.list.failedItems", { count: failures.length })}</p>
          <ul className="flex flex-col gap-1">
            {failures.map((failure) => (
              <li key={failure.id}>
                {failure.title}: {failure.message}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </FormDialog>
  );
}
