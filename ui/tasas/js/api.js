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

function mensajeHttp(estado) {
  if (estado === 404) return 'Endpoint no disponible en Hooty v2';
  if (estado === 405) return 'Método no permitido por Hooty v2';
  if (estado >= 500) return 'Hooty no pudo leer la base de datos';
  return `Error HTTP ${estado}`;
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
    respuesta = await fetch(apiUrl(`${BASE}${ruta}`), init);
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
    throw new ApiError(datos.error || mensajeHttp(respuesta.status), respuesta.status, ruta);
  }
  return datos;
}

export const api = {
  /** Lectura canónica del lote vigente y sus tasas. */
  monitor() {
    return solicitarJson('/monitor');
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

  publicarLote(modoPruebaGlobal) {
    return solicitarJson('/publicar-lote', {
      method: WRITE_METHOD,
      body: JSON.stringify({ modo_prueba_global: modoPruebaGlobal })
    });
  },

  notificarRemesas() {
    return solicitarJson('/notificar-remesas', { method: 'POST' });
  },

  eliminarLote(idTasa) {
    return solicitarJson(`/lote/${encodeURIComponent(idTasa)}`, { method: 'DELETE' });
  }
};
