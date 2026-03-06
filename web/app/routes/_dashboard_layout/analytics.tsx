import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useUserAnalytics } from "~/hooks/queries";
import { AnalyticsDashboard } from "~/components/analytics/analytics-dashboard";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute("/_dashboard_layout/analytics")({
  component: UserAnalyticsPage,
  head: () => {
    return generateMetadata({ title: "Analytics" });
  },
});

function UserAnalyticsPage() {
  const [days, setDays] = useState(30);
  const { data, isLoading } = useUserAnalytics(days);

  return (
    <AnalyticsDashboard
      data={data}
      isLoading={isLoading}
      days={days}
      onDaysChange={setDays}
      title="Analytics"
      subtitle="Overview across all your projects"
    />
  );
}
