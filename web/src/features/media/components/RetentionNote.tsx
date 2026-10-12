import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Download } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { Button } from "@/components/ui/button";
import type { LibraryAsset } from "@/features/media/api";

/**
 * RetentionNote tells, before a replace or a delete, what becomes of the old
 * file, and offers its download while the signed link on the row still works:
 * once the file is deleted for good nothing restores it.
 */
export function RetentionNote({
  asset,
  children,
}: Readonly<{
  asset: Pick<LibraryAsset, "url" | "displayName">;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  return (
    <Callout
      tone="info"
      size="md"
      actions={
        <Button variant="outline" size="sm" asChild>
          <a
            href={asset.url}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t("media.downloadNamed", { name: asset.displayName })}
          >
            <Download aria-hidden="true" />
            {t("media.download")}
          </a>
        </Button>
      }
    >
      {children}
    </Callout>
  );
}
