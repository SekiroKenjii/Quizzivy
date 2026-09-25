/** ProcessingOffNotice says, calmly, why an action that needs the import worker is not offered. */
export function ProcessingOffNotice({ children }: Readonly<{ children: string }>) {
  return (
    <p role="status" className="bg-muted/40 rounded-md p-3 text-sm leading-relaxed">
      {children}
    </p>
  );
}
