import { useMemo } from "react";
import { useRunAIAnalysis } from "~/hooks/queries";
import { useRetryAIAnalysis, usePostAISuggestions } from "~/hooks/mutations";
import type {
  AIAnalysis,
  AIPublication,
  AISuggestion,
  AIAnalysisStatus,
} from "~/lib/api";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Progress } from "~/components/ui/progress";
import { Icons } from "~/components/icons";

interface AIAnalysisTabProps {
  projectId: string;
  runId: string;
  runStatus: string;
}

export function AIAnalysisTab({
  projectId,
  runId,
  runStatus,
}: AIAnalysisTabProps) {
  const {
    data,
    isLoading,
    error,
  } = useRunAIAnalysis(projectId, runId);

  const retryMutation = useRetryAIAnalysis();
  const postSuggestionsMutation = usePostAISuggestions();

  if (isLoading) {
    return (
      <Card>
        <CardContent className="py-12">
          <div className="flex items-center justify-center text-muted-foreground">
            <Icons.Loader className="h-5 w-5 animate-spin mr-2" />
            Loading AI analysis...
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error || !data) {
    const is404 =
      error && "status" in error && (error as { status: number }).status === 404;
    return (
      <Card>
        <CardContent className="py-12">
          <div className="flex flex-col items-center justify-center gap-4 text-muted-foreground">
            <Icons.Lightbulb className="h-10 w-10" />
            <p>{is404 ? "No AI analysis available for this run." : "Failed to load AI analysis."}</p>
            {runStatus === "failed" && (
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  retryMutation.mutate({ projectId, runId })
                }
                disabled={retryMutation.isPending}
              >
                {retryMutation.isPending ? (
                  <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Icons.Retry className="mr-2 h-4 w-4" />
                )}
                Request AI Analysis
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    );
  }

  const { analysis, publications, suggestions } = data;

  return (
    <div className="space-y-4">
      <AnalysisSummaryCard
        analysis={analysis}
        onRetry={() => retryMutation.mutate({ projectId, runId })}
        retryPending={retryMutation.isPending}
      />
      {analysis.status === "success" && (
        <>
          <EvidenceCard evidenceJson={analysis.evidence_json} />
          <SuggestionsCard
            suggestions={suggestions}
            projectId={projectId}
            runId={runId}
            onPost={() =>
              postSuggestionsMutation.mutate({ projectId, runId })
            }
            postPending={postSuggestionsMutation.isPending}
          />
          <PublicationsCard publications={publications} />
        </>
      )}
    </div>
  );
}

function AnalysisStatusBadge({ status }: { status: AIAnalysisStatus }) {
  const variants: Record<
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
  const config = variants[status] ?? { variant: "secondary" as const, label: status };
  return <Badge variant={config.variant}>{config.label}</Badge>;
}

function ConfidenceDisplay({ confidence }: { confidence: number }) {
  const pct = Math.round(confidence * 100);
  const color =
    pct >= 70
      ? "text-green-500"
      : pct >= 40
        ? "text-yellow-500"
        : "text-red-500";

  return (
    <div className="flex items-center gap-3">
      <span className={`text-2xl font-bold ${color}`}>{pct}%</span>
      <Progress
        value={pct}
        className="h-2 flex-1"
      />
    </div>
  );
}

function AnalysisSummaryCard({
  analysis,
  onRetry,
  retryPending,
}: {
  analysis: AIAnalysis;
  onRetry: () => void;
  retryPending: boolean;
}) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Icons.Lightbulb className="h-5 w-5" />
            <CardTitle>AI Failure Analysis</CardTitle>
            <AnalysisStatusBadge status={analysis.status} />
          </div>
          {(analysis.status === "failed" || analysis.status === "success") && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onRetry}
              disabled={retryPending}
            >
              {retryPending ? (
                <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Icons.Retry className="mr-2 h-4 w-4" />
              )}
              Retry
            </Button>
          )}
        </div>
        {analysis.model && (
          <CardDescription>
            Model: {analysis.model}
            {analysis.provider ? ` (${analysis.provider})` : ""}
          </CardDescription>
        )}
      </CardHeader>
      <CardContent className="space-y-4">
        {analysis.status === "in_progress" && (
          <div className="flex items-center gap-2 text-muted-foreground">
            <Icons.Loader className="h-4 w-4 animate-spin" />
            <span>Analysis in progress...</span>
          </div>
        )}

        {analysis.status === "failed" && analysis.error_message && (
          <div className="rounded-none bg-destructive/10 p-3 text-sm text-destructive">
            {analysis.error_message}
          </div>
        )}

        {analysis.confidence != null && (
          <div>
            <p className="text-sm font-medium mb-2">Confidence</p>
            <ConfidenceDisplay confidence={analysis.confidence} />
          </div>
        )}

        {analysis.summary && (
          <div>
            <p className="text-sm font-medium mb-1">Summary</p>
            <p className="text-sm text-muted-foreground">{analysis.summary}</p>
          </div>
        )}

        {analysis.root_cause && (
          <div>
            <p className="text-sm font-medium mb-1">Root Cause</p>
            <p className="text-sm text-muted-foreground">{analysis.root_cause}</p>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

interface EvidenceItem {
  task: string;
  reason: string;
  log_excerpt?: string;
}

function EvidenceCard({ evidenceJson }: { evidenceJson?: string }) {
  const evidence = useMemo<EvidenceItem[]>(() => {
    if (!evidenceJson) return [];
    try {
      const parsed = JSON.parse(evidenceJson);
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }, [evidenceJson]);

  if (evidence.length === 0) return null;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Icons.Target className="h-4 w-4" />
          <CardTitle className="text-base">Evidence</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {evidence.map((item, i) => (
            <div key={i} className="rounded-none border p-3">
              <div className="flex items-center gap-2 mb-1">
                <Badge variant="secondary">{item.task}</Badge>
              </div>
              <p className="text-sm text-muted-foreground">{item.reason}</p>
              {item.log_excerpt && (
                <pre className="mt-2 bg-muted/50 p-2 text-xs font-mono overflow-x-auto rounded-none">
                  {item.log_excerpt}
                </pre>
              )}
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function SuggestionsCard({
  suggestions,
  onPost,
  postPending,
}: {
  suggestions: AISuggestion[] | null;
  projectId: string;
  runId: string;
  onPost: () => void;
  postPending: boolean;
}) {
  if (!suggestions || suggestions.length === 0) return null;

  const pendingCount = suggestions.filter(
    (s) => s.status === "pending",
  ).length;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Icons.FileCode className="h-4 w-4" />
            <CardTitle className="text-base">
              Suggestions ({suggestions.length})
            </CardTitle>
          </div>
          {pendingCount > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={onPost}
              disabled={postPending}
            >
              {postPending ? (
                <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Icons.Send className="mr-2 h-4 w-4" />
              )}
              Post to PR ({pendingCount})
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {suggestions.map((s) => (
            <SuggestionItem key={s.id} suggestion={s} />
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function SuggestionStatusBadge({ status }: { status: AISuggestion["status"] }) {
  const config: Record<
    string,
    { variant: "default" | "secondary" | "destructive" | "outline"; label: string }
  > = {
    pending: { variant: "secondary", label: "Pending" },
    posted: { variant: "default", label: "Posted" },
    accepted: { variant: "default", label: "Accepted" },
    dismissed: { variant: "outline", label: "Dismissed" },
    failed_validation: { variant: "destructive", label: "Failed" },
  };
  const c = config[status] ?? { variant: "secondary" as const, label: status };
  return (
    <Badge variant={c.variant} className="text-xs">
      {c.label}
    </Badge>
  );
}

function SuggestionItem({ suggestion: s }: { suggestion: AISuggestion }) {
  const confidencePct = Math.round(s.confidence * 100);

  return (
    <div className="rounded-none border p-3 space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <code className="text-sm font-mono">
            {s.file_path}:{s.start_line}
            {s.end_line !== s.start_line ? `-${s.end_line}` : ""}
          </code>
          <SuggestionStatusBadge status={s.status} />
        </div>
        <span className="text-xs text-muted-foreground">
          {confidencePct}% confidence
        </span>
      </div>
      <p className="text-sm text-muted-foreground">{s.explanation}</p>
      {s.suggested_code && (
        <pre className="bg-muted/50 p-2 text-xs font-mono overflow-x-auto rounded-none">
          {s.suggested_code}
        </pre>
      )}
      {s.failure_reason && (
        <p className="text-xs text-destructive">{s.failure_reason}</p>
      )}
    </div>
  );
}

function PublicationsCard({
  publications,
}: {
  publications: AIPublication[] | null;
}) {
  if (!publications || publications.length === 0) return null;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Icons.Send className="h-4 w-4" />
          <CardTitle className="text-base">
            Publications ({publications.length})
          </CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {publications.map((p) => (
            <div
              key={p.id}
              className="flex items-center justify-between p-2 rounded-none border"
            >
              <div className="flex items-center gap-2">
                <PublicationDestBadge dest={p.destination} />
                <PublicationStatusBadge status={p.status} />
              </div>
              <span className="text-xs text-muted-foreground">
                {new Date(p.created_at).toLocaleString()}
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function PublicationDestBadge({
  dest,
}: {
  dest: AIPublication["destination"];
}) {
  const labels: Record<string, string> = {
    github_pr_comment: "PR Comment",
    github_check: "Check Run",
    github_pr_review: "PR Review",
  };
  return (
    <Badge variant="outline" className="text-xs">
      {labels[dest] ?? dest}
    </Badge>
  );
}

function PublicationStatusBadge({
  status,
}: {
  status: AIPublication["status"];
}) {
  const config: Record<
    string,
    { variant: "default" | "secondary" | "destructive" | "outline"; label: string }
  > = {
    pending: { variant: "secondary", label: "Pending" },
    sent: { variant: "default", label: "Sent" },
    updated: { variant: "default", label: "Updated" },
    failed: { variant: "destructive", label: "Failed" },
  };
  const c = config[status] ?? { variant: "secondary" as const, label: status };
  return (
    <Badge variant={c.variant} className="text-xs">
      {c.label}
    </Badge>
  );
}
