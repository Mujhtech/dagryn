import { createFileRoute, Link } from "@tanstack/react-router";
import { useCallback, useEffect, useState } from "react";
import { useProject, useProjectAuditLogs } from "~/hooks/queries";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import { Icons } from "~/components/icons";
import { AuditLogDetailSheet } from "~/components/audit-log-detail";
import { generateMetadata } from "~/lib/metadata";
import type { AuditLogEntry } from "~/lib/api";
import type { AuditLogParams } from "~/hooks/queries/use-team-audit-logs";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/audit-logs",
)({
  component: ProjectAuditLogsPage,
  head: () => {
    return generateMetadata({ title: "Project Audit Logs" });
  },
});

const CATEGORY_OPTIONS = [
  { value: "", label: "All categories" },
  { value: "auth", label: "Auth" },
  { value: "team", label: "Team" },
  { value: "project", label: "Project" },
  { value: "member", label: "Member" },
  { value: "billing", label: "Billing" },
  { value: "audit", label: "Audit" },
];

function categoryColor(category: string) {
  switch (category) {
    case "auth":
      return "default";
    case "team":
      return "secondary";
    case "project":
      return "outline";
    case "member":
      return "secondary";
    case "billing":
      return "destructive";
    case "audit":
      return "outline";
    default:
      return "secondary";
  }
}

function formatRelativeTime(dateStr: string) {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return "just now";
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 30) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}

function ProjectAuditLogsPage() {
  const { projectId } = Route.useParams();
  const { data: project, isLoading: projectLoading } = useProject(projectId);

  const [category, setCategory] = useState("");
  const [actorEmail, setActorEmail] = useState("");
  const [since, setSince] = useState("");
  const [until, setUntil] = useState("");
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [allEntries, setAllEntries] = useState<AuditLogEntry[]>([]);
  const [selectedEntry, setSelectedEntry] = useState<AuditLogEntry | null>(null);

  const params: AuditLogParams = {
    ...(category ? { category } : {}),
    ...(actorEmail ? { actor_email: actorEmail } : {}),
    ...(since ? { since: new Date(since).toISOString() } : {}),
    ...(until ? { until: new Date(until).toISOString() } : {}),
    ...(cursor ? { cursor } : {}),
    limit: 50,
  };

  const { data, isLoading, error } = useProjectAuditLogs(projectId, params);

  // Accumulate entries for "load more" pagination
  useEffect(() => {
    if (data) {
      if (cursor) {
        setAllEntries((prev) => {
          const existingIds = new Set(prev.map((e) => e.id));
          const newEntries = data.data.filter((e) => !existingIds.has(e.id));
          return [...prev, ...newEntries];
        });
      } else {
        setAllEntries(data.data);
      }
    }
  }, [data, cursor]);

  const handleFilterChange = useCallback(() => {
    setCursor(undefined);
    setAllEntries([]);
  }, []);

  if (projectLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!project) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>Project not found</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="sm" asChild>
          <Link
            to="/projects/$projectId"
            params={{ projectId }}
          >
            <Icons.ChevronLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Audit Log
          </h1>
          <p className="text-muted-foreground text-sm">
            Activity history for {project.name}
          </p>
        </div>
      </div>

      <Card className="py-3">
        <CardHeader>
          <CardTitle>Audit Log</CardTitle>
          <CardDescription>
            All activity recorded for this project
          </CardDescription>
        </CardHeader>
        <CardContent>
          {/* Filter bar */}
          <div className="flex flex-wrap gap-3 mb-4">
            <Select
              value={category}
              onValueChange={(value) => {
                setCategory(value === "all" ? "" : value);
                handleFilterChange();
              }}
            >
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="All categories" />
              </SelectTrigger>
              <SelectContent>
                {CATEGORY_OPTIONS.map((opt) => (
                  <SelectItem
                    key={opt.value || "all"}
                    value={opt.value || "all"}
                  >
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <div className="relative">
              <Icons.Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Filter by actor email"
                className="pl-8 h-9 w-[220px]"
                value={actorEmail}
                onChange={(e) => {
                  setActorEmail(e.target.value);
                  handleFilterChange();
                }}
              />
            </div>
            <Input
              type="date"
              className="h-9 w-[160px]"
              value={since}
              onChange={(e) => {
                setSince(e.target.value);
                handleFilterChange();
              }}
              placeholder="Since"
            />
            <Input
              type="date"
              className="h-9 w-[160px]"
              value={until}
              onChange={(e) => {
                setUntil(e.target.value);
                handleFilterChange();
              }}
              placeholder="Until"
            />
          </div>

          {/* Table */}
          {isLoading && !allEntries.length ? (
            <div className="flex items-center justify-center h-32">
              <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : error ? (
            <p className="text-sm text-destructive">
              {(error as Error).message}
            </p>
          ) : allEntries.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No audit log entries found.
            </p>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Timestamp</TableHead>
                    <TableHead>Actor</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead>Resource</TableHead>
                    <TableHead>IP Address</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {allEntries.map((entry) => (
                    <TableRow
                      key={entry.id}
                      className="cursor-pointer hover:bg-muted/50"
                      onClick={() => setSelectedEntry(entry)}
                    >
                      <TableCell
                        className="text-muted-foreground text-sm whitespace-nowrap"
                        title={new Date(entry.created_at).toLocaleString()}
                      >
                        {formatRelativeTime(entry.created_at)}
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">
                          {entry.actor_type === "system"
                            ? "System"
                            : entry.actor_email || "Unknown"}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            categoryColor(entry.category) as
                              | "default"
                              | "secondary"
                              | "outline"
                              | "destructive"
                          }
                        >
                          {entry.action}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm">
                        {entry.resource_type}
                        {entry.resource_id
                          ? ` (${entry.resource_id.slice(0, 8)}...)`
                          : ""}
                      </TableCell>
                      <TableCell className="text-muted-foreground text-sm">
                        {entry.ip_address || "\u2014"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              {/* Load more */}
              {data?.has_next && (
                <div className="flex justify-center mt-4">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setCursor(data.next_cursor)}
                    disabled={isLoading}
                  >
                    {isLoading ? (
                      <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                    ) : null}
                    Load more
                  </Button>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <AuditLogDetailSheet
        entry={selectedEntry}
        open={!!selectedEntry}
        onOpenChange={(open) => { if (!open) setSelectedEntry(null); }}
      />
    </div>
  );
}
