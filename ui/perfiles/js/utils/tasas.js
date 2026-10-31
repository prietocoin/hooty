/**
 * @file tasas.js
 * @description Formato estricto de tasas para el módulo de perfiles.
 *
 * Regla financiera:
 *   >= 100       -> entero truncado
 *   1.00 .. 99.99 -> dos decimales truncados
 *   < 1.00       -> tres cifras significativas truncadas
 *
 * Sin Math.round ni toFixed que redondee. El desplazamiento decimal se hace por
 * cadena para evitar artefactos binarios.
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
  if (num === 0) return '0';

  const abs = Math.abs(num);

  // >= 100: entero truncado.
  if (abs >= 100) return String(Math.trunc(num));

  // >= 1 y < 100: dos decimales truncados.
  if (abs >= 1) return truncarA(num, 2).toFixed(2);

  // < 1: tres cifras significativas truncadas.
  const orden = Math.floor(Math.log10(abs));
  const decimales = Math.max(0, 3 - 1 - orden);
  return truncarA(num, decimales).toFixed(decimales);
}
