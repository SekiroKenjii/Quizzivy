import ReactMarkdown, { type Options } from "react-markdown";
import rehypeSanitize from "rehype-sanitize";

import { cn } from "@/lib/utils";
import { gfmSubset } from "@/components/shared/content/markdownGfm";

const REMARK_PLUGINS: Options["remarkPlugins"] = [gfmSubset];
const TABLE =
  "[&_table]:block [&_table]:max-w-full [&_table]:overflow-x-auto [&_table]:border-collapse [&_td]:border [&_td]:px-2.5 [&_td]:py-1.5 [&_td]:align-top [&_th]:border [&_th]:bg-muted [&_th]:px-2.5 [&_th]:py-1.5 [&_th]:text-start [&_th]:align-top [&_th]:font-semibold";

/** Renders a prompt written in Markdown, with GitHub's tables and `~~strikethrough~~` and no other GFM syntax. */
export function Markdown({
  children,
  className,
  plugins = [],
  components,
}: Readonly<{
  children: string;
  className?: string;
  /** Run after sanitising, so a plugin adds only markup this app authored. */
  plugins?: Options["rehypePlugins"];
  // Element overrides, for the markup a plugin above introduced.
  components?: Options["components"];
}>) {
  return (
    <div className={cn("space-y-2 leading-relaxed", TABLE, className)}>
      <ReactMarkdown
        remarkPlugins={REMARK_PLUGINS}
        rehypePlugins={[rehypeSanitize, ...(plugins ?? [])]}
        {...(components ? { components } : {})}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
