import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";
import { GithubInfo } from "fumadocs-ui/components/github-info";

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: "Dagryn Docs",
    },
    links: [
      {
        type: "custom",
        children: <GithubInfo owner="mujhtech" repo="dagryn" />,
      },
    ],
    themeSwitch: {
      enabled: false,
    },
    i18n: false,
  };
}
