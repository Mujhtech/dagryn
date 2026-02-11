import { createFileRoute, Navigate, useLocation } from "@tanstack/react-router";
import { Outlet } from "@tanstack/react-router";
import { AppSidebar } from "~/components/app-sidebar";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "~/components/ui/sidebar";
import { Separator } from "~/components/ui/separator";
import { useAuth } from "~/lib/auth";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/_dashboard_layout")({
  component: LayoutComponent,
});

function LayoutComponent() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" />;
  }

  return (
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
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
    )
  ) {
    return segment.slice(0, 8) + "...";
  }
  // Capitalize first letter
  return segment.charAt(0).toUpperCase() + segment.slice(1);
}
