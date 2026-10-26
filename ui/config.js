// Hooty v2 es el backend de lectura. En desarrollo local apunta al :8080 del
// microservicio; en producción se espera un gateway del mismo origen.
const hootyLocal = ['localhost', '127.0.0.1', '::1'].includes(globalThis.location?.hostname);

window.__GLAUK_CONFIG__ = {
  apiBaseUrl: hootyLocal ? 'http://localhost:8080' : ''
};
