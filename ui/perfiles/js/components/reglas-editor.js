import { CATALOGO_MONEDAS } from '../catalog.js';
import { formatearTasaEstricta } from '../utils/tasas.js?v=20261031_04';
import { escapeHTML } from '../../../shared/js/dom.js';

/** Truncamiento estricto a 2 decimales (sin Math.round). */
export function truncar2(valor) {
  const num = Number(valor);
  if (!Number.isFinite(num)) return 0;
  const signo = num < 0 ? -1 : 1;
  const abs = Math.abs(num);
  const desplazado = Number(`${abs}e2`);
  const truncado = Number.isFinite(desplazado)
    ? Math.trunc(desplazado) / 100
    : Math.trunc(abs * 100) / 100;
  return signo * truncado;
}

/**
 * Tasa base de la divisa. Un valor editado a mano (base_manual) manda; si no,
 * se usa el lote activo y el catálogo queda solo como respaldo.
 */
export function baseMoneda(regla, tasasBase) {
  if (regla?.base_manual) {
    const manual = Number(regla?.base);
    if (Number.isFinite(manual) && manual > 0) return manual;
  }
  const delLote = Number(tasasBase?.tasas?.[regla?.codigo]);
  if (Number.isFinite(delLote) && delLote > 0) return delLote;
  const delCatalogo = Number(regla?.base ?? CATALOGO_MONEDAS[regla?.codigo]?.base);
  return Number.isFinite(delCatalogo) && delCatalogo > 0 ? delCatalogo : 0;
}

/**
 * Tasa resultante del spread. Réplica exacta del Core:
 *   D -> base * (1 + deposito/100)
 *   P -> base * (1 - pago/100)
 * Devuelve el producto crudo; el truncamiento se aplica al formatear.
 */
export function tasaResultante(base, porcentaje, tipo) {
  const pct = Math.abs(Number(porcentaje) || 0) / 100;
  const factor = String(tipo).toUpperCase() === 'P' ? 1 - pct : 1 + pct;
  return (Number(base) || 0) * factor;
}

/**
 * Formato de la tasa resultante del spread:
 *   >= 100 -> entero truncado
 *   < 100  -> dos decimales truncados
 * Nunca redondea a múltiplos.
 */
export function formatearResultadoSpread(valor) {
  const num = Number(valor);
  if (!Number.isFinite(num)) return '0.00';
  if (Math.abs(num) >= 100) return String(Math.trunc(num));
  return truncar2(num).toFixed(2);
}

/** Tasa resultante del spread, formateada con la regla estricta. */
export { formatearTasaEstricta };

function urlBandera(regla) {
  const iso = String(regla?.iso || '').trim().toLowerCase();
  return iso ? `/tasas/flags/${iso}.png` : '';
}

function opcionesCatalogo(seleccionada, reglas) {
  return Object.entries(CATALOGO_MONEDAS).map(([codigo, meta]) => {
    const usada = reglas.some((regla) => regla.codigo === codigo);
    const selected = codigo === seleccionada ? ' selected' : '';
    const disabled = usada ? ' disabled' : '';
    return `<option value="${codigo}"${selected}${disabled}>${escapeHTML(meta.iso)} ${escapeHTML(meta.nombre)} (${codigo})</option>`;
  }).join('');
}

const TIPOS_OPERACION = ['D', 'P', 'A'];
const CLASES_TIPO_ACTIVO = {
  D: 'bg-cyan-500 text-slate-950 font-bold',
  P: 'bg-rose-500 text-white font-bold',
  A: 'bg-amber-500 text-slate-950 font-bold'
};

function renderDef(regla, index) {
  const modoRaw = String(regla.modo || 'D').toUpperCase();
  const modo = TIPOS_OPERACION.includes(modoRaw) ? modoRaw : 'D';
  const boton = (valor) => {
    const clases = modo === valor
      ? `is-active ${CLASES_TIPO_ACTIVO[valor]}`
      : 'text-slate-400 hover:text-white';
    return `
      <button type="button" data-accion="modo" data-indice="${index}" data-modo="${valor}"
              class="${clases}"
              aria-pressed="${modo === valor}">${valor}</button>`;
  };
  return `
    <div class="regla-def flex bg-slate-950 rounded border border-slate-700/60 p-0.5" role="group" aria-label="Tipo de operación de ${escapeHTML(regla.codigo)}">
      ${TIPOS_OPERACION.map(boton).join('')}
    </div>`;
}

function renderColumnaOperacion({ regla, index, campo, tipo, base, etiqueta }) {
  const esDeposito = tipo === 'D';
  const porcentaje = esDeposito ? regla.deposito : regla.pago;
  const resultado = tasaResultante(base, porcentaje, tipo);
  const claseCampo = esDeposito ? 'regla-field--dep' : 'regla-field--pago';
  const claseResultado = esDeposito ? 'is-dep text-cyan-300' : 'is-pago text-rose-300';
  const claseEtiqueta = esDeposito ? 'text-cyan-400' : 'text-rose-400';
  const nombre = esDeposito ? 'depósito' : 'pago';

  return `
    <div class="regla-field ${claseCampo} flex flex-col gap-1">
      <span class="regla-field-label ${claseEtiqueta} text-[10px] font-bold uppercase tracking-wider">${etiqueta}</span>
      <div class="regla-percent flex items-center justify-between bg-slate-900 border border-slate-700 rounded px-2 py-1">
        <input type="number" step="any" inputmode="decimal"
               aria-label="Spread de ${nombre} de ${escapeHTML(regla.codigo)}"
               data-regla="${index}" data-campo="${campo}"
               value="${escapeHTML(porcentaje)}"
               class="regla-percent-input w-6 bg-transparent text-xs text-white font-mono text-center focus:outline-none">
        <span class="text-[10px] text-slate-500" aria-hidden="true">%</span>
        <span class="regla-result ${claseResultado} ml-auto text-xs font-mono font-bold"
              data-resultado="${index}-${campo}"
              title="Tasa resultante">${formatearResultadoSpread(resultado)}</span>
      </div>
    </div>`;
}

