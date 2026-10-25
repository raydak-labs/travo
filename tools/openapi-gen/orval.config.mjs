// Types go to shared (one file per schema), request functions to the
// frontend, which owns the HTTP client they call (apiFetch).
export default {
  travo: {
    input: '../../shared/openapi.json',
    output: {
      target: '../../frontend/src/api/endpoints.ts',
      schemas: '../../shared/src/generated/model',
      client: 'fetch',
      mode: 'single',
      clean: ['../../shared/src/generated/model'],
      override: {
        mutator: { path: '../../frontend/src/lib/api-client.ts', name: 'apiFetch' },
        fetch: { includeHttpResponseReturnType: false },
      },
    },
  },
};
