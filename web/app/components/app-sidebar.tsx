import { useLocation } from "@tanstack/react-router";

import { useAuth } from "~/lib/auth";
import { useNavItems } from "~/hooks/use-nav-items";
import { useHealth } from "~/hooks/queries/use-health";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  // SidebarMenuButton,
  SidebarMenuItem,
} from "~/components/ui/sidebar";
import { NavUser } from "./nav-user";
import { NavMain } from "./nav-main";
import { Logo } from "./logo";

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const location = useLocation();
  const { user } = useAuth();
  const navItems = useNavItems();
  const { data: health } = useHealth();

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
            <Logo className="h-8 w-8" />
            {/* <SidebarMenuButton
              asChild
              className="data-[slot=sidebar-menu-button]:p-1.5! h-fit! [&>svg]:size-5"
            >
             
            </SidebarMenuButton> */}
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={navItems} isActive={isActive} />
        {/* <NavDocuments items={data.documents} />
        <NavSecondary items={data.navSecondary} className="mt-auto" /> */}
      </SidebarContent>
      <SidebarFooter className="gap-1">
        <NavUser
          user={{
            email: user?.email || "",
            name: user?.name || "",
            avatar: user?.avatar_url || "",
          }}
        />
        {health?.version && (
          <div className="px-3">
            <a
              href={`https://github.com/mujhtech/dagryn/releases/tag/${health.version}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-muted-foreground hover:text-foreground text-[0.5rem] transition-colors"
            >
              v{health.version.replace(/^v/, "")}
            </a>
          </div>
        )}
      </SidebarFooter>
    </Sidebar>
  );
}
