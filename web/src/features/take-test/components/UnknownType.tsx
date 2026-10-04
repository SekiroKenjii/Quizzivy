import { useTranslation } from "react-i18next";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

function reloadPage() {
  window.location.reload();
}

/**
 * UnknownType stands in for a question whose type this page has no renderer
 * for, which happens when the page was loaded before a release that added
 * the type. It names the question, offers no way to answer, and reloads the
 * page, which brings the renderer. Answers already written are in the local
 * draft and on the server, so the reload loses nothing.
 */
export function UnknownType({
  number,
  onReload = reloadPage,
}: Readonly<{ number: number; onReload?: () => void }>) {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      className="bg-card flex flex-col items-start gap-3 rounded-[11px] border-[1.5px] p-4"
    >
      <span className="bg-warning-soft text-warning-ink grid size-10 place-items-center rounded-lg">
        <RefreshCw className="size-5" aria-hidden="true" />
      </span>
      <p className="text-base leading-[1.55] text-pretty">
        {t("takeTest.unknownType", { n: number })}
      </p>
      <Button onClick={onReload}>{t("takeTest.reloadPage")}</Button>
    </div>
  );
}
