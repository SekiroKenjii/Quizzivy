import ReactMarkdown, { type Options } from "react-markdown";
import rehypeSanitize from "rehype-sanitize";

import { cn } from "@/lib/utils";

/** Renders a prompt written in Markdown. */
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
  // Element overrides, for the markup a plugin above introduced.
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
