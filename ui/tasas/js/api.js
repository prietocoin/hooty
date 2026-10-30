import { apiUrl } from '../../shared/js/api-base.js';

const BASE = '/api/v1/tasas';

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
  const init = { ...opciones, headers: { Accept: 'application/json', ...(opciones.headers || {}) } };
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
   * Es distinto del lote vigente de /monitor: solo rellena "NUEVO LOTE".
   */
  async borradorHoo() {
    const datos = await solicitarJson('/hoo-borrador');
    return {
      id_tasa: datos.id_tasa || datos.ID || '',
      borrador: datos.borrador ?? datos.tasas ?? datos.rates ?? datos.rates_draft ?? {}
    };
  },

  guardarLote(tasas) {
    return solicitarJson('/guardar-lote', {
      method: 'POST',
      body: JSON.stringify({ tasas })
    });
  },

  publicarLote(modoPruebaGlobal) {
    return solicitarJson('/publicar-lote', {
      method: 'POST',
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
