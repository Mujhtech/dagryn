import { Link } from "@tanstack/react-router";
import type { Project } from "~/lib/api";
import { useRunDashboardSummary, useRuns } from "~/hooks/queries";
import {
  RunStatusIcon,
  formatDuration,
} from "~/components/projects/run-detail/status-ui";
import { Card, CardContent } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Skeleton } from "~/components/ui/skeleton";
import { Icons } from "~/components/icons";
import { cn } from "~/lib/utils";

function statusBgColor(status: string): string {
  switch (status) {
    case "success":
      return "bg-green-500/10";
    case "failed":
      return "bg-red-500/10";
    case "running":
      return "bg-blue-500/10";
    case "pending":
      return "bg-yellow-500/10";
    case "cancelled":
      return "bg-gray-500/10";
    default:
      return "bg-gray-500/10";
  }
}

export function ProjectStatsCard({ project }: { project: Project }) {
  const { data: summary, isLoading: summaryLoading } = useRunDashboardSummary(
    project.id,
    7,
  );
  const { data: runsData, isLoading: runsLoading } = useRuns(project.id, 1, 1);

  const isLoading = summaryLoading || runsLoading;
  const latestRun = runsData?.data?.[0];
  const chart = summary?.chart ?? [];
  const repoName = project.repo_url
    ? project.repo_url
        .replace(/^https?:\/\/(www\.)?github\.com\//, "")
        .replace(/\.git$/, "")
    : "";

  // Stats computation from 7-day chart data
  const totalSuccess = chart.reduce((s, d) => s + d.success, 0);
  const totalFailed = chart.reduce((s, d) => s + d.failed, 0);
  const totalRuns = totalSuccess + totalFailed;
  const reliability =
    totalRuns > 0 ? Math.round((totalSuccess / totalRuns) * 100) : 0;

  const durationsWithData = chart
    .map((d) => d.duration_ms)
    .filter((d) => d > 0);
  const avgDuration =
    durationsWithData.length > 0
      ? durationsWithData.reduce((s, d) => s + d, 0) / durationsWithData.length
      : 0;

  const maxDaily = Math.max(...chart.map((d) => d.success + d.failed), 1);
  const primaryBranch = summary?.branches?.[0];

  if (isLoading) {
    return (
      <Card className="overflow-hidden gap-0 py-0">
        <CardContent className="p-4 space-y-3">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-3 w-32" />
          <div className="flex gap-1 items-end h-10">
            {Array.from({ length: 7 }).map((_, i) => (
              <Skeleton key={i} className="flex-1 h-6" />
            ))}
          </div>
          <div className="flex gap-4">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-16" />
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Link
      to="/projects/$projectId"
      params={{ projectId: project.id }}
      className="block"
    >
      <Card className="overflow-hidden transition-colors gap-0 py-0 hover:bg-accent/30">
        <CardContent className="p-4">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1 space-y-1">
              {/* Project name + visibility */}
              <div className="flex items-center gap-2">
                <h3 className="font-semibold truncate">{project.name}</h3>
                <Badge variant="outline" className="text-[10px] shrink-0">
                  {project.visibility}
                </Badge>
              </div>

              {/* Slug */}
              <p className="font-mono text-xs text-muted-foreground truncate">
                {project.slug}
              </p>

              <div className="flex gap-2 items-center">
                {/* Repo Name */}
                {repoName && (
                  <div className="flex items-center gap-1 text-muted-foreground">
                    <Icons.Github className="h-3 w-3" />
                    <span className="text-sm flex-1 text-left">{repoName}</span>
                  </div>
                )}

                {/* Branch */}
                {primaryBranch && (
                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Icons.GitBranch className="h-3 w-3" />
                    <span className="truncate">{primaryBranch}</span>
                  </div>
                )}
              </div>
            </div>

            {/* Latest run status icon */}
            {latestRun && (
              <div
                className={cn(
                  `shrink-0 flex items-center justify-center h-10 w-10 rounded-lg `,
                  statusBgColor(latestRun.status),
                )}
              >
                <RunStatusIcon status={latestRun.status} className="h-5 w-5" />
              </div>
            )}
          </div>

          {/* Mini bar chart - 7 days */}
          <div className="flex items-end gap-1 h-10 mt-3">
            {chart.map((day, i) => {
              const total = day.success + day.failed;
              const heightPct =
                total > 0 ? Math.max(8, (total / maxDaily) * 100) : 0;
              const successPct = total > 0 ? (day.success / total) * 100 : 100;

              if (total === 0) {
                return (
                  <div
                    key={i}
                    className="flex-1 rounded-none bg-muted"
                    style={{ height: "4px" }}
                  />
                );
              }

              return (
                <div
                  key={i}
                  className="flex-1 rounded-none overflow-hidden"
                  style={{ height: `${heightPct}%` }}
                >
                  <div
                    className="w-full bg-green-500"
                    style={{ height: `${successPct}%` }}
                  />
                  <div
                    className="w-full bg-red-500"
                    style={{ height: `${100 - successPct}%` }}
                  />
                </div>
              );
            })}
            {/* Pad to 7 if fewer data points */}
            {Array.from({ length: Math.max(0, 7 - chart.length) }).map(
              (_, i) => (
                <div
                  key={`empty-${i}`}
                  className="flex-1 rounded-none bg-muted"
                  style={{ height: "4px" }}
                />
              ),
            )}
          </div>

          {/* Stats row */}
          <div className="grid grid-cols-3 gap-2 mt-3 text-center">
            <div>
              <p className="text-xs text-muted-foreground">Speed</p>
              <p className="text-sm font-medium">
                {avgDuration > 0 ? formatDuration(avgDuration) : "--"}
              </p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Reliability</p>
              <p className="text-sm font-medium">
                {totalRuns > 0 ? `${reliability}%` : "--"}
              </p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Builds</p>
              <p className="text-sm font-medium">
                {totalRuns > 0 ? `${totalRuns}/wk` : "--"}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
