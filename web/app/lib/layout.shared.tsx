import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: "Dagryn Docs",
    },
    githubUrl: "https://github.com/mujhtech/dagryn",
    themeSwitch: {
      enabled: false,
    },
    i18n: false,
  };
}
