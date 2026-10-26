/**
 * @file pwa.js
 * @description Registro del service worker y prompt nativo de instalación PWA.
 *
 * El botón flotante solo aparece cuando Chromium dispara
 * `beforeinstallprompt`. Safari iOS no expone ese evento; para ese caso la app
 * sigue siendo instalable mediante "Añadir a pantalla de inicio" usando el
 * manifest, el apple-touch-icon y los metas apple-mobile-web-app.
 */

const SW_VERSION = '20261026_01';
const INSTALL_BANNER_ID = 'glauk-pwa-install';
const DISMISS_KEY = 'glauk.pwa.install.dismissed';

let deferredPrompt = null;
let installed = false;

function isStandalone() {
  return globalThis.matchMedia?.('(display-mode: standalone)')?.matches === true ||
    globalThis.navigator?.standalone === true;
}

function wasDismissed() {
  try {
    return globalThis.sessionStorage?.getItem(DISMISS_KEY) === '1';
  } catch {
    return false;
  }
}

function markDismissed() {
  try {
    globalThis.sessionStorage?.setItem(DISMISS_KEY, '1');
  } catch {
    /* almacenamiento no disponible */
  }
}

function removeBanner() {
  document.getElementById(INSTALL_BANNER_ID)?.remove();
}

function injectStyles() {
  if (document.getElementById(`${INSTALL_BANNER_ID}-styles`)) return;
  const style = document.createElement('style');
  style.id = `${INSTALL_BANNER_ID}-styles`;
  style.textContent = `
    #${INSTALL_BANNER_ID} {
      position: fixed;
      left: 50%;
      bottom: calc(16px + env(safe-area-inset-bottom, 0px));
      transform: translate(-50%, 20px);
      z-index: 9999;
      display: flex;
      align-items: center;
      gap: 8px;
      width: min(420px, calc(100vw - 24px));
      padding: 8px;
      border: 1px solid rgba(34, 211, 238, .35);
      border-radius: 18px;
      background: rgba(3, 6, 17, .96);
      box-shadow: 0 18px 50px rgba(0, 0, 0, .55), 0 0 0 1px rgba(255,255,255,.03) inset;
      backdrop-filter: blur(14px);
      opacity: 0;
      pointer-events: none;
      transition: opacity .22s ease, transform .22s ease;
    }
    #${INSTALL_BANNER_ID}.is-visible {
      opacity: 1;
      transform: translate(-50%, 0);
      pointer-events: auto;
    }
    #${INSTALL_BANNER_ID} .glauk-pwa-main {
      display: flex;
      align-items: center;
      gap: 10px;
      flex: 1;
      min-width: 0;
      border: 0;
      border-radius: 12px;
      padding: 9px 10px;
      background: linear-gradient(135deg, rgba(8,145,178,.28), rgba(37,99,235,.28));
      color: #ecfeff;
      text-align: left;
      font: inherit;
      cursor: pointer;
    }
    #${INSTALL_BANNER_ID} .glauk-pwa-main:active { transform: scale(.985); }
    #${INSTALL_BANNER_ID} .glauk-pwa-main:disabled { opacity: .65; cursor: wait; }
    #${INSTALL_BANNER_ID} .glauk-pwa-icon {
      display: grid;
      place-items: center;
      width: 38px;
      height: 38px;
      flex: 0 0 38px;
      border-radius: 11px;
      background: rgba(34, 211, 238, .14);
      color: #67e8f9;
    }
    #${INSTALL_BANNER_ID} .glauk-pwa-copy {
      display: flex;
      flex-direction: column;
      min-width: 0;
      line-height: 1.15;
    }
    #${INSTALL_BANNER_ID} .glauk-pwa-copy strong {
      font-size: 13px;
      font-weight: 800;
      letter-spacing: .01em;
    }
    #${INSTALL_BANNER_ID} .glauk-pwa-copy small {
      margin-top: 3px;
      color: #94a3b8;
      font-size: 10.5px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    #${INSTALL_BANNER_ID} .glauk-pwa-close {
      width: 34px;
      height: 34px;
      flex: 0 0 34px;
      border: 0;
      border-radius: 10px;
      background: transparent;
      color: #94a3b8;
      font-size: 20px;
      line-height: 1;
      cursor: pointer;
    }
    #${INSTALL_BANNER_ID} .glauk-pwa-close:hover { background: rgba(148,163,184,.12); color: #e2e8f0; }
    @media (max-width: 380px) {
      #${INSTALL_BANNER_ID} .glauk-pwa-copy small { display: none; }
    }
  `;
  document.head.appendChild(style);
}

function createBanner() {
  injectStyles();

  const banner = document.createElement('div');
  banner.id = INSTALL_BANNER_ID;
  banner.setAttribute('role', 'region');
  banner.setAttribute('aria-label', 'Instalar aplicación');

  const main = document.createElement('button');
  main.type = 'button';
  main.className = 'glauk-pwa-main';
  main.innerHTML = `
    <span class="glauk-pwa-icon" aria-hidden="true">
      <svg width="21" height="21" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="2" stroke-linecap="round"
           stroke-linejoin="round">
        <path d="M12 3v11"></path>
        <path d="m7 10 5 5 5-5"></path>
        <path d="M5 21h14"></path>
      </svg>
    </span>
    <span class="glauk-pwa-copy">
      <strong>Instalar App</strong>
      <small>Acceso rápido desde tu celular</small>
    </span>`;
  main.addEventListener('click', promptInstall);

  const close = document.createElement('button');
  close.type = 'button';
  close.className = 'glauk-pwa-close';
  close.setAttribute('aria-label', 'Cerrar invitación de instalación');
  close.textContent = '×';
  close.addEventListener('click', () => {
    markDismissed();
    removeBanner();
  });

  banner.append(main, close);
  return banner;
}

function showBanner() {
  if (installed || isStandalone() || wasDismissed()) return;
  if (document.getElementById(INSTALL_BANNER_ID)) return;
  if (!document.body) {
    document.addEventListener('DOMContentLoaded', showBanner, { once: true });
    return;
  }
  const banner = createBanner();
  document.body.appendChild(banner);
  requestAnimationFrame(() => banner.classList.add('is-visible'));
}

async function promptInstall() {
  if (!deferredPrompt) return;
  const banner = document.getElementById(INSTALL_BANNER_ID);
  const main = banner?.querySelector('.glauk-pwa-main');
  if (main) main.disabled = true;

  try {
    deferredPrompt.prompt();
    const choice = await deferredPrompt.userChoice;
    if (choice?.outcome === 'accepted') {
      installed = true;
    }
  } catch (err) {
    console.warn('No se pudo abrir el instalador PWA:', err);
  } finally {
    deferredPrompt = null;
    if (main) main.disabled = false;
    removeBanner();
  }
}

globalThis.addEventListener?.('beforeinstallprompt', (event) => {
  event.preventDefault();
  deferredPrompt = event;
  showBanner();
});

globalThis.addEventListener?.('appinstalled', () => {
  installed = true;
  deferredPrompt = null;
  removeBanner();
});

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    const swUrl = new URL(`../sw.js?v=${SW_VERSION}`, import.meta.url);

    navigator.serviceWorker
      .register(swUrl, { updateViaCache: 'none' })
      .then((registration) => registration.update())
      .catch((err) => {
        console.warn('Error al registrar Service Worker PWA:', err);
      });
  });
}

// Si la app ya se abrió instalada, no se debe mostrar ninguna invitación.
if (isStandalone()) {
  installed = true;
}
