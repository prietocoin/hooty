/**
 * @file directorioService.js
 * @description Servicio de consulta y normalización del directorio de socios.
 * Mapea la estructura JSONB 'mostrar' proveniente de PostgreSQL hacia las banderas
 * 'activo' y 'mostrar_dashboard' para la UI.
 */

import { apiUrl } from '../../../../../shared/js/api-base.js';

/**
 * Obtiene y normaliza la lista completa de socios del directorio.
 * @returns {Promise<Array>}
 */
export async function obtenerDirectorioNormalizado() {
  try {
    const response = await fetch(apiUrl('/api/v1/perfiles'));
    if (!response.ok) throw new Error('Error al consultar el directorio');

    const rawList = await response.json();
    if (!Array.isArray(rawList)) return [];

    return rawList.map(s => {
      let mostrarObj = s.mostrar;
      if (typeof mostrarObj === 'string') {
        try { mostrarObj = JSON.parse(mostrarObj); } catch (e) { mostrarObj = {}; }
      }
      mostrarObj = (typeof mostrarObj === 'object' && mostrarObj !== null) ? mostrarObj : {};

      const estaActivo = mostrarObj.tasas ?? s.activo ?? true;

      return {
        ...s,
        mostrar: mostrarObj,
        activo: Boolean(estaActivo),
        mostrar_dashboard: Boolean(mostrarObj.dashboard ?? s.mostrar_dashboard ?? true)
      };
    });
  } catch (err) {
    console.error('[directorioService ❌ Error al cargar directorio]:', err);
    return [];
  }
}

/**
 * Persiste únicamente el saldo base/anterior del socio. Usa el PATCH dedicado
 * para no sobrescribir monedas, visibilidad ni herencia del perfil.
 * @param {string} nombreSocio
 * @param {number|string} saldo
 * @returns {Promise<object>}
 */
export async function actualizarSaldoAnteriorSocio(nombreSocio, saldo) {
  const nombre = String(nombreSocio || '').trim();
  if (!nombre) throw new Error('Selecciona un socio para editar su saldo anterior.');

  const valor = Number(saldo);
  if (!Number.isFinite(valor)) throw new Error('El saldo anterior debe ser un número válido.');

  const response = await fetch(apiUrl(`/api/v1/perfiles/${encodeURIComponent(nombre)}/saldo`), {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json'
    },
    body: JSON.stringify({ saldo_inicial: valor })
  });

  const data = await response.json().catch(() => ({}));
  if (!response.ok || data?.success === false) {
    throw new Error(data?.error || data?.message || `HTTP ${response.status}`);
  }
  return data;
}
