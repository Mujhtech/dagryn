import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";

export function useAuthProviders() {
  return useQuery({
    queryKey: ["authProviders"],
    queryFn: async () => {
      const response = await api.getAuthProviders();
      return response.data;
    },
    staleTime: 1000 * 60 * 60, // 1 hour - providers don't change often
  });
}
