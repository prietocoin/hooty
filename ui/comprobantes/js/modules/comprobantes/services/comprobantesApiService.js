/**
 * @file comprobantesApiService.js
 * @description Servicio atómico de red para el CRUD de comprobantes contra el
 * backend Go unificado. Mantiene la paginación y los filtros de la interfaz
 * glauk2 sobre el contrato /api/v1/comprobantes.
 */

import { apiUrl } from '../../../../../shared/js/api-base.js?v=20261031_02';
import { aplicarRangoHash } from './comprobantesFilterService.js?v=20261031_02';

const API_RUTA = '/api/v1/comprobantes';

function obtenerIdentificadorValido(identificador, fallbackPayload = {}) {
  let target = '';

  if (typeof identificador === 'object' && identificador !== null) {
    target = identificador.hash_largo ||
             identificador.id ||
             identificador._id ||
             identificador.hash ||
             identificador.referencia || '';
  } else {
    target = identificador;
  }

  if (!target) {
    target = fallbackPayload?.hash_largo ||
             fallbackPayload?.hash ||
             fallbackPayload?.id ||
             fallbackPayload?._id ||
             fallbackPayload?.referencia || '';
  }

  return String(target || '').trim();
}

function normalizarFecha(valor) {
  const fecha = new Date(valor || 0);
  return isNaN(fecha.getTime()) ? 0 : fecha.getTime();
}

function ordenarLista(lista, orden) {
  const copia = [...lista];
  switch (orden) {
    case 'fecha_asc':
      return copia.sort((a, b) => normalizarFecha(a.fecha_operacion || a.fecha_hora_comprobante || a.timestamp_msg) - normalizarFecha(b.fecha_operacion || b.fecha_hora_comprobante || b.timestamp_msg));
    case 'monto_asc':
      return copia.sort((a, b) => (parseFloat(a.monto_recibo ?? a.monto) || 0) - (parseFloat(b.monto_recibo ?? b.monto) || 0));
    case 'monto_desc':
      return copia.sort((a, b) => (parseFloat(b.monto_recibo ?? b.monto) || 0) - (parseFloat(a.monto_recibo ?? a.monto) || 0));
    case 'fecha_desc':
    default:
      return copia.sort((a, b) => normalizarFecha(b.fecha_operacion || b.fecha_hora_comprobante || b.timestamp_msg) - normalizarFecha(a.fecha_operacion || a.fecha_hora_comprobante || a.timestamp_msg));
  }
}

const LIMITE_API = 100;
const MAXIMO_CONSULTA = 500;
// Tope interno para recuperar datos cuando el filtro de socio del backend
// devuelve cero aunque existan filas. La UI sigue mostrando como máximo 500.
export const MAXIMO_RECUPERACION = 2000;
// Bloque de carga progresiva por red que consume la vista de comprobantes.
export const LIMITE_PAGINA = 20;

function claveItem(item) {
  const c = item?.comprobante || item || {};
  const principal = String(c.hash_largo || c.id || c.referencia || '').trim().toLowerCase();
  if (principal) return principal;
  return `${c.fecha_operacion || c.fecha_hora_comprobante || c.fecha || ''}|${c.monto_recibo ?? c.monto ?? ''}|${c.banco || ''}`;
}

async function consultarLista(params = {}) {
  const url = construirUrlConsulta(params, { limit: LIMITE_API, offset: 0 });

  const res = await fetch(url.toString());
  const contentType = res.headers.get('content-type');
  if (!contentType || !contentType.includes('application/json')) {
    console.warn(`[comprobantesApiService ⚠️ Warning]: La API respondió con formato no JSON (${res.status}).`);
    return { lista: [], truncada: false };
  }
  if (!res.ok) throw new Error(`HTTP Error: ${res.status}`);

  const json = await res.json();
  const lista = json?.comprobantes || json?.data || (Array.isArray(json) ? json : []);
  const normalizada = Array.isArray(lista) ? lista : [];
  return { lista: normalizada, truncada: normalizada.length >= LIMITE_API };
}

