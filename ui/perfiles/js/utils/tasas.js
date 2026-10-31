/**
 * @file tasas.js
 * @description Formato estricto de tasas para el módulo de perfiles.
 *
 * Regla financiera: truncamiento estricto a 2 decimales, siempre mostrando
 * ambos decimales. Sin Math.round, sin toFixed que redondee y sin eliminar
 * ceros. El desplazamiento decimal se hace por cadena para evitar artefactos
 * binarios (4.29 * 100 = 428.99999999999994).
 */

function truncarA(valor, decimales) {
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

export function formatearTasaEstricta(valor) {
  const num = Number(valor);
  if (!Number.isFinite(num)) return '0.00';
  return truncarA(num, 2).toFixed(2);
}
