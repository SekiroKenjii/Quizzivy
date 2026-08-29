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

function TabsList({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.List>) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      className={cn("bg-muted inline-flex gap-0.5 rounded-lg p-[0.1875rem]", className)}
      {...props}
    />
  );
}

function TabsTrigger({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Trigger>) {
  return (
    <TabsPrimitive.Trigger
      data-slot="tabs-trigger"
      className={cn(
        "text-muted-foreground focus-visible:ring-ring data-[state=active]:bg-background data-[state=active]:text-foreground inline-flex h-7 items-center gap-1.5 rounded-md border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors focus-visible:ring-2 focus-visible:outline-none data-[state=active]:shadow-[0_1px_2px_0_oklch(0_0_0/0.06)]",
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
