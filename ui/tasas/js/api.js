import { apiUrl } from '../../shared/js/api-base.js';

const BASE = '/api/v1/tasas';
const HOO_RATES_URL = 'https://hoo.jairokov.com/global-rates';
const WRITE_METHOD = 'PUT';

export class ApiError extends Error {
  constructor(mensaje, estado = 0, ruta = '') {
    super(mensaje);
    this.name = 'ApiError';
    this.estado = estado;
    this.ruta = ruta;
  }
}

function extracto(texto) {
  return String(texto || '')
    .replace(/<style[\s\S]*?<\/style>/gi, ' ')
    .replace(/<script[\s\S]*?<\/script>/gi, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 160);
}

function mensajeHttp(estado, texto = '') {
  if (estado === 404) return 'Endpoint no disponible en Hooty v2';
  if (estado === 405) return 'Método no permitido por Hooty v2';
  const detalle = extracto(texto);
  if (estado >= 500) {
    return detalle
      ? `Hooty respondió HTTP ${estado}: ${detalle}`
      : `Hooty respondió HTTP ${estado} sin detalle (posible timeout del servidor o del proxy)`;
  }
  return detalle ? `Error HTTP ${estado}: ${detalle}` : `Error HTTP ${estado}`;
}

async function solicitarJson(ruta, opciones = {}) {
  const init = {
    cache: 'no-store',
    ...opciones,
    headers: { Accept: 'application/json', ...(opciones.headers || {}) }
  };
  const tieneCuerpo = Boolean(init.body);
  const tieneTipo = Object.keys(init.headers).some((clave) => clave.toLowerCase() === 'content-type');
  if (tieneCuerpo && !tieneTipo) {
    init.headers['Content-Type'] = 'application/json';
  }

  let respuesta;
  try {
    const destino = ruta.startsWith('/api/v1/') ? ruta : `${BASE}${ruta}`;
    respuesta = await fetch(apiUrl(destino), init);
  } catch (error) {
    throw new ApiError('No se pudo conectar con Hooty', 0, ruta);
  }

  const texto = await respuesta.text();
  let datos = {};
  if (texto) {
    try {
      datos = JSON.parse(texto);
    } catch (error) {
      datos = {};
    }
  }

  if (!respuesta.ok || datos.success === false) {
    const detalle = typeof datos.error === 'string' ? datos.error.trim() : '';
    const mensaje = detalle
      ? (respuesta.status >= 400 ? `HTTP ${respuesta.status}: ${detalle}` : detalle)
      : mensajeHttp(respuesta.status, texto || respuesta.statusText);
    throw new ApiError(mensaje, respuesta.status, ruta);
  }
  return datos;
}

const PUBLICACION_INTERVALO_MS = 2000;
const PUBLICACION_ESPERA_MAX_MS = 10 * 60 * 1000;

async function esperarPublicacion(jobId) {
  const limite = Date.now() + PUBLICACION_ESPERA_MAX_MS;
  while (Date.now() < limite) {
    await new Promise((resolver) => setTimeout(resolver, PUBLICACION_INTERVALO_MS));
    const estado = await solicitarJson(`/publicar-lote/estado/${encodeURIComponent(jobId)}`);
    if (estado.estado === 'error') {
      throw new ApiError(estado.error || 'La publicación falló', 502, '/publicar-lote');
    }
    if (estado.estado !== 'en_proceso') return estado;
  }
  throw new ApiError(
    'La publicación sigue en curso; revisa el estado antes de reintentar',
    0,
    '/publicar-lote'
  );
}

export const api = {
  /** Lectura canónica del lote vigente y sus tasas. */
  monitor() {
    return solicitarJson('/monitor');
  },

  perfiles() {
    return solicitarJson('/api/v1/perfiles');
  },

  /**
   * Borrador HOO del monitor en segundo plano (se refresca cada 2 minutos).
   * El origen devuelve un array y claves en minúsculas; se normalizan a
   * mayúsculas para el estado de "NUEVO LOTE".
   */
  async borradorHoo() {
    let respuesta;
    try {
      respuesta = await fetch(HOO_RATES_URL, {
        headers: { Accept: 'application/json' },
        cache: 'no-store'
      });
    } catch (error) {
      throw new ApiError('No se pudo conectar con HOO', 0, HOO_RATES_URL);
    }
    if (!respuesta.ok) {
      throw new ApiError(`HOO respondió HTTP ${respuesta.status}`, respuesta.status, HOO_RATES_URL);
    }
    const data = await respuesta.json().catch(() => null);
    const tasas = Array.isArray(data) ? data[0] : data;
    const borrador = {};
    if (tasas && typeof tasas === 'object') {
      for (const [clave, valor] of Object.entries(tasas)) {
        const code = String(clave).trim().toUpperCase();
        if (!/^[A-Z]{3,5}$/.test(code)) continue;
        const numero = typeof valor === 'number' ? valor : Number(String(valor).replace(',', '.'));
        if (Number.isFinite(numero) && numero >= 0) borrador[code] = numero;
      }
    }
    return { id_tasa: '', borrador };
  },

  guardarLote(tasas) {
    return solicitarJson('/guardar-lote', {
      method: WRITE_METHOD,
      body: JSON.stringify({ tasas })
    });
  },

  async publicarLote(modoPruebaGlobal) {
    const inicio = await solicitarJson('/publicar-lote', {
      method: WRITE_METHOD,
      body: JSON.stringify({
        is_prueba: Boolean(modoPruebaGlobal),
        modo_prueba_global: Boolean(modoPruebaGlobal)
      })
    });
    if (inicio.estado !== 'en_proceso' || !inicio.job_id) return inicio;
    return esperarPublicacion(inicio.job_id);
  },

  estadoPublicacion(jobId) {
    return solicitarJson(`/publicar-lote/estado/${encodeURIComponent(jobId)}`);
  },

  async previewBlob(socio = '', prueba = false) {
    const params = new URLSearchParams();
    if (socio) params.set('socio', socio);
    if (prueba) params.set('prueba', '1');
    const ruta = socio ? `/preview/${encodeURIComponent(socio)}` : '/preview';
    const consulta = params.toString();
    const respuesta = await fetch(apiUrl(`${BASE}${ruta}${consulta ? `?${consulta}` : ''}`), {
      headers: { Accept: 'image/png' },
      cache: 'no-store'
    });
    if (!respuesta.ok) {
      throw new ApiError('No se pudo generar la vista previa', respuesta.status, ruta);
    }
    return respuesta.blob();
  },

  notificarRemesas() {
    return solicitarJson('/notificar-remesas', { method: 'POST' });
  },

  eliminarLote(idTasa) {
    return solicitarJson(`/lote/${encodeURIComponent(idTasa)}`, { method: 'DELETE' });
  }
};
