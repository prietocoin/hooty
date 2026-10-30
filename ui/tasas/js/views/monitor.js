import { qs } from '../../../shared/js/dom.js';
import { api } from '../api.js?v=20261030_01';
import { fusionarTasas } from '../config.js';
import { state } from '../state.js';
import { abrirModal, cerrarModal } from '../ui/modal.js';
import { showToast } from '../ui/toast.js';
import { renderMonitorGrid } from '../components/monitor-grid.js';

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

  function aplicarMapa(mapaTasas) {
    state.borradorTasas = fusionarTasas(mapaTasas);
    renderMonitorGrid(grid, state.borradorTasas, actualizarValor);
  }

  function actualizarEtiquetaLote() {
    if (etiquetaLote) etiquetaLote.textContent = `Lote en Prod: ${state.loteActualCodigo}`;
  }

  async function refresh() {
    try {
      const datos = await api.monitor();
      state.loteActualCodigo = datos.id_tasa || 'T000';
      actualizarEtiquetaLote();
      aplicarMapa(datos.tasas || {});
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
    showToast('⚡ Despachando lote a socios en ON...');
    try {
      const datos = await api.publicarLote(state.modoPruebaGlobal);
      showToast(`🚀 Publicado a ${datos.procesados} socios ON`);
    } catch (error) {
      showToast(`❌ Error al publicar: ${error.message}`);
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
      aplicarMapa({ ...state.borradorTasas, ...datos.borrador });
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
  alHacer('#btnHoo', 'click', traerBorradorHoo);
  alHacer('#btnRemesas', 'click', enviarTasaRemesas);
  alHacer('#btnBorrarLote', 'click', abrirBorrado);
  alHacer('#btnConfirmarBorrar', 'click', confirmarBorrado);
  alHacer('#globalTestSwitch', 'change', (evento) => cambiarModoPrueba(evento.target.checked));

  if (grid && !grid.childElementCount) {
    mostrarEstadoGrid(grid, 'Cargando tasas del lote vigente...');
  }

  return { refresh, aplicarMapa };
}
