import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeader } from "@/components/shared/PageHeader";
import { SourceIntake } from "../components/SourceIntake";
import { storeImport } from "../queries";

export default function NewImportPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  return (
    <>
      <PageHeader
        title={t("imports.newTitle")}
        backTo="/admin/imports"
        backLabel={t("imports.backToHistory")}
      />
      <div className="mx-auto max-w-2xl">
        <Card>
          <CardHeader>
            <CardTitle>{t("imports.newHeading")}</CardTitle>
            <CardDescription>{t("imports.newHint")}</CardDescription>
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
