import { createOpenAPI } from 'fumadocs-openapi/server';
import spec from '../../content/openapi.json';

export const openapi = createOpenAPI({
  input: () => ({
    'dagryn-api': spec,
  }),
});
