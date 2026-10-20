import type * as React from "react";
import { Tabs as TabsPrimitive } from "radix-ui";

import { revealWithin } from "@/lib/revealWithin";
import { cn } from "@/lib/utils";

// Matches the deck's `.tabs` / `.tab` / `.tab.is-active` (kit.css).
function Tabs({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Root>) {
  return <TabsPrimitive.Root data-slot="tabs" className={className} {...props} />;
}

/**
 * TabsList is the row of tabs, the deck's underlined row: 20px between tabs, a
 * 1px line beneath, and sideways scroll when the tabs do not fit. A tab that takes focus while the row's edge cuts it is
 * scrolled into view.
 */
function TabsList({
  className,
  onFocus,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.List>) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      onFocus={(event) => {
        onFocus?.(event);
        revealWithin(event.currentTarget, event.target);
      }}
      className={cn(
        "flex gap-5 overflow-x-auto rounded-none border-b bg-transparent p-0",
        className,
      )}
      {...props}
    />
  );
}

/**
 * TabsTrigger is one tab: text only, with a 2px line
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
        "inline-flex items-center gap-1.5 border-0 bg-transparent font-medium transition-colors",
        "text-muted-fg text-ui h-auto rounded-none px-0.5 pb-2.5 leading-4.5 whitespace-nowrap -outline-offset-2!",
        "data-[state=active]:text-fg data-[state=active]:bg-transparent data-[state=active]:shadow-[inset_0_-2px_0_var(--fg)]",
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
