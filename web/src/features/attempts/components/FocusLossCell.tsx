import { Badge } from "@/components/ui/badge";

/** G-02's "Rời trang" cell: a dash before the first save, the count, warning ink once flagged. */
export function FocusLossCell({
  count,
  flagged,
}: Readonly<{ count: number | null | undefined; flagged: boolean | undefined }>) {
  if (count == null) return <span className="text-muted-foreground">—</span>;
  if (flagged) {
    return (
      <Badge variant="warning" className="tabular-nums">
        {count}
      </Badge>
    );
  }
  return <>{count}</>;
}
