/**
 * @file comprobantesCargaService.js
 * @description Orquestador de carga: red, mapeo canónico y presentación
 * (orden y rango de hash). El backend es la fuente del universo, del total y
 * del filtro de socio.
 */

import {
  fetchComprobantesHasta,
  fetchComprobantesPagina
} from './comprobantesApiService.js?v=20261031_06';
import { prepararEdicionComprobante } from './comprobantesMapperService.js?v=20261101_02';
import {
  aplicarRangoHash,
  ordenarComprobantes
} from './comprobantesFilterService.js?v=20261031_02';

/**
 * Carga una página real de comprobantes para la vista de auditoría.
 * El backend ordena, filtra y pagina; el mapper solo adapta la fila a la vista.
 */
export async function cargarPaginaComprobantes(params = {}, opciones = {}) {
  const limit = Math.max(1, Number(opciones.limit) || 20);
  const offset = Math.max(0, Number(opciones.offset) || 0);
  const loteActivo = opciones.loteActivo || 'T070';

  const pagina = await fetchComprobantesPagina(params, { limit, offset });
  const items = pagina.lista.map(item => prepararEdicionComprobante(item, loteActivo));

  return {
    items,
    total: pagina.total,
    hasMore: pagina.hasMore,
    paginado: pagina.paginado,
    offset: pagina.offset,
    recibidos: pagina.recibidos
  };
}

/**
 * Carga un bloque amplio para el reporte o para una vista que necesita el
 * universo acotado. El filtro de socio viaja al backend; el orden y el rango de
 * hash se resuelven como presentación.
 */
export async function cargarComprobantesFiltrados(params = {}, opciones = {}) {
  const maximo = Math.max(1, Number(opciones.maximo) || 500);
  const orden = params.orden || 'fecha_desc';
  const { desdeHash = '', hastaHash = '' } = params;

  const raw = await fetchComprobantesHasta(params, maximo);
  // El reporte es una copia fiel del payload de /api/v1/comprobantes: no pasa
  // por el mapper de cards para no alterar tipo_op, tasas ni importes.
  const lista = Array.isArray(raw) ? raw : [];
  const ordenados = ordenarComprobantes(lista, orden);
  const acotados = aplicarRangoHash(ordenados, desdeHash, hastaHash);
  return acotados.slice(0, maximo);
}
