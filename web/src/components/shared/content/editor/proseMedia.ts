import {
  createContext,
  useContext,
  type ComponentProps,
  type ComponentType,
  type ReactNode,
} from "react";
import type { NodeViewRenderer } from "@tiptap/core";
import type { Editor } from "@tiptap/react";
import type { Node as EditorNode } from "@tiptap/pm/model";
import { NodeSelection, type EditorState } from "@tiptap/pm/state";
import type { components } from "@/lib/api/schema";
import type { ProseField, QuestionMediaBlock } from "../questionContent";
import { QUESTION_MEDIA_LIMITS } from "../model";

export type MediaAsset = components["schemas"]["MediaAsset"];

/** MediaSource is where "Image" or "Audio" takes its file from: the device, or Media. */
export type MediaSource = "device" | "library";

/**
 * ProseMediaHost is what a question's editor asks its page for when a prompt
 * or an explanation holds media (DG-109): the signed files the prose names,
 * the question's own recording that a prompt's marker places, and the file
 * flows, which the page owns because they reach the question's media and the
 * media library. `audioBlocked` says why a prompt's "Audio" cannot attach a
 * recording, when it cannot; `status` is the field's upload in progress or
 * its failure, drawn under the field's text. The host also brings the
 * editor's media views and its "Image" and "Audio" menu, so a page that edits
 * no question media never loads them.
 */
export interface ProseMediaHost {
  asset: (id: string) => MediaAsset | undefined;
  questionAudio: MediaAsset | null;
  audioBlocked: string | null;
  choose: (
    request: Readonly<{
      kind: "image" | "audio";
      source: MediaSource;
      field: ProseField;
      editor: Editor;
    }>,
  ) => void;
  files: (files: readonly File[], field: ProseField, editor: Editor) => void;
  register: (field: ProseField, editor: Editor | null) => void;
  preview: (node: QuestionMediaBlock) => ReactNode;
  status: (field: ProseField) => ReactNode;
  nodeViews: Readonly<Record<"proseImage" | "proseAudio", NodeViewRenderer>>;
  Menu: ComponentType<ProseMediaMenuProps>;
}

/** ProseMediaMenuProps are what the toolbar gives the host's "Image" or "Audio" menu, beside its roving-focus props. */
export type ProseMediaMenuProps = Omit<
  ComponentProps<"button">,
  "children" | "title" | "onClick"
> & {
  editor: Editor;
  kind: "image" | "audio";
  field: ProseField;
  host: ProseMediaHost;
};

/** ProseMediaContext hands a page's ProseMediaHost to the editors of its question's prose. */
export const ProseMediaContext = createContext<ProseMediaHost | null>(null);

/** useProseMediaHost is the nearest ProseMediaHost, or null where no page offers one. */
export function useProseMediaHost(): ProseMediaHost | null {
  return useContext(ProseMediaContext);
}

function isMarker(node: EditorNode): boolean {
  return node.type.name === "proseAudio" && node.attrs["assetId"] == null;
}

function markerPosition(state: EditorState): number | null {
  let found: number | null = null;
  state.doc.forEach((node, offset) => {
    if (found === null && isMarker(node)) found = offset;
  });
  return found;
}

/** mediaRoom is how many more media blocks, the marker among them, an editor's document takes. */
export function mediaRoom(editor: Editor): number {
  let used = 0;
  editor.state.doc.forEach((node) => {
    if (node.type.name === "proseImage" || node.type.name === "proseAudio") used += 1;
  });
  return Math.max(0, QUESTION_MEDIA_LIMITS.blocks - used);
}

/** hasAudioMarker reports whether an editor's prompt holds the marker that places the question's audio. */
export function hasAudioMarker(editor: Editor): boolean {
  return markerPosition(editor.state) !== null;
}

function insertionPoint(state: EditorState): number {
  const selection = state.selection;
  if (selection instanceof NodeSelection) return selection.to;
  const { $from } = selection;
  return $from.depth > 1 ? $from.after(1) : $from.pos;
}

function dispatched(editor: Editor, run: () => void): boolean {
  const before = editor.state.doc;
  run();
  return !editor.state.doc.eq(before);
}

/**
 * insertProseMedia puts a media node among the document's own blocks at the
 * caret: after the list or table the caret is in, after a selected media
 * block, or splitting the paragraph. It reports whether the document took it;
 * the editor refuses a ninth media block or a second recording of another kind.
 */
export function insertProseMedia(
  editor: Editor,
  node: Readonly<{ type: "proseImage" | "proseAudio"; attrs: Record<string, unknown> }>,
): boolean {
  if (editor.isDestroyed) return false;
  return dispatched(editor, () => {
    const { state } = editor;
    const at = insertionPoint(state);
    const created = state.schema.nodes[node.type]!.create(node.attrs);
    editor.view.dispatch(state.tr.replaceRangeWith(at, at, created).scrollIntoView());
  });
}

/**
 * placeAudioMarker puts the marker that places the question's own audio at
 * the caret, in one undoable step, moving it there when the prompt already
 * holds it (D-064 ruling 3).
 */
export function placeAudioMarker(editor: Editor): boolean {
  if (editor.isDestroyed) return false;
  return dispatched(editor, () => {
    const { state } = editor;
    const at = insertionPoint(state);
    const old = markerPosition(state);
    const tr = state.tr;
    if (old !== null) tr.delete(old, old + 1);
    const target = tr.mapping.map(at);
    tr.replaceRangeWith(target, target, state.schema.nodes["proseAudio"]!.create());
    editor.view.dispatch(tr.scrollIntoView());
  });
}

/** removeAudioMarker takes the marker out of a prompt whose question no longer has audio of its own. */
export function removeAudioMarker(editor: Editor): boolean {
  if (editor.isDestroyed) return false;
  const at = markerPosition(editor.state);
  if (at === null) return false;
  return dispatched(editor, () => {
    editor.view.dispatch(editor.state.tr.delete(at, at + 1));
  });
}
