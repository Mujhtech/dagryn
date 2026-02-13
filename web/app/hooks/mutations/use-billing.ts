import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useCreateCheckoutSession() {
  return useMutation({
    mutationFn: (data: {
      plan_slug: string;
      success_url: string;
      cancel_url: string;
    }) => api.createCheckoutSession(data),
    onSuccess: ({ data }) => {
      if (data.url) {
        window.location.href = data.url;
      }
    },
  });
}

export function useCreatePortalSession() {
  return useMutation({
    mutationFn: (returnUrl: string) => api.createPortalSession(returnUrl),
    onSuccess: ({ data }) => {
      if (data.url) {
        window.location.href = data.url;
      }
    },
  });
}

export function useCancelSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (atPeriodEnd?: boolean) =>
      api.cancelSubscription(atPeriodEnd),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.billingOverview,
      });
    },
  });
}
