import { Link } from "@tanstack/react-router";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Card, CardContent } from "~/components/ui/card";
import { Icons } from "~/components/icons";
import type { Project, Run } from "~/lib/api";
import { RunCard } from "./run-card";
import { TriggerRunDialog } from "./trigger-run-dialog";

type ProjectBasicViewProps = {
  project: Project;
  projectId: string;
  filteredRuns: Run[];
  runsLoading: boolean;
  triggerDialogOpen: boolean;
  setTriggerDialogOpen: (open: boolean) => void;
  triggerTargets: string;
  setTriggerTargets: (targets: string) => void;
  triggerBranch: string;
  setTriggerBranch: (branch: string) => void;
  triggerForce: boolean;
  setTriggerForce: (force: boolean) => void;
  onTriggerRun: () => void;
  triggerRunPending: boolean;
  triggerRunErrorMessage?: string;
  page: number;
  setPage: (page: number) => void;
  totalPages: number;
  total: number;
  perPage: number;
};

export function ProjectBasicView({
  project,
  projectId,
  filteredRuns,
  runsLoading,
  triggerDialogOpen,
  setTriggerDialogOpen,
  triggerTargets,
  setTriggerTargets,
  triggerBranch,
  setTriggerBranch,
  triggerForce,
  setTriggerForce,
  onTriggerRun,
  triggerRunPending,
  triggerRunErrorMessage,
  page,
  setPage,
  totalPages,
  total,
  perPage,
}: ProjectBasicViewProps) {
  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/projects">
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">{project.name}</h1>
            <Badge
              variant={project.visibility === "public" ? "default" : "secondary"}
            >
              {project.visibility}
            </Badge>
          </div>
          <p className="text-muted-foreground font-mono">{project.slug}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="icon" asChild>
            <Link to="/projects/$projectId/plugins" params={{ projectId }}>
              <Icons.Package className="h-4 w-4" />
            </Link>
          </Button>
          <Button variant="outline" size="icon" asChild>
            <Link to="/projects/$projectId/cache" params={{ projectId }}>
              <Icons.Database className="h-4 w-4" />
            </Link>
          </Button>
          <Button variant="outline" size="icon" asChild>
            <Link to="/projects/$projectId/settings" params={{ projectId }}>
              <Icons.Settings className="h-4 w-4" />
            </Link>
          </Button>
        </div>
      </div>

      {project.description ? (
        <Card>
          <CardContent className="pt-6">
            <p className="text-muted-foreground">{project.description}</p>
          </CardContent>
        </Card>
      ) : null}

      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold">Recent Runs</h2>
          <TriggerRunDialog
            open={triggerDialogOpen}
            onOpenChange={setTriggerDialogOpen}
            triggerTargets={triggerTargets}
            setTriggerTargets={setTriggerTargets}
            triggerBranch={triggerBranch}
            setTriggerBranch={setTriggerBranch}
            triggerForce={triggerForce}
            setTriggerForce={setTriggerForce}
            onTriggerRun={onTriggerRun}
            isPending={triggerRunPending}
            errorMessage={triggerRunErrorMessage}
          />
        </div>

        {runsLoading ? (
          <div className="flex items-center justify-center py-12">
            <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : filteredRuns.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Icons.Play className="h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="text-lg font-semibold">No runs yet</h3>
              <p className="text-muted-foreground text-center mt-1 mb-4">
                Run <code className="bg-muted px-1 rounded">dagryn run</code> to start
                your first workflow
              </p>
            </CardContent>
          </Card>
        ) : (
          <>
            <div className="space-y-3">
              {filteredRuns.map((run) => (
                <RunCard
                  key={run.id}
                  run={run}
                  projectId={projectId}
                  repoLabel={
                    project.repo_url
                      ? project.repo_url
                          .replace(/^https?:\/\/(www\.)?github\.com\//, "")
                          .replace(/\.git$/, "")
                      : ""
                  }
                />
              ))}
            </div>
            {totalPages > 1 ? (
              <div className="flex items-center justify-between mt-6">
                <p className="text-sm text-muted-foreground">
                  Showing {(page - 1) * perPage + 1} - {Math.min(page * perPage, total)}
                  of {total} runs
                </p>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(Math.max(1, page - 1))}
                    disabled={page === 1 || runsLoading}
                  >
                    <Icons.ChevronLeft className="h-4 w-4 mr-1" />
                    Previous
                  </Button>
                  <div className="flex items-center gap-1 px-2">
                    <span className="text-sm font-medium">{page}</span>
                    <span className="text-sm text-muted-foreground">of</span>
                    <span className="text-sm font-medium">{totalPages}</span>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(Math.min(totalPages, page + 1))}
                    disabled={page === totalPages || runsLoading}
                  >
                    Next
                    <Icons.ChevronRight className="h-4 w-4 ml-1" />
                  </Button>
                </div>
              </div>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}
