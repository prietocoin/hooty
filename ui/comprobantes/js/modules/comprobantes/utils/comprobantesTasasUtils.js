/**
 * @file comprobantesTasasUtils.js
 * @description Lectura de las tasas, el tipo y el subtotal que ya proyectó el
 * backend. No aplica spreads, no redondea y no reconstruye la polaridad.
 */

import { aNumero as numero } from './comprobantesNumerosUtils.js?v=20261031_02';

/**
 * La tasa del backend ya está truncada por el snapshot. Solo se valida que sea
 * numérica; no se redondea ni se sustituye por 1.
 */
export function normalizarTasaComercial(valor) {
  const num = numero(valor, 0);
  return Number.isFinite(num) && num > 0 ? num : 0;
}

/**
 * Regla estricta de presentación de tasas:
 *   > 99.99        -> entero truncado
 *   1.00 - 99.99   -> 2 decimales truncados
 *   < 1.00         -> 3 cifras significativas truncadas
 * Nunca redondea.
 */
export function formatearTasaRegla(valor) {
  const num = Number(valor);
  if (!num || isNaN(num) || num <= 0) return '';

  if (num > 99.99) {
    return Math.trunc(num).toString();
  }

  if (num >= 1) {
    const factor = 100;
    return (Math.trunc(num * factor) / factor).toFixed(2);
  }

  const str = num.toString();
  const match = str.match(/^0\.0*/);
  if (!match) return num.toString();
  const ceros = match[0].length - 2;
  const factor = Math.pow(10, ceros + 3);
  return (Math.trunc(num * factor) / factor).toString();
}

/**
 * Formatea la tasa en formato decimal plano, sin comas de miles.
 * Es presentación: el valor viene del backend.
 */
export function formatearTasaComercial(valor) {
  const num = numero(valor, NaN);
  return Number.isFinite(num) && num > 0 ? formatearTasaRegla(num) : '0.00';
}

/**
 * Tasa del lado del socio tal como la proyecta el backend.
 */
export function resolverTasaSocio(item = {}, posicion = 1) {
  const esSocio2 = Number(posicion) === 2;
  const bruta = esSocio2
    ? (item?.tasa_2 ?? item?.tasa2)
    : (item?.tasa_1 ?? item?.tasa1);
  return normalizarTasaComercial(bruta);
}

/**
 * Divisa con la que se cotiza la tasa: la del comprobante.
 */
export function resolverMonedaTasa(item = {}) {
  return String(item?.moneda_recibo || item?.moneda || item?.moneda_local || '').trim().toUpperCase();
}

/**
 * Tipo de operación persistido en `comprobantes.tipo_op`. Es la fuente única
 * de verdad para cards y reporte: no se reconstruye desde el perfil ni desde
 * el caption. `tipo` y `tipo_op1/2` quedan solo como respaldo de filas legadas.
 */
export function resolverTipoComprobante(item = {}) {
  const normalizar = (valor) => {
    const letra = String(valor || '').trim().split('-')[0].toUpperCase();
    return letra === 'A' || letra === 'P' || letra === 'D' ? letra : '';
  };

  return normalizar(item?.tipo_op)
    || normalizar(item?.tipo)
    || normalizar(item?.tipo_manual)
    || normalizar(item?.tipo_op1)
    || normalizar(item?.tipo_op2)
    || 'D';
}

/**
 * Moneda base del socio (la del subtotal).
 */
export function resolverMonedaSocio(item = {}, posicion = 1) {
  const esSocio2 = Number(posicion) === 2;
  const moneda = esSocio2
    ? (item?.moneda_socio_2 || item?.moneda_base_socio2 || item?.moneda_recibo || item?.moneda)
    : (item?.moneda_socio_1 || item?.moneda_base_socio1 || item?.moneda_recibo || item?.moneda);
  return String(moneda || '').trim().toUpperCase();
}

/**
 * Subtotal firmado del socio en su moneda local (PEN, COP, etc.), copiado del
 * snapshot del backend. No se usa `me1`/`me2`: esos campos son el equivalente
 * en dólares y no deben etiquetarse con la moneda del socio.
 */
export function resolverSubtotalSocio(item = {}, posicion = 1) {
  const esSocio2 = Number(posicion) === 2;
  const bruto = esSocio2
    ? (item?.monto_socio_2 ?? item?.monto_2 ?? item?.monto2 ?? item?.me2 ?? item?.m2_socio)
    : (item?.monto_socio_1 ?? item?.monto_1 ?? item?.monto1 ?? item?.me1 ?? item?.m1_socio);
  return numero(bruto, 0);
}
