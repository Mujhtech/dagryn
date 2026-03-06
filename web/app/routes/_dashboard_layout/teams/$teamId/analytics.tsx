import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useTeam, useTeamAnalytics } from "~/hooks/queries";
import { AnalyticsDashboard } from "~/components/analytics/analytics-dashboard";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/teams/$teamId/analytics",
)({
  component: TeamAnalyticsPage,
  head: () => {
    return generateMetadata({ title: "Team Analytics" });
  },
});

function TeamAnalyticsPage() {
  const { teamId } = Route.useParams();
  const [days, setDays] = useState(30);
  const { data: team } = useTeam(teamId);
  const { data, isLoading } = useTeamAnalytics(teamId, days);

  return (
    <AnalyticsDashboard
      data={data}
      isLoading={isLoading}
      days={days}
      onDaysChange={setDays}
      title="Team Analytics"
      subtitle="Overview across all team projects"
      backLink={{ to: "/teams/$teamId", params: { teamId } }}
      badgeLabel={team?.name}
    />
  );
}
