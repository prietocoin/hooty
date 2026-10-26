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

function construirTarjeta(code, valor, alCambiar, index = 0) {
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
    el('span', { class: 'prod-value' }, String(valor))
  );

  const entrada = el('input', {
    type: 'number',
    step: 'any',
    class: 'rate-input',
    value: String(valor),
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

export function renderMonitorGrid(contenedor, mapaTasas, alCambiar) {
  const tarjetas = Object.entries(mapaTasas)
    .filter(([clave]) => esClaveMeta(clave))
    .map(([code, valor], index) => construirTarjeta(code, valor, alCambiar, index));
  contenedor.replaceChildren(...tarjetas);
}
