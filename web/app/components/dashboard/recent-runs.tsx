import { Link } from "@tanstack/react-router";
import type { DashboardRun } from "~/lib/api";
import { RunStatusIcon } from "~/components/projects/run-detail/status-ui";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
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

export function RecentRunsSection({ runs }: { runs: DashboardRun[] }) {
  return (
    <Card className="py-0 gap-0">
      <CardHeader className="p-3 gap-0">
        <CardTitle className="text-sm font-medium">Recent Runs</CardTitle>
      </CardHeader>
      <Separator />
      {runs.length === 0 ? (
        <CardContent>
          <p className="text-sm text-muted-foreground">No runs yet</p>
        </CardContent>
      ) : (
        <CardContent className="space-y-0 px-0">
          {runs.map((run, index) => (
            <RunCard
              key={run.id}
              run={run}
              index={index}
              isLast={index === runs.length - 1}
            />
          ))}
        </CardContent>
      )}
    </Card>
  );
}

const RunCard = ({
  run,
  isLast,
}: {
  run: DashboardRun;
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
  }, [run.triggered_by_user, run.commit_author_name]);

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
              {run.project_name}{" "}
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
