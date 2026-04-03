import { createClientAPIPage } from 'fumadocs-openapi/ui/create-client';
import type { FC } from 'react';
import spec from '../../content/openapi.json';

// Client-side API page renderer
const ClientAPIPageInner = createClientAPIPage();

/**
 * APIPage component used by generated OpenAPI MDX files.
 * Bridges the generated MDX props (document + operations) to
 * the client API page component (payload + operations).
 */
export const APIPage: FC<{
  document: string;
  operations?: Array<{ path: string; method: string }>;
  webhooks?: Array<{ name: string; method: string }>;
  showTitle?: boolean;
  showDescription?: boolean;
}> = ({ operations, webhooks, showTitle, showDescription }: any) => {
  return (
    <ClientAPIPageInner
      payload={{
        bundled: spec as any,
      }}
      operations={operations}
      webhooks={webhooks}
      showTitle={showTitle}
      showDescription={showDescription}
    />
  );
};

// Re-export for direct usage
export { ClientAPIPageInner as ClientAPIPage };
