import type { FunctionComponent } from "react";
import type { IconProps } from "@phosphor-icons/react";
import { Icons } from "~/components/icons";
import { useCapabilities } from "./queries/use-capabilities";

const baseNavItems = [
  { title: "Dashboard", url: "/dashboard", icon: Icons.Dashboard },
  { title: "Projects", url: "/projects", icon: Icons.Folder },
  { title: "Teams", url: "/teams", icon: Icons.Users },
  { title: "Invitations", url: "/invitations", icon: Icons.ListDetails },
  { title: "Plugins", url: "/plugins/browse", icon: Icons.Package },
];

const dynamicNavMap: Record<string, { title: string; url: string; icon: FunctionComponent<IconProps> }> = {
  license: { title: "License", url: "/license", icon: Icons.Key },
  billing: { title: "Billing", url: "/billing", icon: Icons.CreditCard },
};

export function useNavItems() {
  const { data: capabilities } = useCapabilities();

  const dynamicItems = (capabilities?.nav ?? [])
    .filter((item) => item.enabled && dynamicNavMap[item.key])
    .map((item) => dynamicNavMap[item.key]);

  return [...baseNavItems, ...dynamicItems];
}
