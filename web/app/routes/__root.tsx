import { HeadContent, Outlet, createRootRoute } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/router-devtools";
import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { queryClient } from "~/lib/query-client";
import { AuthProvider } from "~/lib/auth";
import { RootProvider } from "fumadocs-ui/provider/tanstack";

export const Route = createRootRoute({
  component: RootComponent,
});

function RootComponent() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RootProvider
          theme={{
            enabled: false,
          }}
        >
          <HeadContent />
          <Outlet />
          {import.meta.env.DEV && <TanStackRouterDevtools />}
          {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
        </RootProvider>
      </AuthProvider>
    </QueryClientProvider>
  );
}
