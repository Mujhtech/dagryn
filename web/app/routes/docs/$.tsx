import { createFileRoute, notFound } from '@tanstack/react-router';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import {
  DocsBody,
  DocsDescription,
  DocsPage,
  DocsTitle,
} from 'fumadocs-ui/layouts/docs/page';
import { RootProvider } from 'fumadocs-ui/provider/tanstack';
import { source } from '~/lib/source';
import { baseOptions } from '~/lib/layout.shared';
import { useMDXComponents } from '~/components/mdx';
import browserCollections from 'collections/browser';
import { Suspense } from 'react';

export const Route = createFileRoute('/docs/$')({
  component: Page,
  loader: async ({ params }) => {
    const slugs = params._splat?.split('/') ?? [];
    const page = source.getPage(slugs);
    if (!page) throw notFound();

    await clientLoader.preload(page.path);

    return {
      path: page.path,
      pageTree: source.pageTree,
    };
  },
});

const clientLoader = browserCollections.docs.createClientLoader({
  component(
    { toc, frontmatter, default: MDX },
    _props: undefined,
  ) {
    return (
      <DocsPage toc={toc}>
        <DocsTitle>{frontmatter.title}</DocsTitle>
        <DocsDescription>{frontmatter.description}</DocsDescription>
        <DocsBody>
          <MDX components={useMDXComponents()} />
        </DocsBody>
      </DocsPage>
    );
  },
});

function Page() {
  const data = Route.useLoaderData();

  return (
    <RootProvider theme={{ enabled: false }}>
      <DocsLayout {...baseOptions()} tree={data.pageTree}>
        <Suspense>{clientLoader.useContent(data.path)}</Suspense>
      </DocsLayout>
    </RootProvider>
  );
}
