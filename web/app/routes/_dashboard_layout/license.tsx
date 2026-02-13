import { createFileRoute } from "@tanstack/react-router";
import { useLicenseStatus } from "~/hooks/queries/use-license-status";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/_dashboard_layout/license")({
  component: LicensePage,
});

function LicensePage() {
  const { data: license, isLoading } = useLicenseStatus();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!license) {
    return (
      <div className="p-6">
        <p className="text-muted-foreground">Unable to load license status.</p>
      </div>
    );
  }

  // Cloud mode — license page is not applicable
  if (license.mode === "cloud") {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold">License</h1>
        <p className="text-muted-foreground mt-2">
          License management is not applicable for cloud deployments. Your
          features and limits are managed through your{" "}
          <a href="/billing" className="text-primary underline">
            billing plan
          </a>
          .
        </p>
      </div>
    );
  }

  const editionColors: Record<string, string> = {
    community: "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200",
    pro: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
    enterprise:
      "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  };

  return (
    <div className="p-6 space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold">License</h1>
        <p className="text-muted-foreground">
          View your Dagryn license status and features.
        </p>
      </div>

      {/* Edition & Status */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>License Status</CardTitle>
              <CardDescription>
                {license.licensed
                  ? `Licensed to ${license.customer}`
                  : "Running as Community edition"}
              </CardDescription>
            </div>
            <Badge
              className={editionColors[license.edition] || editionColors.community}
            >
              {license.edition.charAt(0).toUpperCase() +
                license.edition.slice(1)}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">Seats</span>
              <p className="font-medium">{license.seats}</p>
            </div>
            {license.expires_at && (
              <div>
                <span className="text-muted-foreground">Expires</span>
                <p className="font-medium">
                  {new Date(license.expires_at).toLocaleDateString()}
                  {license.days_remaining != null && (
                    <span className="text-muted-foreground ml-1">
                      ({license.days_remaining} days)
                    </span>
                  )}
                </p>
              </div>
            )}
            {license.grace_period && (
              <div className="col-span-2">
                <p className="text-destructive text-sm font-medium">
                  License expired - running in grace period. Features will be
                  disabled soon.
                </p>
              </div>
            )}
            {license.expiring && !license.grace_period && (
              <div className="col-span-2">
                <p className="text-yellow-600 dark:text-yellow-400 text-sm font-medium">
                  License expiring soon. Please renew to avoid interruption.
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Features */}
      <Card>
        <CardHeader>
          <CardTitle>Features</CardTitle>
          <CardDescription>
            Features available in your current edition.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {Object.entries(license.features).map(([feature, enabled]) => (
              <div
                key={feature}
                className="flex items-center gap-2 text-sm"
              >
                {enabled ? (
                  <Icons.Check className="h-4 w-4 text-green-500" />
                ) : (
                  <Icons.Close className="h-4 w-4 text-muted-foreground" />
                )}
                <span className={enabled ? "" : "text-muted-foreground"}>
                  {formatFeatureName(feature)}
                </span>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Limits */}
      <Card>
        <CardHeader>
          <CardTitle>Resource Limits</CardTitle>
          <CardDescription>
            Current resource usage against your licensed limits.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <LimitRow
              label="Projects"
              current={license.limits.projects.current}
              limit={license.limits.projects.limit}
            />
            <LimitRow
              label="Team Members"
              current={license.limits.team_members.current}
              limit={license.limits.team_members.limit}
            />
            <LimitRow
              label="Concurrent Runs"
              current={license.limits.concurrent_runs.current}
              limit={license.limits.concurrent_runs.limit}
            />
          </div>
        </CardContent>
      </Card>

      {!license.licensed && (
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">
              Want to unlock more features?{" "}
              <a
                href="https://dagryn.dev/pricing"
                className="text-primary underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                View pricing
              </a>{" "}
              or run <code className="text-xs bg-muted px-1 py-0.5 rounded">dagryn license activate &lt;key&gt;</code> to activate a license.
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function LimitRow({
  label,
  current,
  limit,
}: {
  label: string;
  current: number;
  limit: number | null;
}) {
  const pct = limit != null && limit > 0 ? Math.min((current / limit) * 100, 100) : 0;

  return (
    <div>
      <div className="flex justify-between text-sm mb-1">
        <span>{label}</span>
        <span className="text-muted-foreground">
          {current} / {limit != null ? limit : "Unlimited"}
        </span>
      </div>
      {limit != null && (
        <div className="h-2 bg-muted rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all ${
              pct > 90 ? "bg-destructive" : pct > 70 ? "bg-yellow-500" : "bg-primary"
            }`}
            style={{ width: `${pct}%` }}
          />
        </div>
      )}
    </div>
  );
}

function formatFeatureName(feature: string): string {
  return feature
    .split("_")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}
