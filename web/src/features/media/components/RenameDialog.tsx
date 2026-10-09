import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Pencil } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { updateMedia, type LibraryAsset } from "@/features/media/api";
import { failureMessage } from "@/lib/api/errors";

type RenameValues = { name: string };

const NAME_MAX = 200;

/**
 * RenameDialog sets the name the library shows for a file. The stored file
 * and its original filename never change, and neither does a question that
 * uses it. `onRenamed` runs once the server has the new name.
 */
export function RenameDialog({
  asset,
  onOpenChange,
  onRenamed,
}: Readonly<{
  asset: LibraryAsset | null;
  onOpenChange: (open: boolean) => void;
  onRenamed: () => void;
}>) {
  const { t } = useTranslation();
  const rename = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) =>
      updateMedia(id, { displayName: name }),
    onSuccess: () => {
      onOpenChange(false);
      onRenamed();
    },
  });
  const fields: readonly FormField<RenameValues>[] = [
    {
      kind: "text",
      name: "name",
      label: t("media.name"),
      required: true,
      maxLength: NAME_MAX,
    },
  ];

  return (
    <FormDialog<RenameValues>
      open={asset !== null}
      onOpenChange={(next) => {
        rename.reset();
        onOpenChange(next);
      }}
      title={t("media.renameTitle")}
      icon={Pencil}
      initial={{ name: asset?.displayName ?? "" }}
      fields={fields}
      validate={(values) =>
        values.name.trim() === "" ? { name: t("formDialog.required") } : null
      }
      submitLabel={t("media.rename")}
      pending={rename.isPending}
      error={
        rename.isError ? failureMessage(rename.error, t("media.renameFailed")) : null
      }
      onSubmit={(values) => {
        if (asset !== null) rename.mutate({ id: asset.id, name: values.name.trim() });
      }}
    />
  );
}
