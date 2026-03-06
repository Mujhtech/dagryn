import { Link } from "@tanstack/react-router";
import type { DashboardProject } from "~/lib/api";
import {
  RunStatusIcon,
  formatDuration,
} from "~/components/projects/run-detail/status-ui";
import { Card, CardContent } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
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

export function ProjectStatsCard({ project }: { project: DashboardProject }) {
  const chart = project.chart ?? [];
  const latestRun = project.latest_run;
  const repoName = project.repo_url
    ? project.repo_url
        .replace(/^https?:\/\/(www\.)?github\.com\//, "")
        .replace(/\.git$/, "")
    : "";

  // Stats from inline data
  const totalSuccess = project.success_runs_7d;
  const totalRuns = project.total_runs_7d;
  const reliability =
    totalRuns > 0 ? Math.round((totalSuccess / totalRuns) * 100) : 0;
  const avgDuration = project.avg_duration_ms;
  const primaryBranch = project.top_branch;

  const maxDaily = Math.max(...chart.map((d) => d.success + d.failed), 1);

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
