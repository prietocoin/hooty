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

function banderaEmoji(regla) {
  const iso = String(regla?.iso || '').trim().toUpperCase();
  if (!/^[A-Z]{2}$/.test(iso)) return '';
  return String.fromCodePoint(...iso.split('').map((letra) => 127397 + letra.charCodeAt(0)));
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
  const polar = esDeposito ? regla.polar_D : regla.polar_P;
  const positivo = polar === '+';
  const porcentaje = esDeposito ? regla.deposito : regla.pago;
  const resultado = tasaResultante(base, porcentaje, tipo);
  const claseCelda = esDeposito ? 'regla-cell--dep' : 'regla-cell--pago';
  const claseEtiqueta = esDeposito ? 'is-dep' : 'is-pago';
  const claseResultado = esDeposito ? 'is-dep' : 'is-pago';
  const nombre = esDeposito ? 'depósito' : 'pago';
  const tituloPolar = positivo
    ? `Polaridad de ${nombre}: suma (+). Clic para restar (−)`
    : `Polaridad de ${nombre}: resta (−). Clic para sumar (+)`;

  return `
    <div class="regla-cell ${claseCelda}">
      <div class="regla-cell-top">
        <span class="regla-cell-label ${claseEtiqueta}">${etiqueta}</span>
        <span class="regla-cell-tasa-wrap">
          <button type="button"
                  data-accion="${esDeposito ? 'toggle-dep' : 'toggle-pago'}"
                  data-indice="${index}"
                  data-moneda="${escapeHTML(regla.codigo)}"
                  data-signo="${positivo ? '+' : '-'}"
                  class="regla-polar ${esDeposito ? 'btn-pol-d btn-polaridad-d' : 'btn-pol-p btn-polaridad-p'} ${positivo ? 'is-pos' : 'is-neg'}"
                  style="width:20px;height:20px;min-width:20px;min-height:20px;display:inline-flex;align-items:center;justify-content:center;padding:0;border-radius:4px;font-weight:bold;font-size:12px;line-height:1;cursor:pointer;position:relative;z-index:1;border:1px solid ${positivo ? 'rgba(16,185,129,.55)' : 'rgba(244,63,94,.55)'};background:${positivo ? 'rgba(16,185,129,.12)' : 'rgba(244,63,94,.12)'};color:${positivo ? '#34d399' : '#fb7185'};"
                  title="${tituloPolar}"
                  aria-label="${tituloPolar}">${positivo ? '+' : '−'}</button>
          <span class="regla-cell-tasa">Tasa</span>
        </span>
      </div>
      <div class="regla-cell-bottom">
        <input type="number" step="any" inputmode="decimal"
               aria-label="Spread de ${nombre} de ${escapeHTML(regla.codigo)}"
               data-regla="${index}" data-campo="${campo}"
               value="${escapeHTML(porcentaje)}"
               class="regla-percent-input">
        <span class="regla-result ${claseResultado}"
              data-resultado="${index}-${campo}"
              title="Tasa resultante">${formatearResultadoSpread(resultado)}</span>
      </div>
    </div>`;
}

function renderRegla(regla, index, tasasBase) {
  const base = baseMoneda(regla, tasasBase);
  const bandera = banderaEmoji(regla);
  const activo = regla.activo !== false;
  const valorBase = Number.isFinite(Number(base)) ? String(base) : '';

  return `
    <div class="regla-card rounded-xl border border-slate-800 bg-[#0b132b] p-3 mb-2.5 ${activo ? '' : 'is-off'}">
      <div class="regla-head flex items-center justify-between gap-2 pb-2 border-b border-slate-800/80">
        <div class="regla-id">
          <span class="regla-flag">${bandera}</span>
          <span class="regla-name">${escapeHTML(regla.nombre)}</span>
          <span class="regla-code">${escapeHTML(regla.codigo)}</span>
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
                  class="regla-del"
                  title="Quitar moneda" aria-label="Quitar ${escapeHTML(regla.codigo)}">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
            </svg>
          </button>
        </div>
      </div>

      <div class="regla-grid grid grid-cols-3 gap-2 pt-2">
        <div class="regla-cell regla-cell--base">
          <div class="regla-cell-top">
            <span class="regla-cell-label is-base">BASE</span>
          </div>
          <input type="number" step="any" inputmode="decimal"
                 aria-label="Tasa base de ${escapeHTML(regla.codigo)}"
                 data-regla="${index}" data-campo="base"
                 value="${escapeHTML(valorBase)}"
                 class="regla-base-input">
        </div>
        ${renderColumnaOperacion({ regla, index, campo: 'deposito', tipo: 'D', base, etiqueta: '↓D%' })}
        ${renderColumnaOperacion({ regla, index, campo: 'pago', tipo: 'P', base, etiqueta: '↑P%' })}
      </div>
    </div>`;
}

// Vista editable: selector de catálogo, tarjetas por moneda y empty state.
// Sin scroll interno: el único scroll es el body del modal.
export function renderReglasEditor(reglas = [], monedaParaAgregar = '', tasasBase = null) {
  const activas = reglas.filter((regla) => regla.activo !== false).length;
  const tarjetas = reglas.length
    ? `<div class="reglas-lista">${reglas.map((regla, index) => renderRegla(regla, index, tasasBase)).join('')}</div>`
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
