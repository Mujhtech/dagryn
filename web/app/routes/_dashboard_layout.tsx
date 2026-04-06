import {
  createFileRoute,
  Link,
  Navigate,
  useLocation,
} from "@tanstack/react-router";
import { Outlet } from "@tanstack/react-router";
import { AppSidebar } from "~/components/app-sidebar";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "~/components/ui/sidebar";
import { Separator } from "~/components/ui/separator";
import {
  Breadcrumb as BreadcrumbRoot,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "~/components/ui/breadcrumb";
import { useAuth } from "~/lib/auth";
import { Icons } from "~/components/icons";
import { useLicenseStatus } from "~/hooks/queries/use-license-status";
import { useFavicon } from "~/hooks/use-favicon";

export const Route = createFileRoute("/_dashboard_layout")({
  component: LayoutComponent,
});

function LayoutComponent() {
  const { isAuthenticated, isLoading } = useAuth();
  useFavicon(null);

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
        <LicenseBanner />
        <div className="flex flex-1 flex-col">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}

function Breadcrumb() {
  const location = useLocation();
  const segments = location.pathname.split("/").filter(Boolean);

  if (segments.length === 0) {
    return <h1 className="text-lg font-semibold">Dashboard</h1>;
  }

  return (
    <BreadcrumbRoot>
      <BreadcrumbList>
        {segments.map((segment, index) => {
          const href = "/" + segments.slice(0, index + 1).join("/");
          const isLast = index === segments.length - 1;

          return (
            <BreadcrumbItem key={href}>
              {index > 0 && <BreadcrumbSeparator />}
              {isLast ? (
                <BreadcrumbPage>{formatPathSegment(segment)}</BreadcrumbPage>
              ) : (
                <BreadcrumbLink asChild>
                  <Link to={href}>{formatPathSegment(segment)}</Link>
                </BreadcrumbLink>
              )}
            </BreadcrumbItem>
          );
        })}
      </BreadcrumbList>
    </BreadcrumbRoot>
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

function LicenseBanner() {
  const { data: license } = useLicenseStatus();

  // No banner in cloud mode or when data is unavailable
  if (!license || license.mode === "cloud") return null;

  if (license.grace_period) {
    return (
      <div className="bg-destructive/10 border-b border-destructive/20 px-4 py-2 text-sm text-destructive">
        Your license has expired. Enterprise features will be disabled soon.{" "}
        <a
          href="https://dagryn.dev/contact"
          className="underline font-medium"
          target="_blank"
          rel="noopener noreferrer"
        >
          Renew now.
        </a>
      </div>
    );
  }

  if (license.expiring && license.days_remaining != null) {
    return (
      <div className="bg-yellow-500/10 border-b border-yellow-500/20 px-4 py-2 text-sm text-yellow-700 dark:text-yellow-400">
        Your license expires in {license.days_remaining} days.{" "}
        <a
          href="https://dagryn.dev/contact"
          className="underline font-medium"
          target="_blank"
          rel="noopener noreferrer"
        >
          Contact us to renew.
        </a>
      </div>
    );
  }

  return null;
}
