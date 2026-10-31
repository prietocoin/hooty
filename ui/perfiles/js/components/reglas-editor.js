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
      : 'text-slate-400';
    return `
      <button type="button" data-accion="modo" data-indice="${index}" data-modo="${valor}"
              class="${clases}"
              aria-pressed="${modo === valor}">${valor}</button>`;
  };
  return `
    <div class="regla-def" role="group" aria-label="Tipo de operación de ${escapeHTML(regla.codigo)}">
      ${TIPOS_OPERACION.map(boton).join('')}
    </div>`;
}

function renderColumnaOperacion({ regla, index, campo, tipo, base, etiqueta }) {
  const esDeposito = tipo === 'D';
  const porcentaje = esDeposito ? regla.deposito : regla.pago;
  const resultado = tasaResultante(base, porcentaje, tipo);
  const claseCampo = esDeposito ? 'regla-field--dep' : 'regla-field--pago';
  const claseResultado = esDeposito ? 'is-dep' : 'is-pago';
  const nombre = esDeposito ? 'depósito' : 'pago';

  return `
    <div class="regla-field ${claseCampo}">
      <span class="regla-field-label">${etiqueta}</span>
      <div class="regla-percent">
        <input type="number" step="0.01" inputmode="decimal"
               aria-label="Spread de ${nombre} de ${escapeHTML(regla.codigo)}"
               data-regla="${index}" data-campo="${campo}"
               value="${escapeHTML(porcentaje)}"
               class="regla-percent-input">
        <span aria-hidden="true">%</span>
      </div>
      <span class="regla-result ${claseResultado}" data-resultado="${index}-${campo}"
            title="Tasa resultante">${formatearResultadoSpread(resultado)}</span>
    </div>`;
}

function renderRegla(regla, index, tasasBase) {
  const base = baseMoneda(regla, tasasBase);
  const bandera = urlBandera(regla);
  const alt = `Bandera ${escapeHTML(regla.codigo)}`;
  const activo = regla.activo !== false;
  const valorBase = Number.isFinite(Number(base)) ? String(base) : '';

  return `
    <div class="regla-card ${activo ? '' : 'is-off'}">
      <div class="regla-head">
        <div class="regla-id">
          ${bandera ? `<img class="regla-flag" src="${bandera}" alt="${alt}" loading="lazy" onerror="this.style.display='none'">` : ''}
          <span class="regla-name">${escapeHTML(regla.nombre)} <span class="regla-code">${escapeHTML(regla.codigo)}</span></span>
        </div>
        <div class="regla-controls">
          ${renderDef(regla, index)}
          <button type="button"
                  data-accion="toggle-cartelera"
                  data-indice="${index}"
                  class="regla-switch ${activo ? 'is-on' : 'is-off'}"
                  aria-pressed="${activo}"
                  title="Incluir ${escapeHTML(regla.codigo)} en cartelera"
                  aria-label="Incluir ${escapeHTML(regla.codigo)} en la cartelera">
            <span class="regla-switch-thumb"></span>
          </button>
          <button type="button" data-accion="eliminar-moneda" data-indice="${index}"
                  class="regla-del text-slate-500 hover:text-rose-400"
                  title="Quitar moneda" aria-label="Quitar ${escapeHTML(regla.codigo)}">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="M19 6l-1 14H6L5 6"/>
              <path d="M10 11v5M14 11v5"/>
            </svg>
          </button>
        </div>
      </div>

      <div class="regla-grid">
        <label class="regla-field">
          <span class="regla-field-label">Base</span>
          <input type="number" step="0.01" inputmode="decimal"
                 aria-label="Tasa base de ${escapeHTML(regla.codigo)}"
                 data-regla="${index}" data-campo="base"
                 value="${escapeHTML(valorBase)}"
                 class="regla-base-input">
        </label>
        ${renderColumnaOperacion({ regla, index, campo: 'deposito', tipo: 'D', base, etiqueta: '↓ DEP' })}
        ${renderColumnaOperacion({ regla, index, campo: 'pago', tipo: 'P', base, etiqueta: '↑ PAG' })}
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
