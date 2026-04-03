/**
 * Pre-build script: generates MDX files from the OpenAPI spec.
 * These files are placed in content/docs/api/ and picked up by
 * the normal fumadocs-mdx pipeline.
 *
 * Usage: pnpm generate:api-docs
 */
import { generateFiles } from 'fumadocs-openapi';
import { createOpenAPI } from 'fumadocs-openapi/server';
import spec from '../content/openapi.json';

const openapi = createOpenAPI({
  input: () => ({
    'dagryn-api': spec as any,
  }),
});

await generateFiles({
  input: openapi,
  output: './content/docs/api',
  per: 'operation',
  groupBy: 'route',
});

console.log('OpenAPI docs generated successfully.');