/**
 * Construye la URL de consulta compartida por la carga legacy y la paginada.
 * Los límites de hash son un recorte posicional local y nunca viajan a la API.
 */
function construirUrlConsulta(params = {}, paginacion = {}) {
  const {
    socio = '',
    rol = '',
    fechaInicio = '',
    fechaFin = '',
    orden = 'fecha_desc',
    hash = ''
  } = params;

  const url = new URL(apiUrl(API_RUTA), window.location.origin);
  const socioNormalizado = String(socio || '').trim().toUpperCase();
  if (socioNormalizado && socioNormalizado !== 'TODOS' && socioNormalizado !== 'TODOS LOS SOCIOS' && socioNormalizado !== 'GENERAL') {
    url.searchParams.append('socio', String(socio).trim());
  }
  if (rol && rol.toUpperCase() !== 'TODOS') url.searchParams.append('rol', rol.trim());
  if (fechaInicio) url.searchParams.append('fechaInicio', fechaInicio.trim());
  if (fechaFin) url.searchParams.append('fechaFin', fechaFin.trim());
  if (hash) url.searchParams.append('hash', hash.trim());
  if (orden === 'fecha_asc' || orden === 'fecha_desc') url.searchParams.append('orden', orden);
  if (paginacion.limit != null) url.searchParams.append('limit', String(paginacion.limit));
  if (paginacion.offset != null) url.searchParams.append('offset', String(paginacion.offset));

  return url;
}

function leerEnteroCabecera(res, nombre) {
  const valor = res.headers.get(nombre);
  if (valor === null || valor === '') return null;
  const numero = Number.parseInt(valor, 10);
  return Number.isFinite(numero) ? numero : null;
}

/**
 * Solicita una página real al backend (limit/offset) y devuelve los metadatos
 * de paginación necesarios para el scroll progresivo.
 *
 * El contrato canónico mantiene un array raíz y expone el universo filtrado en
 * las cabeceras X-Total-Count / X-Has-More. Si el backend aún no las envía y
 * devuelve más filas que la ventana pedida, se marca `paginado: false` para
 * que la UI degrade a paginación local sobre el bloque recibido.
 */
export async function fetchComprobantesPagina(params = {}, opciones = {}) {
  const limit = Math.min(
    Math.max(Number(opciones.limit) || LIMITE_PAGINA, 1),
    LIMITE_API
  );
  const offset = Math.max(Number(opciones.offset) || 0, 0);

  try {
    const res = await fetch(construirUrlConsulta(params, { limit, offset }).toString());
    const contentType = res.headers.get('content-type');
    if (!contentType || !contentType.includes('application/json')) {
      console.warn(`[comprobantesApiService ⚠️ Warning]: La API respondió con formato no JSON (${res.status}).`);
      return { lista: [], total: offset, hasMore: false, paginado: true, limit, offset, recibidos: 0 };
    }
    if (!res.ok) throw new Error(`HTTP Error: ${res.status}`);

    const json = await res.json();
    const bruta = Array.isArray(json)
      ? json
      : (json?.comprobantes || json?.items || json?.data || []);
    const lista = Array.isArray(bruta) ? bruta : [];

    const totalCabecera = leerEnteroCabecera(res, 'x-total-count');
    const hasMoreCabecera = res.headers.get('x-has-more');
    const tieneMetadatos = totalCabecera !== null || hasMoreCabecera !== null;
    // Un backend sin paginación devuelve su tope físico (100) aunque se pidan 20.
    const respetoVentana = tieneMetadatos || lista.length <= limit;

    const total = totalCabecera !== null
      ? Math.max(totalCabecera, offset + lista.length)
      : offset + lista.length;
    const hasMore = hasMoreCabecera !== null
      ? hasMoreCabecera === 'true'
      : (respetoVentana && lista.length >= limit);

    return {
      lista,
      total,
      hasMore,
      paginado: respetoVentana,
      limit,
      offset,
      recibidos: lista.length
    };
  } catch (err) {
    console.error('[comprobantesApiService ❌ Fetch Error]:', err.message);
    return { lista: [], total: offset, hasMore: false, paginado: true, limit, offset, recibidos: 0 };
  }
}

