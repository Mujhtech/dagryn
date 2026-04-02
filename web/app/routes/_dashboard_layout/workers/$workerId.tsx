import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { generateMetadata } from "~/lib/metadata";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/_dashboard_layout/workers/$workerId")({
  component: WorkerDetailPage,
  head: () => generateMetadata({ title: "Worker Detail" }),
});

function WorkerDetailPage() {
  const { workerId } = Route.useParams();
  const { data, isLoading, error } = useQuery({
    queryKey: ["worker", workerId],
    queryFn: async () => {
      const { data } = await api.getWorker(workerId);
      return data;
    },
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-48">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }
  if (error || !data) {
    return <div className="p-6 text-muted-foreground">Worker not found.</div>;
  }

  return (
    <div className="space-y-6 px-6 py-3">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">{data.hostname}</h1>
        <p className="text-muted-foreground">Worker execution details and runtime status.</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Status</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant={data.status === "online" ? "default" : "secondary"}>
              {data.status}
            </Badge>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Active Tasks</CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-semibold">{data.active_tasks}</CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Runtime Info</CardTitle>
          <CardDescription>Platform and capability details.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div>Worker ID: {data.id}</div>
          <div>Cluster ID: {data.cluster_id ?? "-"}</div>
          <div>OS/Arch: {data.os}/{data.arch}</div>
          <div>Environment: {data.environment}</div>
          <div>Version: {data.version}</div>
          <div>Last heartbeat: {new Date(data.last_heartbeat_at).toLocaleString()}</div>
          <div>
            Capabilities: {data.capabilities?.length ? data.capabilities.join(", ") : "none"}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
