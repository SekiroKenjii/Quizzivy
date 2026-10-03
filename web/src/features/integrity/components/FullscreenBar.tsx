import { useTranslation } from "react-i18next";
import { Maximize } from "lucide-react";
import { Button } from "@/components/ui/button";
import { NoticeBar } from "@/features/take-test/components/NoticeBar";
import { enterFullscreen, fullscreenSupported } from "../fullscreen";

/**
 * FullscreenBar is the bar under the engine's header while an assignment that
 * asks for fullscreen is not in it. It is a bar and never a dialog, because
 * Esc must always work and a modal over an exited fullscreen is a trap
 * (§10.2): the paper below stays writable and the bar only offers the way
 * back, in the warning tone with the deck's small bordered button, which
 * keeps the 44px floor below 1024. The click itself asks for fullscreen, since
 * a browser grants it only inside a gesture. A browser with no fullscreen gets
 * one muted sentence and no button.
 */
export function FullscreenBar() {
  const { t } = useTranslation();

  if (!fullscreenSupported()) {
    return (
      <NoticeBar icon={Maximize} tone="muted">
        {t("integrity.fullscreenUnsupported")}
      </NoticeBar>
    );
  }
  return (
    <NoticeBar
      icon={Maximize}
      action={
        <Button
          variant="outline"
          className="text-fg in-data-[scale=deck]:text-meta h-8 flex-none shadow-none in-data-[scale=deck]:px-2.5"
          onClick={() => void enterFullscreen()}
        >
          {t("integrity.fullscreenReturn")}
        </Button>
      }
    >
      {t("integrity.fullscreenExited")}
    </NoticeBar>
  );
}
