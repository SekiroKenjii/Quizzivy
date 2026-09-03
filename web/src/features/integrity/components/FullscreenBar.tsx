import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { enterFullscreen, fullscreenSupported } from "../fullscreen";

/**
 * Fullscreen exit is not one of S-07's dialogs. It is this inline bar, because
 * Esc must always work and a modal over an exited fullscreen is a trap
 * (§10.2). The paper below stays writable; the bar only offers the way back.
 *
 * On a browser with no element fullscreen at all -- every iPhone -- the button
 * would be a promise nothing can keep, so the bar says so instead.
 */
export function FullscreenBar() {
  const { t } = useTranslation();
  const supported = fullscreenSupported();

  return (
    <div className="bg-muted/30 border-b px-4 py-2">
      <div className="mx-auto flex w-full max-w-[720px] items-center gap-3">
        <p className="text-muted-foreground flex-1 text-xs leading-relaxed">
          {t(
            supported
              ? "integrity.fullscreenExited"
              : "integrity.fullscreenUnsupported",
          )}
        </p>
        {supported && (
          <Button variant="outline" size="sm" onClick={() => void enterFullscreen()}>
            {t("integrity.fullscreenReturn")}
          </Button>
        )}
      </div>
    </div>
  );
}
