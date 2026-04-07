import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/runs/",
)({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/projects/$projectId",
      params: { projectId: params.projectId },
    });
  },
});
