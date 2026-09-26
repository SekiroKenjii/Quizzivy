import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { EmptyState } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { useImportAvailability, useImportRetention } from "../availability";
import { SourceIntake } from "../components/SourceIntake";
import { storeImport } from "../queries";

export default function NewImportPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const processing = useImportAvailability() !== "reviewOnly";
  const retention = useImportRetention();
  const header = (
    <PageHeader
      title={t("imports.newTitle")}
      backTo="/admin/imports"
      backLabel={t("imports.backToHistory")}
    />
  );
  if (!processing)
    return (
      <>
        {header}
        <EmptyState
          hint={t("imports.availability.newOffHint")}
          action={
            <Button asChild size="sm" variant="outline">
              <Link to="/admin/imports">{t("imports.backToHistory")}</Link>
            </Button>
          }
        >
          {t("imports.availability.newOff")}
        </EmptyState>
      </>
    );
  return (
    <>
      {header}
      <div className="mx-auto max-w-2xl">
        <Card>
          <CardHeader>
            <CardTitle>{t("imports.newHeading")}</CardTitle>
            <CardDescription>{t("imports.newHint")}</CardDescription>
            {retention === undefined ? null : (
              <p className="text-muted-foreground text-xs leading-relaxed">
                {t("imports.retention.policy", {
                  afterCommit: retention.afterCommitDays,
                  afterCancel: retention.afterCancelDays,
                  idle: retention.idleDays,
                })}
              </p>
            )}
          </CardHeader>
          <CardContent className="pt-1">
            <SourceIntake
              existing={null}
              onStarted={(started) => {
                void storeImport(client, started);
                void navigate(`/admin/imports/${started.id}`);
              }}
            />
          </CardContent>
        </Card>
      </div>
    </>
  );
}
