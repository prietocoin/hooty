import { initPWA } from './pwa.js';
import { cerrarModal } from './ui/modal.js';
import { initMonitor } from './views/monitor.js';

const monitor = initMonitor();

document.addEventListener('click', (evento) => {
  const boton = evento.target.closest('[data-close-modal]');
  if (boton) cerrarModal(boton.dataset.closeModal);
});

initPWA();

window.addEventListener('load', async () => {
  await monitor.refresh();
});
