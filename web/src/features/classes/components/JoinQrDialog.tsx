import { useRef, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Download, Hash, Link2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { InfoField } from "@/components/shared/form/fields/InfoField";
import { JoinQr } from "@/features/classes/components/JoinQr";
import { useCopyJoinCode, useCopyJoinLink } from "@/features/classes/joinLink";
import { downloadJoinQr } from "@/features/classes/joinQrImage";

/**
 * JoinQrDialog is a join code's QR at full size, the code and the link under
 * it, in the class detail's dialog frame with Done to close: the projector
 * view (420px) and, with `share`, the New join code dialog (480px), which
 * adds the rows that copy the link, copy the code and download the QR. It is
 * open while `code` is set, traps focus, and gives it back to `returnFocus`,
 * or to what opened it.
 */
export function JoinQrDialog({
  code,
  title,
  description,
  share = false,
  onClose,
  returnFocus,
}: Readonly<{
  code: string | null;
  title: string;
  description: string;
  share?: boolean;
  onClose: () => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const frame = useRef<HTMLDivElement>(null);
  const copyLink = useCopyJoinLink();
  const copyCode = useCopyJoinCode();
  return (
    <DialogShell
      open={code !== null}
      onOpenChange={(open) => !open && onClose()}
      width={share ? 480 : 420}
      returnFocus={returnFocus}
    >
      <DialogShellHeader title={title} description={description} />
      <DialogShellBody>
        {code === null ? null : (
          <>
            <JoinQr ref={frame} code={code} />
            {share ? (
              <InfoField
                rows={[
                  {
                    icon: Link2,
                    text: t("classDetail.joinLinkRow"),
                    action: {
                      label: t("classDetail.copyLink"),
                      onAction: () => copyLink(code),
                    },
                  },
                  {
                    icon: Hash,
                    text: t("classDetail.codeRow"),
                    action: {
                      label: t("classDetail.copyCode"),
                      onAction: () => copyCode(code),
                    },
                  },
                  {
                    icon: Download,
                    text: t("classes.qrImage"),
                    action: {
                      label: t("classes.download"),
                      onAction: () => downloadJoinQr(frame.current, code),
                    },
                  },
                ]}
              />
            ) : null}
          </>
        )}
      </DialogShellBody>
      <DialogShellFooter>
        <Button
          type="button"
          className="text-ui h-9 px-4 leading-normal shadow-none"
          onClick={onClose}
        >
          {t("classDetail.done")}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
