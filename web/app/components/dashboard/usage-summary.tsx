import { Link } from "@tanstack/react-router";
import { useBillingOverview } from "~/hooks/queries";
import { formatBytes } from "~/components/projects/run-detail/status-ui";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Progress } from "~/components/ui/progress";
import { Skeleton } from "~/components/ui/skeleton";
import { Button } from "~/components/ui/button";
import { Icons } from "~/components/icons";

export function UsageSummaryCard() {
  const { data: overview, isLoading } = useBillingOverview();

  if (isLoading) {
    return (
      <Card className="gap-3">
        <CardHeader className="pb-3 px-3">
          <Skeleton className="h-5 w-32" />
        </CardHeader>
        <CardContent className="space-y-4 px-3">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-2 w-full" />
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-2 w-full" />
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-2 w-full" />
        </CardContent>
      </Card>
    );
  }

  const plan = overview?.plan;
  const subscription = overview?.subscription;
  const usage = overview?.resource_usage;

  const planName = plan?.display_name ?? plan?.name ?? "Free";

  const daysLeft = subscription?.current_period_end
    ? Math.max(
        0,
        Math.ceil(
          (new Date(subscription.current_period_end).getTime() - Date.now()) /
            (1000 * 60 * 60 * 24),
        ),
      )
    : null;

  const metrics: {
    label: string;
    used: number;
    max: number | undefined;
    format: (v: number) => string;
  }[] = [
    {
      label: "Storage",
      used: usage?.total_storage_bytes_used ?? 0,
      max: plan?.max_storage_bytes ?? undefined,
      format: formatBytes,
    },
    {
      label: "Projects",
      used: usage?.projects_used ?? 0,
      max: plan?.max_projects ?? undefined,
      format: (v) => String(v),
    },
    {
      label: "Team Members",
      used: usage?.team_members_used ?? 0,
      max: plan?.max_team_members ?? undefined,
      format: (v) => String(v),
    },
    {
      label: "AI Analyses",
      used: usage?.ai_analyses_used ?? 0,
      max: plan?.max_ai_analyses_per_month ?? undefined,
      format: (v) => String(v),
    },
  ];

  return (
    <Card className="gap-3">
      <CardHeader className="pb-3 px-3 gap-0">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-medium">Usage</CardTitle>
          <Button variant="ghost" size="sm" asChild className="h-7 text-xs">
            <Link to="/billing">
              <Icons.CreditCard className="mr-1 h-3 w-3" />
              Billing
            </Link>
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-3 px-3">
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">Plan</span>
          <span className="font-medium">{planName}</span>
        </div>
        {daysLeft !== null && (
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">Cycle</span>
            <span className="text-muted-foreground">
              {daysLeft} day{daysLeft !== 1 ? "s" : ""} left
            </span>
          </div>
        )}

        <div className="space-y-3 pt-0">
          {metrics.map((m) => {
            const pct =
              m.max && m.max > 0
                ? Math.min(100, Math.round((m.used / m.max) * 100))
                : 0;
            return (
              <div key={m.label} className="space-y-1">
                <div className="flex items-center justify-between text-xs">
                  <span className="text-muted-foreground">{m.label}</span>
                  <span className="font-mono text-muted-foreground">
                    {m.format(m.used)}
                    {m.max ? ` / ${m.format(m.max)}` : ""}
                  </span>
                </div>
                {m.max ? (
                  <Progress value={pct} className="h-1.5" />
                ) : (
                  <Progress value={0} className="h-1.5" />
                )}
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}
