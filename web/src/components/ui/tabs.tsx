import type * as React from "react";
import { Tabs as TabsPrimitive } from "radix-ui";

import { cn } from "@/lib/utils";

// Matches the deck's `.tabs` / `.tab` / `.tab.is-active` (kit.css).
function Tabs({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Root>) {
  return <TabsPrimitive.Root data-slot="tabs" className={className} {...props} />;
}

/**
 * TabsList is the row of tabs. On a deck surface it is the deck's underlined
 * row: 20px between tabs, a 1px line beneath, and sideways scroll when the
 * tabs do not fit.
 */
function TabsList({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.List>) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      className={cn(
        "bg-muted inline-flex gap-0.5 rounded-lg p-[0.1875rem]",
        "in-data-[scale=deck]:flex in-data-[scale=deck]:gap-5 in-data-[scale=deck]:overflow-x-auto in-data-[scale=deck]:rounded-none in-data-[scale=deck]:border-b in-data-[scale=deck]:bg-transparent in-data-[scale=deck]:p-0",
        className,
      )}
      {...props}
    />
  );
}

/**
 * TabsTrigger is one tab. On a deck surface it is text only, with a 2px line
 * under the selected tab, and its focus ring is drawn inside its own box so
 * the scrolling row cannot clip it.
 */
function TabsTrigger({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Trigger>) {
  return (
    <TabsPrimitive.Trigger
      data-slot="tabs-trigger"
      className={cn(
        "text-muted-foreground data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-card inline-flex h-7 items-center gap-1.5 rounded-md border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors",
        "in-data-[scale=deck]:text-muted-fg in-data-[scale=deck]:text-ui in-data-[scale=deck]:h-auto in-data-[scale=deck]:rounded-none in-data-[scale=deck]:px-0.5 in-data-[scale=deck]:pb-2.5 in-data-[scale=deck]:leading-4.5 in-data-[scale=deck]:whitespace-nowrap in-data-[scale=deck]:-outline-offset-2!",
        "in-data-[scale=deck]:data-[state=active]:text-fg in-data-[scale=deck]:data-[state=active]:bg-transparent in-data-[scale=deck]:data-[state=active]:shadow-[inset_0_-2px_0_var(--fg)]",
        className,
      )}
      {...props}
    />
  );
}

function TabsContent({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Content>) {
  return (
    <TabsPrimitive.Content data-slot="tabs-content" className={className} {...props} />
  );
}

export { Tabs, TabsList, TabsTrigger, TabsContent };
