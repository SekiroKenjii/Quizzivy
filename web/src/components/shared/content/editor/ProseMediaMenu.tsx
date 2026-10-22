import type { Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import { AudioLines, Image as ImageIcon, Library, MapPin, Upload } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { editorShortcut } from "./shortcuts";
import { ToolButton } from "./ToolButton";
import {
  placeAudioMarker,
  type MediaSource,
  type ProseMediaMenuProps,
} from "./proseMedia";

const backToText = new WeakSet<Editor>();

type Item = Readonly<{
  key: string;
  icon: typeof Upload;
  label: string;
  hint: string;
  disabled?: boolean;
  run: () => void;
}>;

/**
 * ProseMediaMenu is the toolbar's "Image" or "Audio" with its menu, as the
 * 2026-10-04 editor draws it: "Upload from device" and "Choose from Media",
 * each with its hint, over a note on how the file is used. In a prompt,
 * "Audio" is the question's own recording (D-064): it attaches one and places
 * it at the caret, and once the question has audio it offers "Place the
 * question's audio here" first, which moves the one marker.
 */
export function ProseMediaMenu({
  editor,
  kind,
  field,
  host,
  ...button
}: Readonly<ProseMediaMenuProps>) {
  const { t } = useTranslation();
  const label = t(
    kind === "image" ? "contentEditor.media.image" : "contentEditor.media.audio",
  );
  const choose = (source: MediaSource) => host.choose({ kind, source, field, editor });
  const prompt = field === "prompt" && kind === "audio";
  const blocked = prompt ? host.audioBlocked : null;
  const items: Item[] = [];
  if (prompt && host.questionAudio !== null)
    items.push({
      key: "place",
      icon: MapPin,
      label: t("contentEditor.media.placeAudio"),
      hint: t("contentEditor.media.placeAudioHint"),
      run: () => {
        if (placeAudioMarker(editor)) backToText.add(editor);
      },
    });
  items.push(
    {
      key: "device",
      icon: Upload,
      label: t("contentEditor.media.upload"),
      hint: t(
        kind === "image"
          ? "contentEditor.media.imageLimits"
          : "contentEditor.media.audioLimits",
      ),
      disabled: blocked !== null,
      run: () => choose("device"),
    },
    {
      key: "library",
      icon: Library,
      label: t("contentEditor.media.library"),
      hint: t("contentEditor.media.libraryHint"),
      disabled: blocked !== null,
      run: () => choose("library"),
    },
  );
  let note: string;
  if (kind === "image")
    note = t("contentEditor.media.imageNote", { shortcut: editorShortcut("V").label });
  else if (blocked !== null) note = blocked;
  else
    note = t(
      prompt
        ? "contentEditor.media.promptAudioNote"
        : "contentEditor.media.explanationAudioNote",
    );
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <ToolButton
          icon={kind === "image" ? ImageIcon : AudioLines}
          label={label}
          title={label}
          menu
          {...button}
        />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="w-70"
        onCloseAutoFocus={(event) => {
          if (!backToText.delete(editor)) return;
          event.preventDefault();
          editor.commands.focus(undefined, { scrollIntoView: false });
        }}
      >
        {items.map((item) => (
          <DropdownMenuItem
            key={item.key}
            disabled={item.disabled ?? false}
            className="group data-[disabled]:text-foreground/50 items-start py-2 data-[disabled]:opacity-100"
            onSelect={item.run}
          >
            <item.icon aria-hidden="true" className="mt-px size-4!" />
            <span className="min-w-0">
              <span className="block text-[13px] font-medium">{item.label}</span>
              <span className="text-muted-fg group-data-[disabled]:text-foreground/40 block text-[11.5px] leading-normal">
                {item.hint}
              </span>
            </span>
          </DropdownMenuItem>
        ))}
        <p className="text-muted-fg mt-1 border-t px-2 pt-2 pb-1 text-[11.5px] leading-normal">
          {note}
        </p>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
