import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/_dashboard_layout/projects/new/")({
  beforeLoad: () => {
    throw redirect({ to: "/projects/new/github" });
  },
});
