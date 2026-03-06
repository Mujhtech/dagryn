import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { queryKeys } from "../../lib/query-client";

export function useCapabilities() {
  return useQuery({
    queryKey: queryKeys.capabilities,
    queryFn: async () => {
      const { data } = await api.getCapabilities();
      return data;
    },
    staleTime: 60_000,
  });
}
