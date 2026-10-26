import { get, post, idempotencyKey } from '../../shared/js/http.js';

export const filtros = (rol = '') =>
  get('/api/v1/reportes/filtros', { params: { rol } }).then((data) => data?.data || data || {});

function numeroCabecera(headers, nombre) {
  const valor = headers?.get?.(nombre);
  if (valor === null || valor === undefined || valor === '') return null;
  const numero = Number(valor);
  return Number.isFinite(numero) ? numero : null;
}

/**
 * Devuelve las filas y los metadatos de paginación del backend.
 * El contrato raíz sigue siendo un array; total/limit/offset/hasMore
 * viajan en cabeceras X-Total-Count, X-Limit, X-Offset y X-Has-More.
 */
export async function movimientos(params = {}) {
  const pageSize = 100;
  const maxRows = 5000;
  const rows = [];
  const seen = new Set();
  let offset = 0;
  let total = 0;
  let hasMore = true;

  while (hasMore && rows.length < maxRows) {
    const respuesta = await get('/api/v1/comprobantes', {
      params: { ...params, limit: pageSize, offset },
      includeResponse: true
    });

    const payload = respuesta?.data;
    const pageRows = Array.isArray(payload)
      ? payload
      : (Array.isArray(payload?.data) ? payload.data : []);
    const headers = respuesta?.headers;
    const totalCabecera = numeroCabecera(headers, 'X-Total-Count');
    const hasMoreCabecera = String(headers?.get?.('X-Has-More') || '').toLowerCase() === 'true';

    let added = 0;
    for (const row of pageRows) {
      const key = String(row?.hash_largo || row?.hash_corto || `${offset}-${added}`).trim();
      if (seen.has(key)) continue;
      seen.add(key);
      rows.push(row);
      added += 1;
      if (rows.length >= maxRows) break;
    }

    total = totalCabecera ?? (offset + pageRows.length);
    hasMore = hasMoreCabecera || pageRows.length >= pageSize;
    offset += pageRows.length;
    if (pageRows.length === 0 || added === 0) break;
  }

  return {
    rows,
    total: Math.max(total, rows.length),
    limit: pageSize,
    offset: 0,
    hasMore: false
  };
}

export const socios = () => get('/api/v1/perfiles');

export function enviarImagen(payload, key = idempotencyKey()) {
  return post('/api/v1/reportes/imagen', payload, { idempotencyKey: key, timeout: 30000 })
    .then((data) => ({ ...data, _idempotencyKey: key }));
}

export const enviarTexto = (payload, key = idempotencyKey()) =>
  post('/api/v1/reportes/texto', payload, { idempotencyKey: key, timeout: 30000 });

export const api = { filtros, movimientos, socios, enviarImagen, enviarTexto };
