import { qs } from '../../../shared/js/dom.js';
import { api } from '../api.js?v=20261031_01';
import { fusionarTasas } from '../config.js';
import { state } from '../state.js?v=20261030_02';
import { abrirModal, cerrarModal } from '../ui/modal.js';
import { showToast } from '../ui/toast.js';
import { renderMonitorGrid } from '../components/monitor-grid.js?v=20261030_02';

function escaparHTML(valor) {
  return String(valor ?? '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[char]));
}

function parsearJSON(valor) {
  if (typeof valor === 'string') {
    try { return JSON.parse(valor); } catch (error) { return {}; }
  }
  return valor && typeof valor === 'object' ? valor : {};
}

function socioActivo(perfil) {
  const mostrar = parsearJSON(perfil?.mostrar);
  const tasas = mostrar.tasas;
  return !(tasas === false || tasas === 'false' || tasas === 0 || tasas === '0');
}

function truncar2(valor) {
  const numero = Number(valor);
  if (!Number.isFinite(numero)) return 0;
  return Math.trunc(numero * 100) / 100;
}

function tasasDelSocio(perfil, tasas) {
  const configuradas = parsearJSON(perfil?.monedas);
  const filas = [];
  for (const [codigo, valor] of Object.entries(tasas || {})) {
    const code = String(codigo).toUpperCase();
    if (!/^[A-Z]{3,5}$/.test(code)) continue;
    const base = Number(valor);
    if (!Number.isFinite(base) || base <= 0) continue;
    const regla = configuradas[code] || configuradas[codigo];
    if (regla && regla.activo === false) continue;
    let factor = 1;
    if (regla) {
      const tipo = String(regla.tipo || regla.def || 'D').toUpperCase();
      const porcentaje = tipo === 'P'
        ? (regla.porcentaje?.pago ?? regla.pago ?? 0)
        : (regla.porcentaje?.deposito ?? regla.deposito ?? 0);
      factor = tipo === 'P'
        ? 1 - Math.abs(Number(porcentaje) || 0) / 100
        : 1 + Math.abs(Number(porcentaje) || 0) / 100;
    }
    filas.push({ code, value: truncar2(base * factor) });
  }
  return filas.sort((a, b) => a.code.localeCompare(b.code));
}

function htmlCartelera(nombre, loteID, filas, prueba) {
  const badge = prueba
    ? '<span style="display:inline-block;margin-top:16px;padding:7px 14px;border-radius:999px;background:#f59e0b;color:#0f172a;font-size:14px;font-weight:800;letter-spacing:0.08em;">PRUEBA</span>'
    : '';
  const tarjetas = filas.map((fila, indice) => `
    <div style="position:relative;display:flex;align-items:center;justify-content:space-between;background:linear-gradient(135deg,rgba(21,27,52,0.96),rgba(12,18,38,0.96));border:1px solid rgba(34,211,238,0.18);border-radius:16px;padding:15px 22px;margin-top:11px;box-shadow:0 10px 24px rgba(2,6,23,0.35);">
      <span style="font-size:22px;font-weight:800;color:#e2e8f0;letter-spacing:0.04em;">${escaparHTML(fila.code)}</span>
      <span style="font-size:23px;font-weight:800;color:#34d399;">${fila.value.toFixed(2)}</span>
      <span style="position:absolute;right:-6px;top:-6px;width:10px;height:10px;border-radius:50%;background:${indice % 2 ? '#22d3ee' : '#a78bfa'};box-shadow:0 0 14px currentColor;"></span>
    </div>`).join('');
  return `
    <div style="position:relative;width:900px;box-sizing:border-box;padding:44px 44px 38px;overflow:hidden;background:radial-gradient(circle at 15% 0%,#12204a 0%,#090d18 42%,#060912 100%);color:#f8fafc;font-family:Inter,Arial,sans-serif;">
      <div style="position:absolute;top:-260px;right:-180px;width:620px;height:620px;border-radius:50%;border:1px solid rgba(34,211,238,0.20);"></div>
      <div style="position:absolute;top:-180px;right:-100px;width:460px;height:460px;border-radius:50%;border:1px solid rgba(167,139,250,0.18);"></div>
      <div style="position:absolute;bottom:-240px;left:-160px;width:520px;height:520px;border-radius:50%;border:1px solid rgba(52,211,153,0.12);"></div>
      <div style="position:absolute;top:70px;right:140px;width:8px;height:8px;border-radius:50%;background:#22d3ee;box-shadow:0 0 18px #22d3ee;"></div>
      <div style="position:absolute;top:190px;right:70px;width:5px;height:5px;border-radius:50%;background:#a78bfa;box-shadow:0 0 12px #a78bfa;"></div>
      <div style="position:absolute;top:290px;left:70px;width:6px;height:6px;border-radius:50%;background:#34d399;box-shadow:0 0 14px #34d399;"></div>
      <div style="position:relative;">
        <div style="font-size:48px;font-weight:800;color:#22d3ee;letter-spacing:0.06em;">HOOTY</div>
        <div style="font-size:15px;font-weight:700;color:#94a3b8;margin-top:4px;letter-spacing:0.18em;">MERCADO &amp; TASAS</div>
        <div style="font-size:20px;font-weight:800;margin-top:24px;color:#cbd5e1;">LOTE ${escaparHTML(loteID)}</div>
        <div style="font-size:34px;font-weight:800;margin-top:16px;background:linear-gradient(90deg,#f8fafc,#93c5fd);-webkit-background-clip:text;background-clip:text;color:transparent;">${escaparHTML(nombre)}</div>
        ${badge}
        <div style="margin-top:24px;">${tarjetas || '<div style="color:#94a3b8;font-size:18px;">Sin tasas configuradas</div>'}</div>
        <div style="font-size:13px;color:#64748b;margin-top:26px;letter-spacing:0.04em;">Generado ${new Date().toLocaleString('es-VE')}</div>
      </div>
    </div>`;
}

async function generarCarteleraSocio(nombre, lote, prueba, perfil) {
  const contenedor = document.getElementById('carteleraCaptura');
  if (!contenedor) throw new Error('Falta el contenedor de captura');
  if (typeof globalThis.html2canvas !== 'function') {
    throw new Error('html2canvas no está disponible');
  }
  const filas = tasasDelSocio(perfil, lote.tasas);
  contenedor.innerHTML = htmlCartelera(nombre, lote.id_tasa || 'T000', filas, prueba);
  const objetivo = contenedor.firstElementChild;
  try {
    const canvas = await globalThis.html2canvas(objetivo, {
      backgroundColor: '#090d18',
      scale: 1,
      logging: false,
      useCORS: true
    });
    return canvas.toDataURL('image/png');
  } finally {
    contenedor.replaceChildren();
  }
}

function alHacer(selector, evento, manejador) {
  const elemento = qs(selector);
  elemento?.addEventListener(evento, manejador);
  return elemento;
}

function mostrarEstadoGrid(grid, mensaje, esError = false) {
  if (!grid) return;
  const aviso = document.createElement('p');
  aviso.className = esError ? 'monitor-state is-error' : 'monitor-state';
  aviso.textContent = mensaje;
  grid.replaceChildren(aviso);
}

export function initMonitor() {
  const etiquetaLote = qs('#lblLoteActual');
  const grid = qs('#gridMonitor');
  const entradaBorrado = qs('#txtBatchIdToDelete');

  function actualizarValor(code, valor) {
    state.borradorTasas[code] = parseFloat(valor) || 0;
  }

  function renderGrid() {
    renderMonitorGrid(grid, state.loteVigente, state.borradorTasas, actualizarValor);
  }

  // La base de datos (PROD) y el lote editable son estados independientes.
  // Al cargar la vista, el borrador parte del lote vigente.
  function aplicarLoteVigente(mapaTasas) {
    const tasas = fusionarTasas(mapaTasas);
    state.loteVigente = tasas;
    state.borradorTasas = { ...tasas };
    renderGrid();
  }

  // El borrador HOO solo puede tocar las casillas de "NUEVO LOTE".
  function aplicarBorrador(mapaTasas) {
    state.borradorTasas = fusionarTasas({ ...state.borradorTasas, ...mapaTasas });
    renderGrid();
  }

  function actualizarEtiquetaLote() {
    if (etiquetaLote) etiquetaLote.textContent = `Lote en Prod: ${state.loteActualCodigo}`;
  }

  async function refresh() {
    try {
      const datos = await api.monitor();
      state.loteActualCodigo = datos.id_tasa || 'T000';
      actualizarEtiquetaLote();
      aplicarLoteVigente(datos.tasas || {});
      return true;
    } catch (error) {
      console.error('Error monitor:', error);
      mostrarEstadoGrid(grid, `No se pudieron cargar las tasas (${error.message}).`, true);
      showToast(`❌ No se pudieron cargar las tasas: ${error.message}`);
      return false;
    }
  }

  async function guardarLote() {
    showToast('💾 Guardando en tasa...');
    try {
      const datos = await api.guardarLote(state.borradorTasas);
      state.loteActualCodigo = datos.id_tasa;
      actualizarEtiquetaLote();
      showToast(`✅ Lote ${datos.id_tasa} guardado en BD`);
    } catch (error) {
      showToast(`❌ Error: ${error.message}`);
    }
  }

  async function publicarLote() {
    const prueba = Boolean(state.modoPruebaGlobal);
    showToast(prueba ? '🧪 Generando cartelera de prueba...' : '⚡ Generando carteleras para socios ON...');
    try {
      const lote = await api.monitor();
      const capturas = [];
      if (prueba) {
        const imagen = await generarCarteleraSocio('PRUEBA', lote, true, null);
        capturas.push({ socio: 'PRUEBA', chat_id: '', imagen });
      } else {
        const perfiles = await api.perfiles();
        const socios = (Array.isArray(perfiles) ? perfiles : [])
          .filter((perfil) => perfil && perfil.nombre && perfil.id_grupo &&
            String(perfil.nombre).trim().toUpperCase() !== 'GENERAL' && socioActivo(perfil));
        if (!socios.length) throw new Error('No hay socios activos ON');
        for (let i = 0; i < socios.length; i++) {
          const socio = socios[i];
          showToast(`🎨 Capturando ${socio.nombre} (${i + 1}/${socios.length})`);
          const imagen = await generarCarteleraSocio(socio.nombre, lote, false, socio);
          capturas.push({ socio: socio.nombre, chat_id: socio.id_grupo, imagen });
        }
      }
      showToast(`📤 Enviando ${capturas.length} cartelera(s)...`);
      const datos = await api.publicarLote(prueba, capturas);
      showToast(`🚀 Publicado a ${datos.procesados ?? capturas.length} socios ON`);
    } catch (error) {
      showToast(`❌ Error al publicar: ${error.message}`);
    }
  }

  async function verPrevia() {
    const spinner = qs('#previewSpinner');
    const imagen = qs('#imgPreviewCartelera');
    const etiqueta = qs('#lblPreviewSocio');
    if (!spinner || !imagen) return;
    spinner.classList.remove('hidden');
    imagen.classList.add('hidden');
    imagen.removeAttribute('src');
    abrirModal('modalPreview');
    try {
      const lote = await api.monitor();
      const prueba = Boolean(state.modoPruebaGlobal);
      let perfil = null;
      let nombre = 'PRUEBA';
      if (!prueba) {
        const perfiles = await api.perfiles();
        perfil = (Array.isArray(perfiles) ? perfiles : [])
          .find((item) => item && item.nombre && item.id_grupo &&
            String(item.nombre).trim().toUpperCase() !== 'GENERAL' && socioActivo(item)) || null;
        nombre = perfil?.nombre || 'GENERAL';
      }
      if (etiqueta) etiqueta.textContent = nombre;
      imagen.src = await generarCarteleraSocio(nombre, lote, prueba, perfil);
      imagen.classList.remove('hidden');
      spinner.classList.add('hidden');
    } catch (error) {
      cerrarModal('modalPreview');
      showToast(`❌ Error generando la vista previa: ${error.message}`);
    }
  }

  async function traerBorradorHoo() {
    showToast('📑 Leyendo el borrador Hoo...');
    try {
      const datos = await api.borradorHoo();
      if (!datos.borrador || Object.keys(datos.borrador).length === 0) {
        showToast('❌ No se recibió un borrador reciente de Hoo');
        return;
      }
      aplicarBorrador(datos.borrador);
      showToast('✅ Borrador cargado en NUEVO LOTE');
    } catch (error) {
      showToast(`❌ Error de conexión al leer el borrador: ${error.message}`);
    }
  }

  async function enviarTasaRemesas() {
    showToast('⏳ Activando flujo de remesas...');
    try {
      await api.notificarRemesas();
      showToast('✅ Notificación insertada en BD');
    } catch (error) {
      showToast(`❌ Error: ${error.message}`);
    }
  }

  function abrirBorrado() {
    if (entradaBorrado) entradaBorrado.value = state.loteActualCodigo;
    abrirModal('modalDeleteBatch');
  }

  async function confirmarBorrado() {
    const batchId = entradaBorrado?.value.trim() || '';
    if (!batchId) {
      showToast('⚠️ Ingrese un ID de lote válido');
      return;
    }

    showToast(`🗑️ Borrando lote ${batchId}...`);
    try {
      await api.eliminarLote(batchId);
      showToast(`✅ Lote ${batchId} eliminado`);
      cerrarModal('modalDeleteBatch');
      await refresh();
    } catch (error) {
      showToast(`❌ ${error.message}`);
    }
  }

  function cambiarModoPrueba(checked) {
    state.modoPruebaGlobal = checked;
    showToast(`Prueba Lote: ${checked ? 'ON' : 'OFF'}`);
  }

  alHacer('#btnGuardarLote', 'click', guardarLote);
  alHacer('#btnPublicar', 'click', publicarLote);
  alHacer('#btnVerPrevia', 'click', verPrevia);
  alHacer('#btnHoo', 'click', traerBorradorHoo);
  alHacer('#btnRemesas', 'click', enviarTasaRemesas);
  alHacer('#btnBorrarLote', 'click', abrirBorrado);
  alHacer('#btnConfirmarBorrar', 'click', confirmarBorrado);
  alHacer('#globalTestSwitch', 'change', (evento) => cambiarModoPrueba(evento.target.checked));

  if (grid && !grid.childElementCount) {
    mostrarEstadoGrid(grid, 'Cargando tasas del lote vigente...');
  }

  return { refresh, aplicarBorrador, aplicarLoteVigente };
}
