import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { deleteMedia, type LibraryAsset } from "@/features/media/api";
import {
  failureMessage,
  referencingTests,
  type ReferencingTest,
} from "@/lib/api/errors";

/**
 * DeleteMediaDialog asks before a library file is deleted, or, when a
 * published version uses it, says it cannot be and names those versions
 * (DG-09). A refusal from the server that names versions turns the dialog
 * into that notice. `onDeleted` runs once the file is gone.
 */
export function DeleteMediaDialog({
  asset,
  onOpenChange,
  onDeleted,
}: Readonly<{
  asset: LibraryAsset | null;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
}>) {
  const { t } = useTranslation();
  const remove = useMutation({
    mutationFn: (id: string) => deleteMedia(id),
    onSuccess: () => {
      onOpenChange(false);
      onDeleted();
    },
  });
  const close = (open: boolean) => {
    remove.reset();
    onOpenChange(open);
  };
  const refused = referencingTests(remove.error);
  const holders = refused.length > 0 ? refused : (asset?.usedIn ?? []);
  const blocked = refused.length > 0 || (asset?.usageCount ?? 0) > 0;
  const name = asset?.displayName ?? "";

  if (blocked)
    return (
      <ConfirmDialog
        open={asset !== null}
        onOpenChange={close}
        title={t("media.deleteBlockedTitle")}
        description={t("media.deleteBlockedBody", { name })}
        confirmLabel={t("media.understood")}
      >
        <VersionList tests={holders} />
      </ConfirmDialog>
    );

  return (
    <ConfirmDialog
      open={asset !== null}
      onOpenChange={close}
      title={t("media.deleteTitle", { name })}
      description={
        (asset?.questionCount ?? 0) > 0
          ? t("media.deleteUsedBody", { count: asset?.questionCount ?? 0 })
          : t("media.deleteUnusedBody")
      }
      confirmLabel={t("media.delete")}
      destructive
      pending={remove.isPending}
      error={
        remove.isError ? failureMessage(remove.error, t("media.deleteFailed")) : null
      }
      onConfirm={() => {
        if (asset !== null) remove.mutate(asset.id);
      }}
    />
  );
}

function VersionList({ tests }: Readonly<{ tests: readonly ReferencingTest[] }>) {
  const { t } = useTranslation();
  if (tests.length === 0) return null;
  return (
    <ul className="flex flex-col gap-1 text-sm">
      {tests.map((test) => (
        <li key={`${test.id}-${test.version ?? 0}`} className="min-w-0 break-words">
          <Link
            to={`/teacher/tests/${test.id}`}
            className="font-medium underline underline-offset-4"
          >
            {test.title}
          </Link>
          {test.version === undefined ? null : (
            <span className="text-muted-fg ml-1.5 tabular-nums">
              {t("tests.versionNumber", { n: test.version })}
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}
