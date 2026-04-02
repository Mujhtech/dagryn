import { createFileRoute, notFound } from '@tanstack/react-router';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import {
  DocsBody,
  DocsDescription,
  DocsPage,
  DocsTitle,
} from 'fumadocs-ui/layouts/docs/page';
import { source } from '~/lib/source';
import { baseOptions } from '~/lib/layout.shared';
import { useMDXComponents } from '~/components/mdx';
import { ClientAPIPage } from '~/components/api-page';
import browserCollections from 'collections/browser';
import { useFumadocsLoader } from 'fumadocs-core/source/client';
import { type ReactNode, Suspense } from 'react';

export const Route = createFileRoute('/docs/$')({
  component: Page,
  loader: async ({ params }) => {
    const slugs = params._splat?.split('/') ?? [];
    const page = source.getPage(slugs);
    if (!page) throw notFound();

    if (page.data.type === 'openapi') {
      return {
        type: 'openapi' as const,
        title: page.data.title,
        description: page.data.description,
        props: await page.data.getClientAPIPageProps(),
        pageTree: source.pageTree,
      };
    }

    await clientLoader.preload(page.path);

    return {
      type: 'docs' as const,
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
  const data = useFumadocsLoader(Route.useLoaderData());
  let content: ReactNode;

  if (data.type === 'openapi') {
    content = (
      <DocsPage full>
        <DocsTitle>{data.title}</DocsTitle>
        <DocsDescription>{data.description}</DocsDescription>
        <DocsBody>
          <ClientAPIPage {...data.props} />
        </DocsBody>
      </DocsPage>
    );
  } else {
    content = <Suspense>{clientLoader.useContent(data.path)}</Suspense>;
  }

  return (
    <DocsLayout {...baseOptions()} tree={data.pageTree}>
      {content}
    </DocsLayout>
  );
}
