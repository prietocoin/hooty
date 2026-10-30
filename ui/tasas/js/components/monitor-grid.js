import { el } from '../../../shared/js/dom.js';
import {
  BANDERAS_EMOJI_FALLBACK,
  esClaveMeta,
  obtenerUrlBandera
} from '../config.js';

function construirBandera(code) {
  const imagen = el('img', {
    class: 'flag',
    src: obtenerUrlBandera(code),
    alt: `Bandera ${code}`
  });
  const respaldo = el('span', { class: 'flag-fallback' });
  imagen.addEventListener('error', () => {
    imagen.remove();
    respaldo.textContent = BANDERAS_EMOJI_FALLBACK[code] || '🌐';
  });
  return [imagen, respaldo];
}

function construirTarjeta(code, valorProd, valorNuevo, alCambiar, index = 0) {
  const tarjeta = el('div', { class: 'currency-card' });
  tarjeta.style.setProperty('--card-index', String(index));
  const estado = el('span', {
    class: 'rate-status',
    title: 'Tasa activa',
    'aria-label': 'Tasa activa'
  });
  const cabecera = el(
    'div',
    { class: 'currency-code' },
    construirBandera(code),
    el('span', { class: 'currency-name' }, code),
    estado
  );
  const produccion = el(
    'div',
    { class: 'prod-row' },
    el('span', {}, 'PROD:'),
    el('span', { class: 'prod-value' }, String(valorProd))
  );

  const entrada = el('input', {
    type: 'number',
    step: 'any',
    class: 'rate-input',
    value: String(valorNuevo),
    'aria-label': `Nuevo lote para ${code}`
  });
  entrada.addEventListener('input', () => alCambiar(code, entrada.value));

  const bloque = el(
    'div',
    {},
    el('span', { class: 'rate-label' }, 'NUEVO LOTE'),
    entrada
  );

  tarjeta.append(cabecera, produccion, bloque);
  return tarjeta;
}

export function renderMonitorGrid(contenedor, tasasProd, tasasNuevo, alCambiar) {
  const codigos = new Set([...Object.keys(tasasProd), ...Object.keys(tasasNuevo)]);
  const tarjetas = [...codigos]
    .filter((clave) => esClaveMeta(clave))
    .map((code, index) => construirTarjeta(
      code,
      tasasProd[code] ?? 0,
      tasasNuevo[code] ?? tasasProd[code] ?? 0,
      alCambiar,
      index
    ));
  contenedor.replaceChildren(...tarjetas);
}
