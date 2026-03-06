import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";

export function useRunLogs(
  projectId: string,
  runId: string,
  options?: {
    page?: number;
    perPage?: number;
    task?: string;
    enabled?: boolean;
  }
) {
  return useQuery({
    queryKey: [
      "run-logs",
      projectId,
      runId,
      options?.page,
      options?.perPage,
      options?.task,
    ],
    queryFn: () =>
      api.getRunLogs(projectId, runId, {
        page: options?.page,
        perPage: options?.perPage,
        task: options?.task,
      }),
    enabled: options?.enabled !== false,
    staleTime: 30 * 1000, // 30 seconds
  });
}
