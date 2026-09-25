import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";

/** ContentImage offers an explicit recovery action when a signed image cannot load. */
export function ContentImage({
  src,
  alt,
  onRetry,
}: Readonly<{ src: string; alt: string; onRetry?: (() => void) | undefined }>) {
  const { t } = useTranslation();
  const [failedFor, setFailedFor] = useState<string | null>(null);
  if (failedFor === src)
    return (
      <div className="flex flex-col items-start gap-2">
        <p role="status" className="text-sm">
          {t("preview.materialUnavailable")}
        </p>
        {onRetry ? (
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setFailedFor(null);
              onRetry();
            }}
          >
            {t("common.retry")}
          </Button>
        ) : null}
      </div>
    );
  return (
    <img
      src={src}
      alt={alt}
      loading="lazy"
      onError={() => setFailedFor(src)}
      className="h-auto max-w-full rounded-md"
    />
  );
}
