import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "~/components/ui/sheet";
import { Badge } from "~/components/ui/badge";
import type { AuditLogEntry } from "~/lib/api";

interface AuditLogDetailSheetProps {
  entry: AuditLogEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  if (!value) return null;
  return (
    <div className="grid grid-cols-[120px_1fr] gap-2 py-2 border-b border-border last:border-0">
      <span className="text-sm font-medium text-muted-foreground">{label}</span>
      <span className="text-sm break-all">{value}</span>
    </div>
  );
}

export function AuditLogDetailSheet({
  entry,
  open,
  onOpenChange,
}: AuditLogDetailSheetProps) {
  if (!entry) return null;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-[480px] sm:max-w-[480px] overflow-y-auto">
        <SheetHeader>
          <SheetTitle>Audit Log Entry</SheetTitle>
          <SheetDescription>
            <Badge variant="outline" className="font-mono text-xs">
              {entry.id}
            </Badge>
          </SheetDescription>
        </SheetHeader>

        <div className="mt-6 space-y-1">
          <DetailRow
            label="Timestamp"
            value={new Date(entry.created_at).toLocaleString()}
          />
          <DetailRow
            label="Action"
            value={<Badge>{entry.action}</Badge>}
          />
          <DetailRow label="Category" value={entry.category} />
          <DetailRow label="Description" value={entry.description} />
          <DetailRow
            label="Actor Type"
            value={entry.actor_type}
          />
          <DetailRow
            label="Actor"
            value={
              entry.actor_type === "system"
                ? "System"
                : entry.actor_email || "Unknown"
            }
          />
          {entry.actor_id && (
            <DetailRow
              label="Actor ID"
              value={
                <span className="font-mono text-xs">{entry.actor_id}</span>
              }
            />
          )}
          <DetailRow label="Resource Type" value={entry.resource_type} />
          {entry.resource_id && (
            <DetailRow
              label="Resource ID"
              value={
                <span className="font-mono text-xs">{entry.resource_id}</span>
              }
            />
          )}
          {entry.project_id && (
            <DetailRow
              label="Project ID"
              value={
                <span className="font-mono text-xs">{entry.project_id}</span>
              }
            />
          )}
          <DetailRow
            label="IP Address"
            value={entry.ip_address || "\u2014"}
          />
          <DetailRow
            label="User Agent"
            value={entry.user_agent || "\u2014"}
          />
          <DetailRow
            label="Sequence"
            value={
              <span className="font-mono text-xs">
                #{entry.sequence_num}
              </span>
            }
          />
          <DetailRow
            label="Entry Hash"
            value={
              <span className="font-mono text-xs">{entry.entry_hash}</span>
            }
          />
          {entry.prev_hash && (
            <DetailRow
              label="Previous Hash"
              value={
                <span className="font-mono text-xs">{entry.prev_hash}</span>
              }
            />
          )}
          {entry.metadata &&
            Object.keys(entry.metadata).length > 0 && (
              <div className="py-2">
                <span className="text-sm font-medium text-muted-foreground">
                  Metadata
                </span>
                <pre className="mt-1 rounded-md bg-muted p-3 text-xs font-mono overflow-x-auto">
                  {JSON.stringify(entry.metadata, null, 2)}
                </pre>
              </div>
            )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
