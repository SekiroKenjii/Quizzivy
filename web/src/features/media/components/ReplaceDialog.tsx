import type { RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Replace } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import {
  replaceMedia,
  type LibraryAsset,
  type MediaReplacement,
} from "@/features/media/api";
import { UploadStatus } from "@/features/media/components/UploadStatus";
import { acceptAttribute } from "@/features/media/limits";
import { quickCheck } from "@/features/media/probe";
import { rejectionMessage, useMediaUpload } from "@/features/media/useMediaUpload";

type ReplaceValues = { file: File | null };

const INITIAL: ReplaceValues = { file: null };

/**
 * ReplaceDialog replaces a library file with a new one of the same kind
 * (DG-09): bank questions and drafts get the new file, published versions keep
 * theirs. The file is checked against the kind's limits before it is sent.
 * A replacement is not idempotent, so a failure is never retried: `onSettled`
 * runs after every answer, failures included, for the caller to re-read what
 * may have changed. Focus goes to `returnFocus` when the dialog closes.
 */
export function ReplaceDialog({
  returnFocus,
  asset,
  onOpenChange,
  onSettled,
}: Readonly<{
  asset: LibraryAsset | null;
  returnFocus: RefObject<HTMLElement | null>;
  onOpenChange: (open: boolean) => void;
  onSettled: (replacement: MediaReplacement | null) => void;
}>) {
  const { t } = useTranslation();
  const kind = asset?.kind ?? "audio";
  const upload = useMediaUpload<MediaReplacement>({
    kind,
    onUploaded: (replacement) => {
      onOpenChange(false);
      toast(t("media.replaced"));
      onSettled(replacement);
    },
  });
  const fields: readonly FormField<ReplaceValues>[] = [
    {
      kind: "file",
      name: "file",
      required: true,
      accept: acceptAttribute(kind),
      limits: t(kind === "image" ? "media.limitsImage" : "media.limitsAudio"),
    },
  ];

  return (
    <FormDialog<ReplaceValues>
      returnFocus={returnFocus}
      open={asset !== null}
      onOpenChange={(next) => {
        upload.reset();
        onOpenChange(next);
      }}
      title={t("media.replaceTitle")}
      description={t("media.replaceDescription")}
      icon={Replace}
      initial={INITIAL}
      fields={fields}
      validate={(values) => {
        if (values.file === null) return null;
        const rejection = quickCheck(values.file, kind);
        return rejection === null ? null : { file: rejectionMessage(t, rejection) };
      }}
      submitLabel={t("media.replaceSubmit")}
      pending={upload.busy}
      onSubmit={(values) => {
        if (values.file === null || asset === null) return;
        const id = asset.id;
        void upload.start(values.file, {
          send: async (file, options) => {
            try {
              return await replaceMedia(id, file, options);
            } catch (cause) {
              onSettled(null);
              throw cause;
            }
          },
        });
      }}
    >
      <UploadStatus state={upload.state} kind={kind} onCancel={upload.cancel} />
    </FormDialog>
  );
}
