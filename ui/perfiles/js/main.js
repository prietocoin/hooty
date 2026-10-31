import { api } from './api.js?v=20261031_03';
import { state } from './store.js?v=20261024_99_FIX_TASAS';
import { crearEditor } from './controllers/editor.js?v=20261031_02';
import { crearDirectorio } from './controllers/directorio.js?v=20261031_01';
import { crearAcciones } from './controllers/acciones.js?v=20261031_02';

async function recargarSocios() {
  try {
    state.socios = await api.listar();
  } catch (err) {
    console.error('Error cargando socios:', err);
  }
  directorio.render();
}

// Lote activo de tasas para la píldora "base" y el cálculo en vivo del spread.
async function cargarTasasBase() {
  try {
    state.tasasBase = await api.tasasUltimas();
  } catch (err) {
    console.warn('No se pudo cargar el lote activo de tasas:', err);
  }
  if (state.modalAbierto) editor.render();
}

async function abrirPrevia(socio) {
  const nombre = String(socio?.nombre || '').trim();
  if (!nombre) return;
  let url = '';
  const overlay = document.createElement('div');
  overlay.style.cssText = 'position:fixed;inset:0;z-index:9999;background:rgba(2,6,23,.9);display:flex;align-items:center;justify-content:center;padding:16px;';
  const card = document.createElement('div');
  card.style.cssText = 'width:min(620px,100%);max-height:92vh;overflow:auto;background:#0b1020;border:1px solid rgba(34,211,238,.25);border-radius:18px;box-shadow:0 24px 80px rgba(0,0,0,.55);';
  card.innerHTML = `
    <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;padding:14px 16px;border-bottom:1px solid rgba(148,163,184,.16);">
      <div style="min-width:0;">
        <strong style="display:block;color:#e2e8f0;font-size:14px;">👁 Cartelera del socio</strong>
        <span data-nombre style="display:block;color:#22d3ee;font-size:12px;font-weight:700;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;"></span>
      </div>
      <div style="display:flex;align-items:center;gap:6px;flex-shrink:0;">
        <button type="button" data-test title="Enviar a Prueba" aria-label="Enviar a Prueba"
                style="display:inline-flex;align-items:center;justify-content:center;width:36px;height:36px;border-radius:10px;border:1px solid rgba(168,85,247,.45);background:rgba(88,28,135,.35);color:#d8b4fe;cursor:pointer;">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 3h6"/><path d="M10 3v6l-5 9a2 2 0 0 0 1.8 3h10.4a2 2 0 0 0 1.8-3l-5-9V3"/><path d="M7.5 15h9"/></svg>
        </button>
        <button type="button" data-enviar title="Enviar al socio" aria-label="Enviar al socio"
                style="display:inline-flex;align-items:center;justify-content:center;width:36px;height:36px;border-radius:10px;border:1px solid rgba(245,158,11,.45);background:rgba(120,53,15,.35);color:#fcd34d;cursor:pointer;">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>
        </button>
        <button type="button" data-descargar title="Descargar" aria-label="Descargar"
                style="display:inline-flex;align-items:center;justify-content:center;width:36px;height:36px;border-radius:10px;border:1px solid rgba(34,211,238,.45);background:rgba(8,47,73,.35);color:#67e8f9;cursor:pointer;">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/></svg>
        </button>
        <button type="button" data-cerrar title="Cerrar" aria-label="Cerrar"
                style="display:inline-flex;align-items:center;justify-content:center;width:36px;height:36px;border-radius:10px;border:1px solid rgba(148,163,184,.25);background:#030611;color:#cbd5e1;cursor:pointer;">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
        </button>
      </div>
    </div>
    <div data-cuerpo style="padding:14px;">
      <p style="margin:0;padding:32px 0;text-align:center;color:#94a3b8;font-size:12px;font-weight:700;">Generando cartelera oficial…</p>
    </div>`;
  card.querySelector('[data-nombre]').textContent = nombre;
  overlay.append(card);
  document.body.append(overlay);

  const cerrar = () => {
    if (url) URL.revokeObjectURL(url);
    url = '';
    overlay.remove();
    document.removeEventListener('keydown', alPresionar);
  };
  const alPresionar = (event) => { if (event.key === 'Escape') cerrar(); };
  const enviar = async (prueba) => {
    try {
      await api.disparar(nombre, prueba);
      alert(`✅ Cartelera de ${nombre} enviada ${prueba ? 'al grupo de prueba' : 'al socio'}.`);
    } catch (error) {
      alert('❌ Error al enviar cartelera: ' + error.message);
    }
  };
  const descargar = () => {
    if (!url) {
      alert('La cartelera todavía se está generando.');
      return;
    }
    const enlace = document.createElement('a');
    enlace.href = url;
    enlace.download = `Cartelera_${nombre.replace(/[^a-z0-9]/gi, '_')}.png`;
    document.body.append(enlace);
    enlace.click();
    enlace.remove();
  };
  card.querySelector('[data-cerrar]').addEventListener('click', cerrar);
  card.querySelector('[data-test]').addEventListener('click', () => { void enviar(true); });
  card.querySelector('[data-enviar]').addEventListener('click', () => { void enviar(false); });
  card.querySelector('[data-descargar]').addEventListener('click', descargar);
  overlay.addEventListener('click', (event) => { if (event.target === overlay) cerrar(); });
  document.addEventListener('keydown', alPresionar);

  try {
    const blob = await api.previewBlob(nombre);
    url = URL.createObjectURL(blob);
    const imagen = document.createElement('img');
    imagen.src = url;
    imagen.alt = `Cartelera de ${nombre}`;
    imagen.style.cssText = 'display:block;width:100%;height:auto;border-radius:12px;border:1px solid rgba(34,211,238,.18);';
    card.querySelector('[data-cuerpo]').replaceChildren(imagen);
  } catch (error) {
    const aviso = document.createElement('p');
    aviso.textContent = error.message || 'No se pudo generar la cartelera.';
    aviso.style.cssText = 'margin:0;padding:32px 0;text-align:center;color:#fb7185;font-size:12px;font-weight:700;';
    card.querySelector('[data-cuerpo]').replaceChildren(aviso);
  }
}

const editor = crearEditor({ recargar: recargarSocios });
const directorio = crearDirectorio({
  recargar: recargarSocios,
  editar: (socio) => editor.abrir(socio),
  previa: abrirPrevia
});
const acciones = crearAcciones({
  recargar: recargarSocios,
  editar: (socio) => editor.abrir(socio)
});

function registrarServiceWorker() {
  if (!('serviceWorker' in navigator)) return;
  const script = new URL('../sw.js?v=20261024_99_FIX_TASAS', import.meta.url);
  const scope = new URL('../', import.meta.url).pathname;
  navigator.serviceWorker
    .register(script, { scope, updateViaCache: 'none' })
    .catch(() => { /* offline opcional */ });
}

function boot() {
  directorio.mount();
  editor.mount();
  acciones.mount();
  registrarServiceWorker();
  directorio.render();
  void cargarTasasBase();
  void recargarSocios();
}

boot();
