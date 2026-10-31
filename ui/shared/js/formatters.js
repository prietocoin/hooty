/**
 * @file formatters.js
 * @description Regla universal de presentación de tasas y montos contables.
 *
 * Tasas y montos: siempre dos decimales truncados hacia cero, sin redondear y
 * sin eliminar los ceros ("1652.40").
 *
 * Prohibido redondear: no se usa Math.round en ningún punto.
 */

/**
 * Trunca hacia cero con desplazamiento decimal por cadena, evitando el error
 * binario de `4.29 * 100 = 428.99999999999994`.
 */
export function truncarDecimales(valor, decimales = 0) {
  const num = Number(valor);
  if (!Number.isFinite(num)) return 0;

  const lugares = Math.max(0, Math.trunc(decimales));
  const signo = num < 0 ? -1 : 1;
  const abs = Math.abs(num);
  const desplazado = Number(`${abs}e${lugares}`);

  if (!Number.isFinite(desplazado)) {
    const factor = Math.pow(10, lugares);
    return signo * (Math.trunc(abs * factor) / factor);
  }

  return signo * (Math.trunc(desplazado) / Math.pow(10, lugares));
}

/** Formato universal de tasa: 2 decimales truncados, siempre visibles. */
export function formatTasa(valor) {
  const num = Number(valor);
  if (!Number.isFinite(num)) return '0.00';
  return truncarDecimales(num, 2).toFixed(2);
}

/** Monto contable plano: dos decimales truncados, sin comas. */
export function formatMontoContable(valor) {
  const num = Number(valor);
  if (!Number.isFinite(num)) return '0.00';
  return truncarDecimales(num, 2).toFixed(2);
}
