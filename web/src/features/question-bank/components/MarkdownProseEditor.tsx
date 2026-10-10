import {
  useDeferredValue,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import type { Options } from "react-markdown";
import { Markdown } from "@/components/shared/Markdown";
import { EDITOR_BOX, frameStyle } from "@/components/shared/content/editor/frame";
import { cn } from "@/lib/utils";
import { markdownWords } from "../markdownActions";
import { MarkdownToolbar, type MarkdownAction } from "./MarkdownToolbar";
import "@/components/shared/content/content.css";

type View = "write" | "preview";

const VIEWS: readonly View[] = ["write", "preview"];

function ViewTabs({
  id,
  label,
  view,
  onView,
}: Readonly<{ id: string; label: string; view: View; onView: (view: View) => void }>) {
  const { t } = useTranslation();
  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    event.preventDefault();
    const next = view === "write" ? "preview" : "write";
    onView(next);
    document.getElementById(`${id}-tab-${next}`)?.focus();
  }
  return (
    <div
      role="tablist"
      aria-label={t("markdownEditor.views", { label })}
      className="bg-muted ml-auto flex flex-none gap-0.5 rounded-[7px] p-0.5"
    >
      {VIEWS.map((option) => {
        const on = option === view;
        return (
          <button
            key={option}
            type="button"
            role="tab"
            id={`${id}-tab-${option}`}
            aria-selected={on}
            aria-controls={`${id}-panel`}
            tabIndex={on ? 0 : -1}
            onClick={() => onView(option)}
            onKeyDown={onKeyDown}
            className={cn(
              "h-6.5 rounded-[5px] px-[9px] text-xs font-medium whitespace-nowrap",
              on ? "bg-card text-fg shadow-card ring-border ring-1" : "text-muted-fg",
            )}
          >
            {t(`markdownEditor.${option}`)}
          </button>
        );
      })}
    </div>
  );
}

/** MarkdownProseEditorProps are the Markdown body's field, its frame and its optional replacement body. */
export type MarkdownProseEditorProps = {
  id: string;
  label: string;
  describedBy?: string | undefined;
  value: string;
  onChange: (value: string) => void;
  minHeight: number;
  fontSize: number;
  clearOnFocus?: boolean | undefined;
  placeholder?: string | undefined;
  previewPlugins?: Options["rehypePlugins"];
  focusOnMount?: boolean | undefined;
  replacement?: ReactNode;
};

/**
 * MarkdownProseEditor is the Markdown mode's box: the toolbar with "Write |
 * Preview" at its end, a monospace textarea or the preview the student's
 * reader draws, and the footer with the syntax hint and the words count.
 * `replacement` takes the body's place and hides the views. `clearOnFocus`
 * empties the builder's starter prompt on first focus without saving.
 */
export function MarkdownProseEditor({
  id,
  label,
  describedBy,
  value,
  onChange,
  minHeight,
  fontSize,
  clearOnFocus = false,
  placeholder,
  previewPlugins,
  focusOnMount = false,
  replacement,
}: Readonly<MarkdownProseEditorProps>) {
  const { t } = useTranslation();
  const field = useRef<HTMLTextAreaElement>(null);
  const selection = useRef<{ start: number; end: number } | null>(null);
  const untouched = useRef(clearOnFocus && value === t("builder.starterPrompt"));
  const [cleared, setCleared] = useState(false);
  const [view, setView] = useState<View>("write");
  const displayed = cleared && value === t("builder.starterPrompt") ? "" : value;
  const deferred = useDeferredValue(displayed);
  const words = useMemo(() => markdownWords(deferred), [deferred]);

  useEffect(() => {
    if (focusOnMount) field.current?.focus();
  }, [focusOnMount]);

  useLayoutEffect(() => {
    const pending = selection.current;
    if (!pending || !field.current) return;
    selection.current = null;
    field.current.focus();
    field.current.setSelectionRange(pending.start, pending.end);
  }, [displayed, view]);

  function edit(action: MarkdownAction) {
    const start = field.current?.selectionStart ?? displayed.length;
    const end = field.current?.selectionEnd ?? displayed.length;
    const next = action(displayed, start, end);
    selection.current = { start: next.start, end: next.end };
    untouched.current = false;
    setView("write");
    onChange(next.value);
  }

  return (
    <div className={EDITOR_BOX} style={frameStyle(minHeight, fontSize)}>
      <div className="bg-sidebar flex items-center gap-2 rounded-t-[10px] border-b px-1.5 py-[5px]">
        <MarkdownToolbar label={label} disabled={replacement != null} onEdit={edit} />
        {replacement == null && (
          <ViewTabs id={id} label={label} view={view} onView={setView} />
        )}
      </div>
      {replacement ?? (
        <div role="tabpanel" id={`${id}-panel`} aria-labelledby={`${id}-tab-${view}`}>
          {view === "write" ? (
            <textarea
              placeholder={placeholder}
              ref={field}
              id={id}
              aria-describedby={describedBy}
              value={displayed}
              spellCheck={false}
              onFocus={() => {
                if (!untouched.current) return;
                untouched.current = false;
                setCleared(true);
              }}
              onChange={(event) => {
                untouched.current = false;
                setCleared(false);
                onChange(event.target.value);
              }}
              style={{ minHeight }}
              className="content-markdown text-fg block w-full resize-y border-0 bg-transparent px-4 py-3 font-mono text-[13.5px] leading-[1.65] outline-none"
            />
          ) : (
            <div className="content-editor-body">
              {displayed.trim() ? (
                <Markdown plugins={previewPlugins}>{displayed}</Markdown>
              ) : (
                <p className="text-muted-fg">{t("markdownEditor.previewEmpty")}</p>
              )}
            </div>
          )}
        </div>
      )}
      <div className="text-muted-fg flex flex-wrap justify-between gap-x-3 gap-y-1 border-t px-3 py-1.5 text-[11.5px] leading-normal">
        <span className="min-w-0">{t("markdownEditor.footerHint")}</span>
        <span className="whitespace-nowrap tabular-nums">
          {t("contentEditor.words", { count: words })}
        </span>
      </div>
    </div>
  );
}
