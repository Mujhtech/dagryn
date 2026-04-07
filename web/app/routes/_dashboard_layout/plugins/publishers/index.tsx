import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute(
  "/_dashboard_layout/plugins/publishers/",
)({
  beforeLoad: () => {
    throw redirect({ to: "/plugins/browse" });
  },
});
