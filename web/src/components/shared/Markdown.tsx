import ReactMarkdown, { type Options } from "react-markdown";
import rehypeSanitize from "rehype-sanitize";

import { cn } from "@/lib/utils";

/**
 * Renders a prompt written in Markdown.
 *
 * rehype-sanitize is not optional and dangerouslySetInnerHTML is never used
 * (§2): a prompt is authored by a teacher and read by every student in the
 * class, so an unsanitised one is stored XSS aimed at exactly the people who
 * cannot avoid opening it.
 */
export function Markdown({
  children,
  className,
  plugins = [],
  components,
}: {
  children: string;
  className?: string;
  /** Run after sanitising, so a plugin adds only markup this app authored. */
  plugins?: Options["rehypePlugins"];
  /**
   * Element overrides, for the markup a plugin above introduced.
   *
   * Only reachable for elements that survived the sanitiser, so this cannot be
   * used to render something a teacher wrote -- it renders something this app
   * put in the tree after sanitising.
   */
  components?: Options["components"];
}) {
  return (
    <div className={cn("space-y-2 leading-relaxed", className)}>
      <ReactMarkdown
        rehypePlugins={[rehypeSanitize, ...(plugins ?? [])]}
        {...(components ? { components } : {})}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
