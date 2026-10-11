import { useRef, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { InfoField } from "@/components/shared/form/fields/InfoField";
import { JoinQr } from "@/features/classes/components/JoinQr";
import { downloadJoinQr } from "@/features/classes/joinQrImage";
import { useCopyJoinLink } from "@/features/classes/joinLink";

const FOOTER_BUTTON =
  "text-ui h-9 px-4 leading-normal shadow-none in-data-[scale=deck]:h-9 in-data-[scale=deck]:px-4";

/**
 * ShowQrDialog is the deck's "Show QR code" for one class: its name, how a
 * student joins, the QR with the code and the link, a row that downloads the
 * QR image "for slides or Zalo", and Close beside "Copy join link". It is
 * open while `target` is set; focus goes back to `returnFocus` when it closes.
 */
export function ShowQrDialog({
  target,
  onClose,
  returnFocus,
}: Readonly<{
  target: { name: string; code: string } | null;
  onClose: () => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const frame = useRef<HTMLDivElement>(null);
  const copyLink = useCopyJoinLink();
  return (
    <DialogShell
      open={target !== null}
      onOpenChange={(open) => !open && onClose()}
      width={420}
      returnFocus={returnFocus}
    >
      <DialogShellHeader
        title={target?.name ?? ""}
        description={t("classes.qrHint", { site: window.location.host })}
      />
      <DialogShellBody>
        {target === null ? null : (
          <>
            <JoinQr ref={frame} code={target.code} />
            <InfoField
              rows={[
                {
                  icon: Download,
                  text: t("classes.qrImage"),
                  action: {
                    label: t("classes.download"),
                    onAction: () => downloadJoinQr(frame.current, target.code),
                  },
                },
              ]}
            />
          </>
        )}
      </DialogShellBody>
      <DialogShellFooter>
        <Button
          type="button"
          variant="outline"
          className={`bg-card ${FOOTER_BUTTON} px-3.5 in-data-[scale=deck]:px-3.5`}
          onClick={onClose}
        >
          {t("common.close")}
        </Button>
        <Button
          type="button"
          className={FOOTER_BUTTON}
          onClick={() => target && copyLink(target.code)}
        >
          {t("classes.copyLink")}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
