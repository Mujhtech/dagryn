import { Badge } from "~/components/ui/badge";
import { Icons } from "~/components/icons";
import { cn } from "~/lib/utils";

export function RunStatusIcon({
  status,
  className,
}: {
  status: string;
  className?: string;
}) {
  switch (status) {
    case "success":
      return <Icons.CheckCircle className={cn("text-green-500", className)} />;
    case "failed":
      return <Icons.XCircle className={cn("text-red-500", className)} />;
    case "running":
      return (
        <Icons.Loader className={cn("text-blue-500 animate-spin", className)} />
      );
    case "pending":
      return <Icons.Circle className={cn("text-yellow-500", className)} />;
    case "cancelled":
      return <Icons.XCircle className={cn("text-gray-500", className)} />;
    case "stale":
      return <Icons.WifiOff className={cn("text-yellow-500", className)} />;
    default:
      return <Icons.Circle className={cn("text-gray-400", className)} />;
  }
}

export function TaskStatusIcon({ status }: { status: string }) {
  switch (status) {
    case "success":
      return <Icons.CheckCircle className="h-5 w-5 text-green-500" />;
    case "failed":
      return <Icons.XCircle className="h-5 w-5 text-red-500" />;
    case "running":
      return <Icons.Loader className="h-5 w-5 text-blue-500 animate-spin" />;
    case "cached":
      return <Icons.Database className="h-5 w-5 text-purple-500" />;
    case "pending":
      return <Icons.Circle className="h-5 w-5 text-yellow-500" />;
    default:
      return <Icons.Circle className="h-5 w-5 text-gray-400" />;
  }
}

export function StatusBadge({ status }: { status: string }) {
  const variants: Record<
    string,
    "default" | "secondary" | "destructive" | "outline"
  > = {
    success: "default",
    failed: "destructive",
    running: "default",
    pending: "default",
    cancelled: "secondary",
    cached: "secondary",
    skipped: "outline",
    stale: "outline",
  };

  const labels: Record<string, string> = {
    stale: "Connection Lost",
  };

  return (
    <Badge
      variant={variants[status] || "outline"}
      className={status === "stale" ? "border-yellow-500 text-yellow-500" : ""}
    >
      {labels[status] || status.charAt(0).toUpperCase() + status.slice(1)}
    </Badge>
  );
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000);
  return `${minutes}m ${seconds}s`;
}

export function formatBytes(bytes: number): string {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const exp = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  const value = bytes / Math.pow(1024, exp);
  return `${value.toFixed(value >= 10 || exp === 0 ? 0 : 1)} ${units[exp]}`;
}