function renderRegla(regla, index, tasasBase) {
  const base = baseMoneda(regla, tasasBase);
  const bandera = urlBandera(regla);
  const alt = `Bandera ${escapeHTML(regla.codigo)}`;
  const activo = regla.activo !== false;
  const valorBase = Number.isFinite(Number(base)) ? String(base) : '';

  return `
    <div class="regla-card rounded-xl border border-slate-800 bg-[#0b132b] p-3 mb-2.5 ${activo ? '' : 'is-off'}">
      <div class="regla-head flex items-center justify-between gap-2 pb-2 border-b border-slate-800/80">
        <div class="regla-id flex items-center gap-2 min-w-0">
          <span class="text-base leading-none">
            ${bandera ? `<img class="regla-flag" src="${bandera}" alt="${alt}" loading="lazy" onerror="this.style.display='none'">` : ''}
          </span>
          <span class="regla-name font-bold text-white text-sm truncate">${escapeHTML(regla.nombre)}</span>
          <span class="regla-code text-xs font-semibold text-cyan-400 font-mono">${escapeHTML(regla.codigo)}</span>
        </div>
        <div class="regla-controls flex items-center gap-2 shrink-0">
          ${renderDef(regla, index)}
          <button type="button"
                  data-accion="toggle-cartelera"
                  data-indice="${index}"
                  class="regla-switch relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full transition-colors duration-200 ${activo ? 'is-on bg-emerald-500' : 'is-off bg-slate-700'}"
                  aria-pressed="${activo}"
                  title="Incluir ${escapeHTML(regla.codigo)} en cartelera"
                  aria-label="Incluir ${escapeHTML(regla.codigo)} en la cartelera">
            <span class="regla-switch-thumb ${activo ? 'translate-x-4' : 'translate-x-0.5'} inline-block h-4 w-4 transform rounded-full bg-white transition duration-200 mt-0.5 shadow"></span>
          </button>
          <button type="button" data-accion="eliminar-moneda" data-indice="${index}"
                  class="regla-del text-slate-500 hover:text-rose-400 pl-1"
                  title="Quitar moneda" aria-label="Quitar ${escapeHTML(regla.codigo)}">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
            </svg>
          </button>
        </div>
      </div>

      <div class="regla-grid grid grid-cols-3 gap-2 pt-2">
        <div class="regla-field flex flex-col gap-1">
          <span class="regla-field-label text-[10px] font-bold text-slate-400 uppercase tracking-wider">Base</span>
          <input type="number" step="any" inputmode="decimal"
                 aria-label="Tasa base de ${escapeHTML(regla.codigo)}"
                 data-regla="${index}" data-campo="base"
                 value="${escapeHTML(valorBase)}"
                 class="regla-base-input w-full bg-slate-900 border border-slate-700 rounded px-2 py-1 text-xs text-white font-mono font-bold text-center focus:outline-none focus:border-cyan-500">
        </div>
        ${renderColumnaOperacion({ regla, index, campo: 'deposito', tipo: 'D', base, etiqueta: '↓ Depósito' })}
        ${renderColumnaOperacion({ regla, index, campo: 'pago', tipo: 'P', base, etiqueta: '↑ Pago' })}
      </div>
    </div>`;
}

// Vista editable: selector de catálogo, tarjetas por moneda y empty state.
// Sin scroll interno: el único scroll es el body del modal.
export function renderReglasEditor(reglas = [], monedaParaAgregar = '', tasasBase = null) {
  const activas = reglas.filter((regla) => regla.activo !== false).length;
  const tarjetas = reglas.length
    ? `<div class="grid grid-cols-1 md:grid-cols-2 gap-3">${reglas.map((regla, index) => renderRegla(regla, index, tasasBase)).join('')}</div>`
    : '';
  const vacio = reglas.length === 0
    ? '<div class="text-center py-6 border border-dashed border-slate-800 rounded-xl text-slate-500 text-xs">Sin reglas configuradas.</div>'
    : '';
  const deshabilitado = monedaParaAgregar ? '' : ' disabled';
  return `
    <div class="space-y-4">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <h3 class="text-xs font-bold text-slate-200 tracking-wider uppercase flex items-center gap-2">
          <span>Reglas Individuales (%)</span>
          <span class="text-[10px] px-2 py-0.5 rounded border bg-cyan-500/10 text-cyan-300 border-cyan-500/20">${activas} activas</span>
        </h3>

        <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
          <select id="perfiles-moneda-agregar" aria-label="Moneda para agregar" class="min-h-11 bg-[#030611] border border-slate-700 text-xs rounded-lg px-3 py-2 text-slate-200 focus:outline-none focus:border-cyan-500">
            <option value="">Seleccionar país/moneda...</option>
            ${opcionesCatalogo(monedaParaAgregar, reglas)}
          </select>
          <button type="button" data-accion="agregar-moneda"${deshabilitado} class="min-h-11 px-3 py-2 rounded-lg border border-cyan-500/50 bg-cyan-500/10 text-cyan-400 text-xs font-bold hover:bg-cyan-500/20 disabled:opacity-40 disabled:cursor-not-allowed transition flex items-center justify-center gap-1">+ Agregar Moneda</button>
        </div>
      </div>

      ${tarjetas}
      ${vacio}
    </div>`;
}
