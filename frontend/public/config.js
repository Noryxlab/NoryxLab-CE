/*
 * Default runtime configuration (Community Edition, in-cluster defaults).
 *
 * In Kubernetes this file is replaced by a ConfigMap mount. The EE overlay
 * ships its own config.js instead of patching the application source.
 */
window.__NORYX_CONFIG__ = {
  apiBaseUrl: '',
  authMode: 'oidc',
  oidc: {
    url: 'https://auth.example.local',
    realm: 'noryx',
    clientId: 'noryx-frontend',
  },
  edition: 'ce',
  defaultLocale: 'fr',
  brand: {
    productName: 'NoryxLab',
    editionLabel: 'Community Edition',
    logoUrl: '/favicon.svg',
    tokens: {},
  },
  links: {
    documentation: 'https://docs.noryxlab.ai',
    apiReference: '/swagger',
    status: 'https://status.noryxlab.ai',
  },
  features: {},
};
