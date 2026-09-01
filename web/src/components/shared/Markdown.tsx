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
}: {
  children: string;
  className?: string;
  /** Run after sanitising, so a plugin adds only markup this app authored. */
  plugins?: Options["rehypePlugins"];
}) {
  return (
    <div className={cn("space-y-2 leading-relaxed", className)}>
      <ReactMarkdown rehypePlugins={[rehypeSanitize, ...(plugins ?? [])]}>
        {children}
      </ReactMarkdown>
    </div>
  );
}