/**
 * Recupera una sola fila canónica por hash. Se usa después del PUT para
 * actualizar la UI con la entidad que proyecta el backend, no con el payload
 * local del formulario.
 */
export async function fetchComprobantePorHash(hashLargo) {
  const target = String(hashLargo || '').trim();
  if (!target) return null;

  const res = await fetch(apiUrl(`${API_RUTA}/${encodeURIComponent(target)}`), {
    headers: { Accept: 'application/json' }
  });
  if (res.status === 404) return null;
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || data.message || `HTTP Error: ${res.status}`);
  }
  const data = await res.json().catch(() => null);
  return data && typeof data === 'object' ? data : null;
}

/**
 * Reúne hasta el tope solicitado respetando el límite físico de 100 filas por
 * consulta del backend. Si un bloque llega lleno, parte el rango de fechas y
 * consulta los dos subrangos. El máximo de recuperación es 2000; la UI
 * normal sigue solicitando 500.
 */
export async function fetchComprobantesHasta(params = {}, maximo = MAXIMO_CONSULTA) {
  const tope = Math.min(Math.max(Number(maximo) || MAXIMO_CONSULTA, 1), MAXIMO_RECUPERACION);
  const acumulado = [];
  const vistos = new Set();
  let offset = 0;

  try {
    while (acumulado.length < tope) {
      const limit = Math.min(LIMITE_API, tope - acumulado.length);
      const pagina = await fetchComprobantesPagina(params, { limit, offset });
      if (!pagina.lista.length) break;

      let agregados = 0;
      for (const item of pagina.lista) {
        if (acumulado.length >= tope) break;
        const clave = claveItem(item);
        if (vistos.has(clave)) continue;
        vistos.add(clave);
        acumulado.push(item);
        agregados += 1;
      }

      offset += pagina.lista.length;
      if (!pagina.hasMore || agregados === 0) break;
    }
    return ordenarLista(acumulado, params.orden || 'fecha_desc');
  } catch (err) {
    console.error('[comprobantesApiService ❌ Fetch Error]:', err.message);
    return [];
  }
}

/**
 * Carga los comprobantes y emula la paginación de la API v2 sobre el contrato
 * canónico. Se conserva para consumidores que necesitan una página puntual.
 */
export async function fetchComprobantesPaginados(params = {}) {
  try {
    const { page = 1, limit = 20, desdeHash = '', hastaHash = '', orden = 'fecha_desc' } = params;
    const respuesta = await consultarLista(params);
    const ordenada = ordenarLista(respuesta.lista, orden);
    const filtrada = aplicarRangoHash(ordenada, desdeHash, hastaHash);
    const inicio = Math.max(0, (Number(page) - 1) * Number(limit));
    return filtrada.slice(inicio, inicio + Number(limit));
  } catch (err) {
    console.error('[comprobantesApiService ❌ Fetch Error]:', err.message);
    return [];
  }
}

/**
 * Guarda la edición completa de un comprobante: cabecera + ambos socios.
 * Envía solo el contrato editable de PUT /api/v1/comprobantes/{hash}; el
 * backend conserva hash, grupos, lote, fecha de operación y URL de imagen.
 */
