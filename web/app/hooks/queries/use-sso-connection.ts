import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useSSOConnection(teamId: string) {
  return useQuery({
    queryKey: queryKeys.ssoConnection(teamId),
    queryFn: async () => {
      const { data } = await api.getSSOConnection(teamId);
      return data;
    },
    enabled: !!teamId,
    retry: (failureCount, error) => {
      // Don't retry on 404 (no SSO configured yet)
      if (error && "status" in error && (error as { status: number }).status === 404) {
        return false;
      }
      return failureCount < 3;
    },
  });
}
