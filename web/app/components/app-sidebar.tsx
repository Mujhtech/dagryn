import { useLocation } from "@tanstack/react-router";
import { Icons } from "./icons";

import { useAuth } from "~/lib/auth";
import { useLicenseStatus } from "~/hooks/queries/use-license-status";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "~/components/ui/sidebar";
import { NavUser } from "./nav-user";
import { NavMain } from "./nav-main";
import { Logo } from "./logo";

const baseNavItems = [
  {
    title: "Dashboard",
    url: "/dashboard",
    icon: Icons.Dashboard,
  },
  {
    title: "Projects",
    url: "/projects",
    icon: Icons.Folder,
  },
  {
    title: "Teams",
    url: "/teams",
    icon: Icons.Users,
  },
  {
    title: "Invitations",
    url: "/invitations",
    icon: Icons.ListDetails,
  },
  {
    title: "Plugins",
    url: "/plugins/browse",
    icon: Icons.Package,
  },
];

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const location = useLocation();
  const { user } = useAuth();
  const { data: license } = useLicenseStatus();

  const isCloud = license?.mode === "cloud";

  // Build nav items based on deployment mode.
  // Cloud mode: show Billing, hide License.
  // Self-hosted mode: show License, hide Billing.
  const navItems = [
    ...baseNavItems,
    ...(isCloud
      ? [{ title: "Billing", url: "/billing", icon: Icons.CreditCard }]
      : [{ title: "License", url: "/license", icon: Icons.Key }]),
  ];

  const isActive = (url: string) => {
    if (url === "/") {
      return location.pathname === "/";
    }
    return location.pathname.startsWith(url);
  };

  return (
    <Sidebar collapsible="offcanvas" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              asChild
              className="data-[slot=sidebar-menu-button]:p-1.5! h-fit! [&>svg]:size-5"
            >
              <a href="#">
                {/* <IconInnerShadowTop className="!size-5" /> */}
                <Logo />
                <span className="text-base font-semibold">DAGRYN.</span>
              </a>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={navItems} isActive={isActive} />
        {/* <NavDocuments items={data.documents} />
        <NavSecondary items={data.navSecondary} className="mt-auto" /> */}
      </SidebarContent>
      <SidebarFooter>
        <NavUser
          user={{
            email: user?.email || "",
            name: user?.name || "",
            avatar: user?.avatar_url || "",
          }}
        />
      </SidebarFooter>
    </Sidebar>
  );
}
