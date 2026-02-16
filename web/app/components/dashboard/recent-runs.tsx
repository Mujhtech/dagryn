import { Link } from "@tanstack/react-router";
import { useQueries } from "@tanstack/react-query";
import type { Project, Run } from "~/lib/api";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";
import { RunStatusIcon } from "~/components/projects/run-detail/status-ui";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";
import { cn } from "~/lib/utils";
import { Separator } from "../ui/separator";
import { Badge } from "../ui/badge";
import { useMemo } from "react";
import { Avatar, AvatarFallback, AvatarImage } from "../ui/avatar";

function timeAgo(isoDate: string): string {
  const diffMs = Date.now() - new Date(isoDate).getTime();
  const mins = Math.floor(diffMs / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function RecentRunsSection({ projects }: { projects: Project[] }) {
  const topProjects = [...projects]
    .sort(
      (a, b) =>
        new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime(),
    )
    .slice(0, 8);

  const runQueries = useQueries({
    queries: topProjects.map((p) => ({
      queryKey: queryKeys.runs(p.id, 1),
      queryFn: async () => {
        const { data } = await api.listRuns(p.id, 1, 3);
        return data.data.map((run: Run) => ({
          ...run,
          _projectName: p.name,
        }));
      },
      enabled: !!p.id,
      staleTime: 1000 * 60 * 2,
    })),
  });

  const isLoading = runQueries.some((q) => q.isLoading);

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-medium">Recent Runs</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="flex items-center gap-3">
              <Skeleton className="h-5 w-5 rounded-full" />
              <div className="flex-1 space-y-1">
                <Skeleton className="h-3 w-32" />
                <Skeleton className="h-3 w-20" />
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    );
  }

  const allRuns = runQueries
    .flatMap((q) => q.data ?? [])
    .sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    )
    .slice(0, 5);

  if (allRuns.length === 0) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-medium">Recent Runs</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">No runs yet</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="py-0 gap-0">
      <CardHeader className="p-3 gap-0">
        <CardTitle className="text-sm font-medium">Recent Runs</CardTitle>
      </CardHeader>
      <Separator />
      <CardContent className="space-y-0 px-0">
        {allRuns.map((run, index) => (
          <RunCard
            key={run.id}
            run={run}
            index={index}
            isLast={index === allRuns.length - 1}
          />
        ))}
      </CardContent>
    </Card>
  );
}

const RunCard = ({
  run,
  isLast,
}: {
  run: Run;
  index: number;
  isLast: boolean;
}) => {
  const triggerBy = useMemo(() => {
    if (run.triggered_by_user) {
      return {
        name: run.triggered_by_user.name,
        avatar: run.triggered_by_user.avatar_url,
      };
    }

    return {
      name: run.commit_author_name || "Unknown",
      avatar: null,
    };
  }, [run.triggered_by_user, run.commit_author_name, run.commit_author_name]);

  return (
    <div
      className={cn(
        "border-b border-border last:border-b-0",
        isLast && "border-b-0",
      )}
    >
      <Link
        key={run.id}
        to="/projects/$projectId/runs/$runId"
        params={{ projectId: run.project_id, runId: run.id }}
        className="flex items-center gap-3 rounded-none p-3 transition-colors hover:bg-accent/50"
      >
        <div className="min-w-0 flex-1 flex-col gap-0.5 flex">
          <div className="flex gap-1 items-center">
            <RunStatusIcon status={run.status} className="h-3 w-3 shrink-0" />
            <p className="text-sm font-medium truncate">
              {(run as Run & { _projectName: string })._projectName}{" "}
              <span className="text-muted-foreground">{run.workflow_name}</span>
            </p>
          </div>
          <div className="flex gap-1">
            {triggerBy && (
              <div className="flex items-center gap-0.5">
                <Avatar className="h-3 w-3">
                  {triggerBy.avatar && <AvatarImage src={triggerBy.avatar} />}
                  <AvatarFallback className="text-[10px]">U</AvatarFallback>
                </Avatar>
                <span className="text-xs text-muted-foreground truncate">
                  {triggerBy.name}
                </span>
              </div>
            )}
            {run.trigger_ref && (
              <Badge
                variant="outline"
                className="text-xs px-1 text-muted-foreground truncate"
              >
                {run.trigger_ref || "no branch"}
              </Badge>
            )}
          </div>
        </div>
        <span className="text-xs text-muted-foreground shrink-0">
          {timeAgo(run.created_at)}
        </span>
      </Link>
    </div>
  );
};
