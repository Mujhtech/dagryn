import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useOfficialPlugins(params?: {
  q?: string;
  type?: string;
  sort?: string;
}) {
  return useQuery({
    queryKey: queryKeys.officialPlugins(params?.q, params?.type, params?.sort),
    queryFn: async () => {
      const { data } = await api.listOfficialPlugins(params);
      return data;
    },
  });
}
