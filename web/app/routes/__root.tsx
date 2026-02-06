import { Outlet, createRootRoute, useLocation } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/router-devtools";
import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { queryClient } from "~/lib/query-client";
import { AuthProvider } from "~/lib/auth";
import { AppSidebar } from "~/components/app-sidebar";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "~/components/ui/sidebar";
import { Separator } from "~/components/ui/separator";

export const Route = createRootRoute({
  component: RootComponent,
});

// Routes that don't need the sidebar layout
const publicRoutes = [
  "/login",
  "/auth/github/callback",
  "/auth/google/callback",
  "/auth/device",
];

function RootComponent() {
  const location = useLocation();
  const isPublicRoute = publicRoutes.some((route) =>
    location.pathname.startsWith(route)
  );

  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        {isPublicRoute ? (
          <Outlet />
        ) : (
          <SidebarProvider
            style={
              {
                "--sidebar-width": "calc(var(--spacing) * 72)",
              } as React.CSSProperties
            }
          >
            <AppSidebar variant="inset" />
            <SidebarInset>
              <header className="flex h-(--header-height) shrink-0 items-center gap-2 border-b transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-(--header-height)">
                <div className="flex w-full items-center gap-1 px-4 lg:gap-2 lg:px-6">
                  <SidebarTrigger className="-ml-1" />
                  <Separator
                    orientation="vertical"
                    className="mx-2 data-[orientation=vertical]:h-4"
                  />
                  <Breadcrumb />
                  <div className="ml-auto flex items-center gap-2"></div>
                </div>
              </header>
              <div className="flex flex-1 flex-col">
                <Outlet />
              </div>
            </SidebarInset>
          </SidebarProvider>
        )}
        {import.meta.env.DEV && <TanStackRouterDevtools />}
        {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
      </AuthProvider>
    </QueryClientProvider>
  );
}

function Breadcrumb() {
  const location = useLocation();
  const paths = location.pathname.split("/").filter(Boolean);

  if (paths.length === 0) {
    return <h1 className="text-lg font-semibold">Dashboard</h1>;
  }

  return (
    <nav className="flex items-center gap-2 text-sm">
      {paths.map((path, index) => (
        <span key={path} className="flex items-center gap-2">
          {index > 0 && <span className="text-muted-foreground">/</span>}
          <span
            className={
              index === paths.length - 1
                ? "font-medium"
                : "text-muted-foreground"
            }
          >
            {formatPathSegment(path)}
          </span>
        </span>
      ))}
    </nav>
  );
}

function formatPathSegment(segment: string): string {
  // Handle UUIDs - shorten them
  if (
    segment.match(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
    )
  ) {
    return segment.slice(0, 8) + "...";
  }
  // Capitalize first letter
  return segment.charAt(0).toUpperCase() + segment.slice(1);
}
