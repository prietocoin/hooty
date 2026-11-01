/**
 * @file comprobantesTasasUtils.js
 * @description Lectura de las tasas, el tipo y el subtotal que ya proyectó el
 * backend. No aplica spreads, no redondea y no reconstruye la polaridad.
 */

import { aNumero as numero, formatearTasaPlana } from './comprobantesNumerosUtils.js?v=20261031_02';

/**
 * La tasa del backend ya está truncada por el snapshot. Solo se valida que sea
 * numérica; no se redondea ni se sustituye por 1.
 */
export function normalizarTasaComercial(valor) {
  const num = numero(valor, 0);
  return Number.isFinite(num) && num > 0 ? num : 0;
}

/**
 * Formatea la tasa en formato decimal plano, sin comas de miles.
 * Es presentación: el valor viene del backend.
 */
export function formatearTasaComercial(valor) {
  return formatearTasaPlana(numero(valor, NaN));
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
 * Subtotal firmado del socio, copiado directamente del snapshot del backend.
 * No se multiplica por `polaridad_1/2`: el signo ya está en `me1`/`me2`.
 */
export function resolverSubtotalSocio(item = {}, posicion = 1) {
  const esSocio2 = Number(posicion) === 2;
  const bruto = esSocio2
    ? (item?.me2 ?? item?.m2_socio ?? item?.monto_2 ?? item?.monto2)
    : (item?.me1 ?? item?.m1_socio ?? item?.monto_1 ?? item?.monto1);
  return numero(bruto, 0);
}
