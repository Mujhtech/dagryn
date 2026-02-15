import { createFileRoute } from "@tanstack/react-router";
import {
  useBillingOverview,
  useBillingPlans,
  useBillingInvoices,
} from "~/hooks/queries";
import {
  useCreateCheckoutSession,
  useCreatePortalSession,
  useCancelSubscription,
} from "~/hooks/mutations";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Progress } from "~/components/ui/progress";
import { Separator } from "~/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "~/components/ui/alert-dialog";
import { Icons } from "~/components/icons";
import type {
  BillingPlan,
  BillingOverview as BillingOverviewType,
} from "~/lib/api";

export const Route = createFileRoute("/_dashboard_layout/billing")({
  component: BillingPage,
});

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

function formatCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function BillingPage() {
  const { data: overview, isLoading: overviewLoading } = useBillingOverview();
  const { data: plans, isLoading: plansLoading } = useBillingPlans();
  const { data: invoices, isLoading: invoicesLoading } = useBillingInvoices();

  const checkoutMutation = useCreateCheckoutSession();
  const portalMutation = useCreatePortalSession();
  const cancelMutation = useCancelSubscription();

  const loading = overviewLoading || plansLoading;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const currentPlanSlug = overview?.plan?.slug || "free";

  return (
    <div className="container px-6 py-8 space-y-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Billing</h1>
        <p className="text-muted-foreground mt-1">
          Manage your subscription, usage, and invoices.
        </p>
      </div>

      <Tabs defaultValue="overview" className="space-y-6">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="plans">Plans</TabsTrigger>
          <TabsTrigger value="invoices">Invoices</TabsTrigger>
        </TabsList>

        {/* Overview Tab */}
        <TabsContent value="overview" className="space-y-6">
          {/* Current Plan Card */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Current Plan</CardTitle>
                  <CardDescription>
                    Your active subscription details
                  </CardDescription>
                </div>
                <Badge
                  variant={
                    overview?.subscription?.status === "active"
                      ? "default"
                      : "secondary"
                  }
                  className="text-sm"
                >
                  {overview?.subscription?.status || "free"}
                </Badge>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center gap-4">
                <div className="flex-1">
                  <h3 className="text-2xl font-bold">
                    {overview?.plan?.display_name || "Free"}
                  </h3>
                  <p className="text-muted-foreground">
                    {overview?.plan?.description ||
                      "Basic features for individuals"}
                  </p>
                </div>
                <div className="text-right">
                  <p className="text-2xl font-bold">
                    {overview?.plan
                      ? formatCents(overview.plan.price_cents)
                      : "$0.00"}
                  </p>
                  <p className="text-sm text-muted-foreground">
                    /{overview?.plan?.billing_period || "month"}
                  </p>
                </div>
              </div>

              {overview?.subscription?.current_period_end && (
                <p className="text-sm text-muted-foreground">
                  {overview.subscription.cancel_at_period_end
                    ? "Cancels on "
                    : "Renews on "}
                  {formatDate(overview.subscription.current_period_end)}
                </p>
              )}

              {overview?.subscription?.cancel_at_period_end && (
                <div className="rounded-md bg-yellow-50 dark:bg-yellow-950 border border-yellow-200 dark:border-yellow-800 p-3">
                  <p className="text-sm text-yellow-800 dark:text-yellow-200">
                    Your subscription is set to cancel at the end of the current
                    billing period.
                  </p>
                </div>
              )}
            </CardContent>
            <CardFooter className="flex gap-2">
              {overview?.account?.stripe_customer_id && (
                <Button
                  variant="outline"
                  onClick={() => portalMutation.mutate(window.location.href)}
                  disabled={portalMutation.isPending}
                >
                  {portalMutation.isPending ? (
                    <Icons.Loader className="h-4 w-4 animate-spin mr-2" />
                  ) : null}
                  Manage Subscription
                </Button>
              )}
              {currentPlanSlug !== "free" &&
                !overview?.subscription?.cancel_at_period_end && (
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button variant="ghost" className="text-destructive">
                        Cancel Subscription
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>
                          Cancel Subscription?
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                          Your subscription will remain active until the end of
                          the current billing period. You can resubscribe at any
                          time.
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>Keep Subscription</AlertDialogCancel>
                        <AlertDialogAction
                          onClick={() => cancelMutation.mutate(true)}
                          className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                        >
                          Cancel Subscription
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                )}
            </CardFooter>
          </Card>

          {/* Usage Overview */}
          <UsageOverview overview={overview} />
        </TabsContent>

        {/* Plans Tab */}
        <TabsContent value="plans" className="space-y-6">
          <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
            {plans?.map((plan) => (
              <PlanCard
                key={plan.id}
                plan={plan}
                isCurrentPlan={plan.slug === currentPlanSlug}
                onUpgrade={() => {
                  checkoutMutation.mutate({
                    plan_slug: plan.slug,
                    success_url: `${window.location.origin}/billing?upgraded=true`,
                    cancel_url: window.location.href,
                  });
                }}
                isLoading={checkoutMutation.isPending}
              />
            ))}
          </div>
        </TabsContent>

        {/* Invoices Tab */}
        <TabsContent value="invoices" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Invoice History</CardTitle>
              <CardDescription>
                Your past invoices and payment history
              </CardDescription>
            </CardHeader>
            <CardContent>
              {invoicesLoading ? (
                <div className="flex items-center justify-center py-8">
                  <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
              ) : !invoices || invoices.length === 0 ? (
                <p className="text-center text-muted-foreground py-8">
                  No invoices yet.
                </p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Date</TableHead>
                      <TableHead>Period</TableHead>
                      <TableHead>Amount</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {invoices.map((invoice) => (
                      <TableRow key={invoice.id}>
                        <TableCell>{formatDate(invoice.created_at)}</TableCell>
                        <TableCell>
                          {invoice.period_start && invoice.period_end
                            ? `${formatDate(invoice.period_start)} - ${formatDate(invoice.period_end)}`
                            : "-"}
                        </TableCell>
                        <TableCell>
                          {formatCents(invoice.amount_cents)}{" "}
                          {invoice.currency.toUpperCase()}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              invoice.status === "paid"
                                ? "default"
                                : "secondary"
                            }
                          >
                            {invoice.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          {invoice.hosted_invoice_url && (
                            <Button variant="ghost" size="sm" asChild>
                              <a
                                href={invoice.hosted_invoice_url}
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                View
                              </a>
                            </Button>
                          )}
                          {invoice.pdf_url && (
                            <Button variant="ghost" size="sm" asChild>
                              <a
                                href={invoice.pdf_url}
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                PDF
                              </a>
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function UsageOverview({
  overview,
}: {
  overview: BillingOverviewType | undefined;
}) {
  if (!overview?.resource_usage) return null;

  const plan = overview.plan;
  const res = overview.resource_usage;

  const usageMetrics: {
    label: string;
    current: number;
    limit: number | null | undefined;
    format: (n: number) => string;
    subtitle?: string;
  }[] = [
    {
      label: "Storage",
      current: res.total_storage_bytes_used ?? 0,
      limit: plan?.max_storage_bytes ?? plan?.max_cache_bytes,
      format: formatBytes,
      subtitle: `Cache: ${formatBytes(res.cache_bytes_used)} · Artifacts: ${formatBytes(res.artifact_bytes_used)}`,
    },
    {
      label: "Bandwidth",
      current: res.bandwidth_bytes_used ?? 0,
      limit: plan?.max_bandwidth_bytes,
      format: formatBytes,
    },
    {
      label: "Projects",
      current: res.projects_used ?? 0,
      limit: plan?.max_projects,
      format: (n: number) => String(n),
    },
    {
      label: "Team Members",
      current: res.team_members_used ?? 0,
      limit: plan?.max_team_members,
      format: (n: number) => String(n),
    },
    {
      label: "Concurrent Runs",
      current: res.concurrent_runs ?? 0,
      limit: plan?.max_concurrent_runs,
      format: (n: number) => String(n),
    },
    {
      label: "AI Analyses",
      current: res.ai_analyses_used ?? 0,
      limit: plan?.max_ai_analyses_per_month,
      format: (n: number) => String(n),
      subtitle: "This month",
    },
  ];

  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {usageMetrics.map((metric) => {
        const hasLimit = metric.limit != null;
        const pct = hasLimit
          ? Math.min((metric.current / metric.limit!) * 100, 100)
          : 0;
        return (
          <Card key={metric.label}>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">
                {metric.label}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span>{metric.format(metric.current)}</span>
                <span className="text-muted-foreground">
                  {hasLimit
                    ? `of ${metric.format(metric.limit!)}`
                    : "Unlimited"}
                </span>
              </div>
              {hasLimit ? (
                <Progress value={pct} className="h-2" />
              ) : (
                <Progress value={0} className="h-2" />
              )}
              {metric.subtitle && (
                <p className="text-xs text-muted-foreground">
                  {metric.subtitle}
                </p>
              )}
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}

function PlanCard({
  plan,
  isCurrentPlan,
  onUpgrade,
  isLoading,
}: {
  plan: BillingPlan;
  isCurrentPlan: boolean;
  onUpgrade: () => void;
  isLoading: boolean;
}) {
  return (
    <Card className={isCurrentPlan ? "border-primary" : ""}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>{plan.display_name}</CardTitle>
          {isCurrentPlan && <Badge>Current</Badge>}
        </div>
        <CardDescription>{plan.description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 flex-1">
        <div>
          {plan.slug === "enterprise" ? (
            <>
              <span className="text-3xl font-bold">Custom</span>
            </>
          ) : (
            <>
              <span className="text-3xl font-bold">
                {formatCents(plan.price_cents)}
              </span>
              <span className="text-muted-foreground">
                /{plan.billing_period}
              </span>
              {plan.is_per_seat && (
                <span className="text-sm text-muted-foreground ml-1">
                  per seat
                </span>
              )}
            </>
          )}
        </div>

        <Separator />

        <ul className="space-y-2 text-sm">
          <PlanFeature
            label={
              plan.max_projects
                ? `${plan.max_projects} projects`
                : "Unlimited projects"
            }
          />
          <PlanFeature
            label={
              plan.max_team_members
                ? `${plan.max_team_members} team members`
                : "Unlimited team members"
            }
          />
          <PlanFeature
            label={
              plan.max_storage_bytes
                ? `${formatBytes(plan.max_storage_bytes)} storage`
                : plan.max_cache_bytes
                  ? `${formatBytes(plan.max_cache_bytes)} cache`
                  : "Unlimited storage"
            }
          />
          <PlanFeature
            label={
              plan.max_bandwidth_bytes
                ? `${formatBytes(plan.max_bandwidth_bytes)} bandwidth`
                : "Unlimited bandwidth"
            }
          />
          <PlanFeature
            label={
              plan.max_concurrent_runs
                ? `${plan.max_concurrent_runs} concurrent runs`
                : "Unlimited concurrent runs"
            }
          />
          <PlanFeature
            label={
              plan.cache_ttl_days
                ? `${plan.cache_ttl_days}-day cache TTL`
                : "Unlimited cache TTL"
            }
          />
          <PlanFeature
            label={
              plan.artifact_retention_days
                ? `${plan.artifact_retention_days}-day artifact retention`
                : "Unlimited artifact retention"
            }
          />
          <PlanFeature
            label={
              plan.log_retention_days
                ? `${plan.log_retention_days}-day log retention`
                : "Unlimited log retention"
            }
          />
          <PlanFeature
            label="Container execution"
            included={plan.container_execution}
          />
          <PlanFeature label="Priority queue" included={plan.priority_queue} />
          <PlanFeature label="SSO / SAML" included={plan.sso_enabled} />
          <PlanFeature label="Audit logs" included={plan.audit_logs} />
          <PlanFeature label="AI analysis" included={plan.ai_enabled} />
          <PlanFeature
            label="AI suggestions"
            included={plan.ai_suggestions_enabled}
          />
          <PlanFeature
            label={
              plan.max_ai_analyses_per_month
                ? `${plan.max_ai_analyses_per_month} AI analyses/mo`
                : plan.ai_enabled
                  ? "Unlimited AI analyses"
                  : "No AI analyses"
            }
          />
        </ul>
      </CardContent>
      <CardFooter>
        {isCurrentPlan ? (
          <Button className="w-full" variant="outline" disabled>
            Current Plan
          </Button>
        ) : plan.slug === "enterprise" ? (
          <Button className="w-full" disabled={isLoading}>
            {isLoading ? (
              <Icons.Loader className="h-4 w-4 animate-spin mr-2" />
            ) : null}
            Contact Sales
          </Button>
        ) : plan.price_cents === 0 ? (
          <Button className="w-full" variant="outline" disabled>
            Free
          </Button>
        ) : (
          <Button className="w-full" onClick={onUpgrade} disabled={isLoading}>
            {isLoading ? (
              <Icons.Loader className="h-4 w-4 animate-spin mr-2" />
            ) : null}
            Upgrade
          </Button>
        )}
      </CardFooter>
    </Card>
  );
}

function PlanFeature({
  label,
  included = true,
}: {
  label: string;
  included?: boolean;
}) {
  return (
    <li className="flex items-center gap-2">
      {included ? (
        <Icons.Check className="h-4 w-4 text-primary shrink-0" />
      ) : (
        <Icons.Minus className="h-4 w-4 text-muted-foreground/50 shrink-0" />
      )}
      <span className={included ? "" : "text-muted-foreground/50"}>
        {label}
      </span>
    </li>
  );
}
