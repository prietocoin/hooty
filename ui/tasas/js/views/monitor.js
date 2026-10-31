import { qs } from '../../../shared/js/dom.js';
import { api } from '../api.js?v=20261031_02';
import { fusionarTasas } from '../config.js';
import { state } from '../state.js?v=20261030_02';
import { abrirModal, cerrarModal } from '../ui/modal.js';
import { showToast } from '../ui/toast.js';
import { renderMonitorGrid } from '../components/monitor-grid.js?v=20261030_02';

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
  let previewUrl = '';

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
    showToast(prueba ? '🧪 Publicando lote de prueba...' : '⚡ Publicando carteleras oficiales...');
    try {
      const datos = await api.publicarLote(prueba);
      showToast(`🚀 Publicado a ${datos.procesados ?? 0} socios ON`);
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
    const prueba = Boolean(state.modoPruebaGlobal);
    if (etiqueta) etiqueta.textContent = prueba ? 'PRUEBA' : 'CARTELERA OFICIAL';
    try {
      const blob = await api.previewBlob('', prueba);
      if (previewUrl) URL.revokeObjectURL(previewUrl);
      previewUrl = URL.createObjectURL(blob);
      imagen.src = previewUrl;
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
