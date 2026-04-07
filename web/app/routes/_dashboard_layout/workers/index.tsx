import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/_dashboard_layout/workers/")({
  beforeLoad: () => {
    throw redirect({ to: "/clusters", search: { teamId: undefined } });
  },
});
