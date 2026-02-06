import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function usePendingInvitations() {
  return useQuery({
    queryKey: queryKeys.invitations,
    queryFn: async () => {
      const response = await api.listPendingInvitations();
      return response.data;
    },
  });
}
