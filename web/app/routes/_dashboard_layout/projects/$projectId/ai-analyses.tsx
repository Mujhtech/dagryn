import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { useProject, useProjectAIAnalyses } from "~/hooks/queries";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Progress } from "~/components/ui/progress";
import { Icons } from "~/components/icons";
import type { AIAnalysis, AIAnalysisStatus } from "~/lib/api";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/ai-analyses",
)({
  component: AIAnalysesPage,
  head: () => {
    return generateMetadata({ title: "AI Analyses" });
  },
});

function AIAnalysesPage() {
  const { projectId } = Route.useParams();
  const { data: project, isLoading: projectLoading } = useProject(projectId);
  const [offset, setOffset] = useState(0);
  const limit = 20;
  const {
    data: analysesData,
    isLoading: analysesLoading,
  } = useProjectAIAnalyses(projectId, limit, offset);

  if (projectLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  if (!project) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Project not found</CardTitle>
        </CardHeader>
      </Card>
    );
  }

  const analyses = analysesData?.analyses ?? [];
  const total = analysesData?.total ?? 0;
  const totalPages = Math.ceil(total / limit);
  const currentPage = Math.floor(offset / limit) + 1;

  const successCount = analyses.filter((a) => a.status === "success").length;
  const failedCount = analyses.filter((a) => a.status === "failed").length;
  const avgConfidence =
    analyses.filter((a) => a.confidence != null).length > 0
      ? analyses
          .filter((a) => a.confidence != null)
          .reduce((sum, a) => sum + (a.confidence ?? 0), 0) /
        analyses.filter((a) => a.confidence != null).length
      : 0;

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/projects/$projectId" params={{ projectId }}>
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">AI Analyses</h1>
            <Badge variant="secondary">{project.name}</Badge>
          </div>
          <p className="text-muted-foreground">
            AI-powered failure analysis for your CI runs
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Total</CardTitle>
            <Icons.Lightbulb className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{total}</div>
            <p className="text-xs text-muted-foreground">analyses</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Successful</CardTitle>
            <Icons.CheckCircle className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-500">{successCount}</div>
            <p className="text-xs text-muted-foreground">on this page</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Failed</CardTitle>
            <Icons.XCircle className="h-4 w-4 text-destructive" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-destructive">{failedCount}</div>
            <p className="text-xs text-muted-foreground">on this page</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Avg Confidence</CardTitle>
            <Icons.Target className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {avgConfidence > 0 ? `${Math.round(avgConfidence * 100)}%` : "--"}
            </div>
            <p className="text-xs text-muted-foreground">across successes</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Analysis History</CardTitle>
        </CardHeader>
        <CardContent>
          {analysesLoading ? (
            <div className="flex items-center justify-center h-32 text-muted-foreground">
              <Icons.Loader className="h-5 w-5 animate-spin mr-2" />
              Loading analyses...
            </div>
          ) : analyses.length === 0 ? (
            <div className="flex items-center justify-center h-32 text-muted-foreground">
              No AI analyses yet. Analyses are automatically triggered when runs fail.
            </div>
          ) : (
            <div className="space-y-2">
              {analyses.map((analysis) => (
                <AnalysisRow
                  key={analysis.id}
                  analysis={analysis}
                  projectId={projectId}
                />
              ))}
            </div>
          )}

          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-4 pt-4 border-t">
              <span className="text-sm text-muted-foreground">
                Page {currentPage} of {totalPages} ({total} total)
              </span>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - limit))}
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={offset + limit >= total}
                  onClick={() => setOffset(offset + limit)}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function AnalysisStatusBadge({ status }: { status: AIAnalysisStatus }) {
  const config: Record<
    string,
    { variant: "default" | "secondary" | "destructive" | "outline"; label: string }
  > = {
    pending: { variant: "secondary", label: "Pending" },
    in_progress: { variant: "default", label: "Analyzing" },
    success: { variant: "default", label: "Complete" },
    failed: { variant: "destructive", label: "Failed" },
    quota_exceeded: { variant: "destructive", label: "Quota Exceeded" },
    superseded: { variant: "outline", label: "Superseded" },
  };
  const c = config[status] ?? { variant: "secondary" as const, label: status };
  return <Badge variant={c.variant}>{c.label}</Badge>;
}

function AnalysisRow({
  analysis,
  projectId,
}: {
  analysis: AIAnalysis;
  projectId: string;
}) {
  const confidencePct = analysis.confidence != null
    ? Math.round(analysis.confidence * 100)
    : null;

  return (
    <Link
      to="/projects/$projectId/runs/$runId"
      params={{ projectId, runId: analysis.run_id }}
      className="block"
    >
      <div className="flex items-center justify-between p-3 rounded-none border hover:bg-muted/50 transition-colors">
        <div className="flex items-center gap-3 min-w-0">
          <AnalysisStatusBadge status={analysis.status} />
          <div className="min-w-0">
            <p className="font-medium truncate">
              Run{" "}
              <span className="font-mono text-xs">{analysis.run_id.slice(0, 8)}</span>
            </p>
            {analysis.summary && (
              <p className="text-sm text-muted-foreground truncate max-w-md">
                {analysis.summary}
              </p>
            )}
          </div>
        </div>
        <div className="flex items-center gap-4 shrink-0">
          {confidencePct != null && (
            <div className="flex items-center gap-2 w-32">
              <Progress value={confidencePct} className="h-1.5 flex-1" />
              <span className="text-xs text-muted-foreground w-8 text-right">
                {confidencePct}%
              </span>
            </div>
          )}
          <span className="text-xs text-muted-foreground">
            {new Date(analysis.created_at).toLocaleDateString()}
          </span>
          <Icons.ArrowRight className="h-4 w-4 text-muted-foreground" />
        </div>
      </div>
    </Link>
  );
}
