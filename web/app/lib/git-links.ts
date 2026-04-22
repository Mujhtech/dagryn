export type GitPlatform = "github" | "gitlab" | "bitbucket" | "unknown";

type RepoInfo = {
  platform: GitPlatform;
  host: string;
  path: string;
  webUrl: string;
};

function parseRepoUrl(repoUrl?: string | null): RepoInfo | null {
  if (!repoUrl) return null;

  const trimmed = repoUrl.trim();
  if (!trimmed) return null;

  const match = trimmed.match(
    /^(?:https?:\/\/|ssh:\/\/(?:.+@)?|(?:.+)@)([^/:]+)[:/]([^?#]+?)(?:\.git)?\/?$/i,
  );

  if (!match) return null;

  const host = match[1].toLowerCase();
  const path = match[2].replace(/^\/+|\/+$/g, "");

  if (!path) return null;

  let platform: GitPlatform = "unknown";
  if (host.includes("github.com")) {
    platform = "github";
  } else if (host.includes("gitlab.com")) {
    platform = "gitlab";
  } else if (host.includes("bitbucket.org")) {
    platform = "bitbucket";
  }

  return {
    platform,
    host,
    path,
    webUrl: `https://${host}/${path}`,
  };
}

export function getRepoWebUrl(repoUrl?: string | null): string | null {
  return parseRepoUrl(repoUrl)?.webUrl ?? null;
}

export function getRepoDisplayName(repoUrl?: string | null): string {
  return parseRepoUrl(repoUrl)?.path ?? "";
}

export function getRepoPlatformName(repoUrl?: string | null): string {
  const platform = parseRepoUrl(repoUrl)?.platform;
  if (platform === "github") return "GitHub";
  if (platform === "gitlab") return "GitLab";
  if (platform === "bitbucket") return "Bitbucket";
  return "Repository";
}

export function getCommitUrl(
  repoUrl: string | null | undefined,
  commitSha: string | null | undefined,
): string | null {
  const repo = parseRepoUrl(repoUrl);
  if (!repo || !commitSha) return null;

  const sha = commitSha.trim();
  if (!sha) return null;

  switch (repo.platform) {
    case "bitbucket":
      return `${repo.webUrl}/commits/${encodeURIComponent(sha)}`;
    default:
      return `${repo.webUrl}/commit/${encodeURIComponent(sha)}`;
  }
}

export function getPullRequestUrl(
  repoUrl: string | null | undefined,
  prNumber: number | null | undefined,
): string | null {
  const repo = parseRepoUrl(repoUrl);
  if (!repo || prNumber == null) return null;

  switch (repo.platform) {
    case "gitlab":
      return `${repo.webUrl}/-/merge_requests/${prNumber}`;
    default:
      return `${repo.webUrl}/pull/${prNumber}`;
  }
}
