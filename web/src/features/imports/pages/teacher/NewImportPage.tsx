import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { EmptyState } from "@/components/shared/ListState";
import { PageHead } from "@/layouts/shell/PageHead";
import { Segmented } from "@/components/ui/segmented";
import { Callout } from "@/components/shared/Callout";
import { ClipboardPaste, FileUp, ShieldCheck } from "lucide-react";
import { useImportAvailability, useImportRetention } from "../../availability";
import { SourceIntake } from "../../components/SourceIntake";
import { storeImport } from "../../queries";

export default function NewImportPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const availability = useImportAvailability();
  const processing = availability === "on";
  const [params, setParams] = useSearchParams();
  const sourceMode = params.get("source") === "paste" ? "paste" : "file";
  const retention = useImportRetention();
  const header = (
    <PageHead
      title={t("imports.newTitle")}
      back={{ to: "/teacher/imports", label: t("imports.backToHistory") }}
    />
  );
  if (availability === "reviewOnly" || availability === "off")
    return (
      <>
        {header}
        <EmptyState hint={t("imports.availability.newOffHint")}>
          {t("imports.availability.newOff")}
        </EmptyState>
      </>
    );
  return (
    <div
      className="mx-auto flex w-full max-w-[860px] min-w-0 flex-col gap-4.5"
      data-scale="deck"
    >
      {header}
      <Segmented
        fill
        label={t("imports.upload.source")}
        value={sourceMode}
        options={[
          { value: "file", label: t("imports.upload.fileMode"), icon: FileUp },
          {
            value: "paste",
            label: t("imports.upload.pasteMode"),
            icon: ClipboardPaste,
          },
        ]}
        onChange={(value) =>
          setParams(
            (current) => {
              const next = new URLSearchParams(current);
              if (value === "paste") next.set("source", "paste");
              else next.delete("source");
              return next;
            },
            { replace: true },
          )
        }
        className="[&_button]:min-w-[168px]"
      />
      {sourceMode === "paste" ? (
        <EmptyState hint={t("imports.upload.pasteDeferredHint")}>
          {t("imports.upload.pasteDeferred")}
        </EmptyState>
      ) : null}
      <SourceIntake
        existing={null}
        deck
        fileMode={sourceMode === "file"}
        retention={retention}
        enabled={processing && sourceMode === "file"}
        privacy={
          <Callout icon={ShieldCheck} lead={t("imports.upload.privacyLead")}>
            {t("imports.upload.privacy", {
              processing: processing
                ? t("imports.upload.processingOn")
                : t("imports.upload.processingUnknown"),
            })}
          </Callout>
        }
        onStarted={(started) => {
          void storeImport(client, started);
          void navigate(`/teacher/imports/${started.id}`);
        }}
      />
    </div>
  );
}
