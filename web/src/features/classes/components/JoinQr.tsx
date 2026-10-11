import type { Ref } from "react";
import { useTranslation } from "react-i18next";
import { QRCodeSVG } from "qrcode.react";
import { joinLinkOf } from "@/features/classes/joinLink";
import { QR_INK, QR_PAPER } from "@/features/classes/joinQrImage";

/**
 * JoinQr is a class's join link as a QR code on a paper-white square, the
 * code under it in mono and the link under that, as the deck's QR dialog
 * draws it. Its ref is the frame `downloadJoinQr` rasterises.
 */
export function JoinQr({
  code,
  ref,
}: Readonly<{ code: string; ref?: Ref<HTMLDivElement> | undefined }>) {
  const { t } = useTranslation();
  const link = joinLinkOf(code);
  return (
    <div className="flex flex-col items-center gap-2.5 py-2">
      <div
        ref={ref}
        className="bg-paper text-paper-fg grid size-50 place-items-center rounded-xl border"
      >
        <QRCodeSVG
          value={link}
          size={150}
          level="M"
          bgColor={QR_PAPER}
          fgColor={QR_INK}
          role="img"
          aria-label={t("classes.qrLabel", { code })}
        />
      </div>
      <span className="text-stat font-mono font-semibold tracking-[.1em]">{code}</span>
      <span className="text-muted-fg text-meta max-w-full text-center wrap-anywhere">
        {link}
      </span>
    </div>
  );
}