export async function guardarCambiosComprobante(hashLargo, payload = {}) {
  const idValido = obtenerIdentificadorValido(hashLargo, payload);
  if (!idValido) {
    throw new Error('No se proporcionó un hash válido para actualizar');
  }

  const numero = (valor) => {
    const n = Number(valor);
    return Number.isFinite(n) ? n : 0;
  };
  const texto = (valor) => String(valor ?? '').trim();
  const socio1 = texto(payload.socio_1 ?? payload.nombre_socio_1);
  const socio2 = texto(payload.socio_2 ?? payload.nombre_socio_2);

  const body = {
    banco: texto(payload.banco),
    referencia: texto(payload.referencia),
    titular: texto(payload.titular),
    monto_recibo: numero(payload.monto_recibo ?? payload.monto),
    moneda_recibo: texto(payload.moneda_recibo ?? payload.moneda ?? 'USDT').toUpperCase(),
    socio_1: socio1,
    monto_1: numero(payload.monto_1),
    tasa_1: numero(payload.tasa_1),
    socio_2: socio2,
    monto_2: numero(payload.monto_2),
    tasa_2: numero(payload.tasa_2)
  };

  // tipo_op es persistente y opcional: solo se reenvía si el consumidor lo
  // entrega con una letra válida, para no pisar el valor guardado.
  const tipoOp = texto(payload.tipo_op).toUpperCase().split('-')[0].trim();
  if (tipoOp === 'D' || tipoOp === 'P' || tipoOp === 'A') {
    body.tipo_op = tipoOp;
  }

  // ME es estadístico, no editable. Solo se reenvía si un consumidor lo
  // proporciona explícitamente con un valor finito; si se omite, el backend
  // conserva el me1/me2 originales de la fila (ComprobantePatch usa punteros).
  for (const clave of ['me1', 'me2']) {
    const valor = payload[clave];
    if (valor === undefined || valor === null || valor === '') continue;
    const numerico = Number(valor);
    if (Number.isFinite(numerico) && numerico !== 0) body[clave] = numerico;
  }

  const res = await fetch(apiUrl(`${API_RUTA}/${encodeURIComponent(idValido)}`), {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json'
    },
    body: JSON.stringify(body)
  });

  const raw = await res.text();
  let data = {};
  if (raw) {
    try {
      data = JSON.parse(raw);
    } catch (error) {
      data = {};
    }
  }
  if (!res.ok || data?.success === false) {
    console.error('[Hooty] Error en PUT:', res.status, raw);
    throw new Error(data?.error || data?.message || raw || `HTTP Error: ${res.status}`);
  }
  return data;
}

/**
 * Solicita relectura con IA.
 */
export async function solicitarRelecturaIA(hashLargo) {
  try {
    const idValido = obtenerIdentificadorValido(hashLargo);
    if (!idValido) {
      throw new Error('No se proporcionó un hash o ID válido para releer con IA');
    }

    const res = await fetch(apiUrl(`${API_RUTA}/${encodeURIComponent(idValido)}/releer`), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });

    if (!res.ok) throw new Error(`HTTP Error: ${res.status}`);
    return await res.json().catch(() => true);
  } catch (err) {
    console.error('[comprobantesApiService ❌ IA Re-read Error]:', err.message);
    throw err;
  }
}

/**
 * Elimina un comprobante por su hash o ID.
 */
export async function eliminarComprobantePorHash(hashLargo) {
  try {
    const idValido = obtenerIdentificadorValido(hashLargo);
    if (!idValido) {
      throw new Error('No se proporcionó un hash o ID válido para eliminar');
    }

    const res = await fetch(apiUrl(`${API_RUTA}/${encodeURIComponent(idValido)}`), {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' }
    });

    const data = await res.json().catch(() => ({}));
    if (!res.ok || data?.success === false) {
      throw new Error(data?.error || data?.message || `HTTP Error: ${res.status}`);
    }
    return data || true;
  } catch (err) {
    console.error('[comprobantesApiService ❌ Delete Error]:', err.message);
    throw err;
  }
}

export {
  guardarCambiosComprobante as actualizarComprobanteApi,
  solicitarRelecturaIA as releerIaApi
};
