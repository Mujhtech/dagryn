import { Link } from "@tanstack/react-router";
import { Avatar, AvatarFallback, AvatarImage } from "~/components/ui/avatar";
import { Card, CardContent } from "~/components/ui/card";
import { Progress } from "~/components/ui/progress";
import { Icons } from "~/components/icons";
import type { Run } from "~/lib/api";

type CurrentUser = {
  id: string;
  name: string;
  email: string;
  avatar_url?: string;
} | null;

type RunCardProps = {
  run: Run;
  projectId: string;
  repoLabel: string;
  currentUser?: CurrentUser;
};

export function RunCard({
  run,
  projectId,
  repoLabel,
  currentUser,
}: RunCardProps) {
  const triggerInfo = getTriggerInfo(run, currentUser);
  const eventType = getEventType(run);
  const displayName = run.pr_title || run.workflow_name;
  const description = run.commit_message || run.pr_title || "";
  const branch = run.trigger_ref?.replace("refs/heads/", "") || "";

  const progress =
    run.status === "success"
      ? 100
      : run.status === "failed"
        ? 100
        : run.status === "running"
          ? 50
          : 0;

  return (
    <Link
      to="/projects/$projectId/runs/$runId"
      params={{ projectId, runId: run.id }}
      className="block"
    >
      <Card className="hover:border-primary/50 transition-colors cursor-pointer">
        <CardContent className="p-4">
          <div className="flex items-start gap-4">
            <div className="pt-1">
              <RunStatusIcon status={run.status} />
            </div>
            <div className="flex-1 min-w-0">
              <h3 className="font-semibold text-base mb-2">{displayName}</h3>
              <div className="flex items-center gap-2 mb-2">
                <Avatar className="h-5 w-5">
                  <AvatarImage src={triggerInfo.avatar} />
                  <AvatarFallback className="text-xs">
                    {triggerInfo.name[0]?.toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <span className="text-sm text-muted-foreground">
                  {triggerInfo.name}
                </span>
                <EventIcon eventType={eventType} />
                <span className="text-sm text-muted-foreground">
                  {eventType}
                </span>
                {run.pr_number ? (
                  <span className="text-sm text-muted-foreground">
                    #{run.pr_number}
                  </span>
                ) : null}
                <span className="text-sm text-muted-foreground">·</span>
                <span className="text-sm text-muted-foreground">
                  {formatTimeAgo(run.created_at)}
                </span>
              </div>

              {description ? (
                <p className="text-sm text-muted-foreground mb-3 line-clamp-2">
                  {description}
                </p>
              ) : null}

              <div className="flex items-center gap-4 text-sm text-muted-foreground mb-3">
                <div className="flex items-center gap-1">
                  <Icons.Github className="h-4 w-4" />
                  <span>{repoLabel}</span>
                </div>
                {branch ? (
                  <div className="flex items-center gap-1">
                    <Icons.GitBranch className="h-4 w-4" />
                    <span>{branch}</span>
                  </div>
                ) : null}
              </div>

              <div className="flex items-center gap-4">
                {run.duration_ms != null ? (
                  <div className="flex items-center gap-1 text-sm text-muted-foreground">
                    <Icons.Clock className="h-4 w-4" />
                    <span>{formatDuration(run.duration_ms)}</span>
                  </div>
                ) : null}
                {run.task_count > 0 ? (
                  <div className="flex-1">
                    <Progress value={progress} className="h-1.5" />
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}

export function getEventType(run: Run): string {
  if (run.pr_number) return "pull_request";
  if (run.trigger_source === "cli") return "push";
  if (run.trigger_source === "ci") return "push";
  if (run.trigger_source === "api" || run.trigger_source === "dashboard") {
    return "workflow_dispatch";
  }
  return run.trigger_source || "push";
}

function getTriggerInfo(
  run: Run,
  currentUser?: CurrentUser,
): { name: string; avatar?: string } {
  if (run.commit_author_name) {
    return {
      name: run.commit_author_name,
      avatar: undefined,
    };
  }

  if (run.triggered_by_user) {
    return {
      name: run.triggered_by_user.name,
      avatar: run.triggered_by_user.avatar_url,
    };
  }

  if (currentUser) {
    return {
      name: currentUser.name,
      avatar: currentUser.avatar_url,
    };
  }

  return { name: "Unknown" };
}

function EventIcon({ eventType }: { eventType: string }) {
  switch (eventType) {
    case "pull_request":
      return <Icons.GitPullRequest className="h-4 w-4 text-muted-foreground" />;
    case "schedule":
      return <Icons.Calendar className="h-4 w-4 text-muted-foreground" />;
    case "workflow_dispatch":
      return <Icons.Play className="h-4 w-4 text-muted-foreground" />;
    case "push":
    default:
      return <Icons.GitCommit className="h-4 w-4 text-muted-foreground" />;
  }
}

function RunStatusIcon({ status }: { status: string }) {
  switch (status) {
    case "success":
      return <Icons.CheckCircle className="h-5 w-5 text-green-500" />;
    case "failed":
      return <Icons.XCircle className="h-5 w-5 text-red-500" />;
    case "running":
      return <Icons.Loader className="h-5 w-5 text-blue-500 animate-spin" />;
    case "pending":
      return <Icons.Circle className="h-5 w-5 text-yellow-500" />;
    case "cancelled":
      return <Icons.XCircle className="h-5 w-5 text-gray-500" />;
    default:
      return <Icons.Circle className="h-5 w-5 text-gray-400" />;
  }
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000);
  return `${minutes}m ${seconds}s`;
}

function formatTimeAgo(iso: string): string {
  const date = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}
