import { get, post, patch, del } from '../../shared/js/http.js';
import { apiUrl } from '../../shared/js/api-base.js';

const BASE = '/api/v1/perfiles';

// GET /api/v1/perfiles responde un array raíz, no un envelope {success,data}.
export const listar = () => get(BASE).then((datos) => (Array.isArray(datos) ? datos : []));

// POST /api/v1/perfiles/guardar responde {success,data}; el panel recarga el
// directorio y no consume el perfil devuelto.
export const guardar = (payload) => post(`${BASE}/guardar`, payload);

export const toggle = (nombre, activo) =>
  patch(`${BASE}/${encodeURIComponent(nombre)}/estado`, { activo });

export const eliminar = (nombre) => del(`${BASE}/${encodeURIComponent(nombre)}`);

// Lote activo de tasas: alimenta la píldora "base" y el cálculo en vivo del
// spread en las tarjetas de moneda del modal.
export const tasasUltimas = () => get('/api/v1/tasas/monitor');

export async function previewBlob(nombre) {
  const respuesta = await fetch(apiUrl(`/api/v1/tasas/preview/${encodeURIComponent(nombre)}`), {
    headers: { Accept: 'image/png' },
    cache: 'no-store'
  });
  if (!respuesta.ok) throw new Error(`No se pudo generar la cartelera (HTTP ${respuesta.status})`);
  return respuesta.blob();
}

export const apagarTodos = () => post(`${BASE}/apagar-todos`);
export const memorizar = () => post(`${BASE}/plantilla/memorizar`);
export const restaurar = () => post(`${BASE}/plantilla/restaurar`);

export const api = {
  listar, guardar, toggle, eliminar, apagarTodos, memorizar, restaurar, tasasUltimas, previewBlob
};
