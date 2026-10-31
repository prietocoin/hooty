import { initPWA } from './pwa.js';
import { cerrarModal } from './ui/modal.js';
import { initMonitor } from './views/monitor.js?v=20261031_01';

const monitor = initMonitor();

document.addEventListener('click', (evento) => {
  const boton = evento.target.closest('[data-close-modal]');
  if (boton) cerrarModal(boton.dataset.closeModal);
});

initPWA();

async function iniciarMonitor() {
  await monitor.refresh();
}

if (document.readyState === 'complete') {
  iniciarMonitor();
} else {
  window.addEventListener('load', iniciarMonitor, { once: true });
}
