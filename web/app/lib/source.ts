import { loader } from 'fumadocs-core/source';

// Use import.meta.glob to eagerly load all doc and meta files at build time.
// This avoids importing collections/server which uses node:path.
const docModules = import.meta.glob<any>('../../content/docs/**/*.{mdx,md}', {
  query: { collection: 'docs' },
  eager: true,
});

const metaModules = import.meta.glob<{ default: any }>('../../content/docs/**/*.{json,yaml}', {
  query: { collection: 'docs' },
  import: 'default',
  eager: true,
});

// Build the files array that fumadocs-core/source expects
const files: any[] = [];

for (const [key, mod] of Object.entries(docModules)) {
  // key is like "../../content/docs/getting-started.mdx"
  // Extract relative path from content/docs/
  const path = key.replace(/^.*?content\/docs\//, '');

  files.push({
    type: 'page' as const,
    path,
    data: {
      ...mod,
      ...(mod.frontmatter ?? {}),
      body: mod.default,
      toc: mod.toc,
      structuredData: mod.structuredData,
      _exports: mod,
    },
  });
}

for (const [key, data] of Object.entries(metaModules)) {
  const path = key.replace(/^.*?content\/docs\//, '');
  files.push({
    type: 'meta' as const,
    path,
    data,
  });
}

export const source = loader(
  { files },
  {
    baseUrl: '/docs',
  },
);
