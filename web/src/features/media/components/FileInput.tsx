import type { RefObject } from "react";
import { useTranslation } from "react-i18next";
import { acceptAttribute } from "../limits";
import { ANY_MEDIA_ACCEPT } from "../filePicker";
import type { MediaKind } from "../api";

/**
 * FileInput is the hidden file input a media control opens: named "Chọn tệp
 * từ máy", taking the `accept` of `kind` (either kind when it is "any"), and
 * cleared after each choice, so choosing the same file again is a new choice.
 * Its host clicks it through `inputRef`, or through openFilePicker.
 */
export function FileInput({
  inputRef,
  kind = "any",
  onFile,
}: Readonly<{
  inputRef: RefObject<HTMLInputElement | null>;
  kind?: MediaKind | "any";
  onFile: (file: File) => void;
}>) {
  const { t } = useTranslation();
  return (
    <input
      ref={inputRef}
      type="file"
      accept={kind === "any" ? ANY_MEDIA_ACCEPT : acceptAttribute(kind)}
      className="sr-only"
      tabIndex={-1}
      aria-label={t("media.chooseFile")}
      onChange={(event) => {
        const file = event.target.files?.[0];
        event.target.value = "";
        if (file) onFile(file);
      }}
    />
  );
}
