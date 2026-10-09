import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Upload } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import { uploadMedia, type MediaAsset, type MediaKind } from "@/features/media/api";
import { UploadStatus } from "@/features/media/components/UploadStatus";
import { playLimitLabel } from "@/features/media/format";
import { acceptAttribute, kindOfName } from "@/features/media/limits";
import { quickCheck } from "@/features/media/probe";
import { rejectionMessage, useMediaUpload } from "@/features/media/useMediaUpload";

type UploadValues = {
  kind: MediaKind;
  file: File | null;
  plays: string;
};

const PLAY_LIMITS = ["0", "1", "2", "3"] as const;

/**
 * UploadDialog is the deck's "Upload media": the kind, the file and, for
 * audio, the default play limit the file is stored with. The file is checked
 * against its kind's limits before it is sent, and the dialog shows the
 * upload's progress until the server answers; closing it cancels the upload.
 * `initialFile` opens it with a file already chosen, as a drop on the page
 * does.
 */
export function UploadDialog({
  open,
  initialFile,
  onOpenChange,
  onUploaded,
}: Readonly<{
  open: boolean;
  initialFile: File | null;
  onOpenChange: (open: boolean) => void;
  onUploaded: (asset: MediaAsset) => void;
}>) {
  const { t } = useTranslation();
  const [sending, setSending] = useState<MediaKind>("audio");
  const upload = useMediaUpload({
    onUploaded: (asset: MediaAsset) => {
      onOpenChange(false);
      toast(t("media.uploaded", { name: asset.originalFilename }));
      onUploaded(asset);
    },
  });
  const fileField = (kind: MediaKind): FormField<UploadValues> => ({
    kind: "file",
    name: "file",
    label: t("media.file"),
    required: true,
    accept: acceptAttribute(kind),
    limits: t(kind === "image" ? "media.limitsImage" : "media.limitsAudio"),
    when: (values) => values.kind === kind,
  });
  const fields: readonly FormField<UploadValues>[] = [
    {
      kind: "seg",
      name: "kind",
      label: t("media.type"),
      options: [
        { value: "audio", label: t("media.kindAudio") },
        { value: "image", label: t("media.kindImage") },
      ],
    },
    fileField("audio"),
    fileField("image"),
    {
      kind: "select",
      name: "plays",
      label: t("media.playLimit"),
      options: PLAY_LIMITS.map((value) => ({
        value,
        label: playLimitLabel(Number(value), t),
      })),
      when: (values) => values.kind === "audio",
    },
  ];

  return (
    <FormDialog<UploadValues>
      open={open}
      onOpenChange={(next) => {
        upload.reset();
        onOpenChange(next);
      }}
      title={t("media.uploadTitle")}
      description={t("media.uploadDescription")}
      icon={Upload}
      initial={{
        kind:
          initialFile === null ? "audio" : (kindOfName(initialFile.name) ?? "audio"),
        file: initialFile,
        plays: "0",
      }}
      fields={fields}
      validate={(values) => {
        if (values.file === null) return null;
        const rejection = quickCheck(values.file, values.kind);
        return rejection === null ? null : { file: rejectionMessage(t, rejection) };
      }}
      submitLabel={t("media.upload")}
      pending={upload.busy}
      onSubmit={(values) => {
        if (values.file === null) return;
        const plays = values.kind === "audio" ? Number(values.plays) : undefined;
        setSending(values.kind);
        void upload.start(values.file, {
          kind: values.kind,
          send: (file, options) =>
            uploadMedia(file, { ...options, defaultMaxPlays: plays }),
        });
      }}
    >
      <UploadStatus state={upload.state} kind={sending} onCancel={upload.cancel} />
    </FormDialog>
  );
}
