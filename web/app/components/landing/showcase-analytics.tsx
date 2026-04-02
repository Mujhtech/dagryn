import { AnalyticsDashboard } from "~/components/analytics/analytics-dashboard";
import { ShowcaseFrame } from "./showcase-frame";
import { SHOWCASE_ANALYTICS } from "./showcase-data";

export default function ShowcaseAnalytics() {
  return (
    <ShowcaseFrame
      label="Analytics"
      title="Understand your builds at every level"
      description="KPIs, charts, and breakdowns for runs, cache, bandwidth, artifacts, AI, and audit logs."
      gradient="bottom-heavy"
      maxHeight="36rem"
      perspective
    >
      <AnalyticsDashboard
        data={SHOWCASE_ANALYTICS}
        isLoading={false}
        days={14}
        onDaysChange={() => {}}
        title="Project Analytics"
        subtitle="Last 14 days"
        containerClassName="px-6 md:px-8"
      />
    </ShowcaseFrame>
  );
}
