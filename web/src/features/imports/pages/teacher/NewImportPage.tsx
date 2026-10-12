import { useTranslation } from "react-i18next";
import { Link, useNavigate, useSearchParams } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
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
  const pasting = sourceMode === "paste";
  const header = (
    <PageHead
      title={pasting ? t("imports.paste.title") : t("imports.newTitle")}
      description={pasting ? t("imports.paste.sub") : t("imports.newHint")}
      crumb={pasting}
    />
  );
  if (availability === "reviewOnly" || availability === "off")
    return (
      <>
        {header}
        <EmptyState
          hint={t("imports.availability.newOffHint")}
          action={
            <Button asChild size="sm" variant="outline">
              <Link to="/teacher/imports">{t("imports.backToHistory")}</Link>
            </Button>
          }
        >
          {t("imports.availability.newOff")}
        </EmptyState>
      </>
    );
  return (
    <div className="flex w-full min-w-0 flex-col gap-4.5" data-scale="deck">
      {header}
      <Segmented
        size="lg"
        scroll
        className="self-start [&>button]:min-w-[168px] [&>button]:justify-center"
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
      />
      <SourceIntake
        existing={null}
        deck
        fileMode={!pasting}
        retention={retention}
        enabled={processing}
        privacy={
          <Callout
            icon={ShieldCheck}
            lead={t("imports.upload.privacyLead")}
            className="gap-3 [&>svg]:size-[17px]"
          >
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
