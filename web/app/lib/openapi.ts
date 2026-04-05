import { createOpenAPI } from 'fumadocs-openapi/server';
import spec from '../../content/openapi.json';

export const openapi = createOpenAPI({
  // Swagger 2.0 spec — cast needed as fumadocs types expect OpenAPI 3.x
  input: () => ({
    'dagryn-api': spec as any,
  }),
});
