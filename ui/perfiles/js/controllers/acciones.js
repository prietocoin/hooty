import { qs } from '../../../shared/js/dom.js';
import { api } from '../api.js?v=20261031_03';

export function crearAcciones({
  recargar = async () => {},
  editar = () => {},
  confirmar = globalThis.confirm,
  alertar = globalThis.alert
} = {}) {
  let montado = false;

  async function apagarTodos() {
    if (!confirmar('¿Deseas apagar las tasas de todos los socios excepto FUNDDA?')) return;
    try {
      await api.apagarTodos();
      await recargar();
    } catch {
      alertar('Error al apagar todos');
    }
  }

  async function memorizarPlantilla() {
    try {
      const data = await api.memorizar();
      alertar(data?.message || 'Plantilla memorizada');
    } catch {
      alertar('Error guardando plantilla');
    }
  }

  async function restaurarPlantilla() {
    if (!confirmar('¿Restaurar la última plantilla de socios activos?')) return;
    try {
      const data = await api.restaurar();
      alertar(data?.message || 'Plantilla restaurada');
      await recargar();
    } catch {
      alertar('Error restaurando plantilla');
    }
  }

  async function notificarFundda() {
    try {
      await api.notificarTasas();
      mostrarToast('✅ Trigger de tasas enviado a n8n');
      await recargar();
    } catch (error) {
      mostrarToast('❌ No se pudo disparar el trigger de tasas', true);
    }
  }

  let temporizadorToast = null;
  function mostrarToast(mensaje, esError = false) {
    let toast = document.getElementById('perfiles-toast');
    if (!toast) {
      toast = document.createElement('div');
      toast.id = 'perfiles-toast';
      toast.style.cssText = 'position:fixed;left:50%;bottom:20px;transform:translateX(-50%);z-index:10000;padding:10px 16px;border-radius:12px;font-size:12px;font-weight:700;color:#f8fafc;box-shadow:0 10px 30px rgba(0,0,0,.35);transition:opacity .2s;';
      document.body.append(toast);
    }
    toast.textContent = mensaje;
    toast.style.background = esError ? 'rgba(127,29,29,.95)' : 'rgba(6,78,59,.95)';
    toast.style.border = esError ? '1px solid rgba(248,113,113,.5)' : '1px solid rgba(52,211,153,.5)';
    toast.style.opacity = '1';
    if (temporizadorToast) clearTimeout(temporizadorToast);
    temporizadorToast = setTimeout(() => { toast.style.opacity = '0'; }, 2600);
  }

  function mount() {
    if (montado) return;
    montado = true;
    qs('#btn-refrescar')?.addEventListener('click', () => { void notificarFundda(); });
    qs('#btn-apagar-todos')?.addEventListener('click', () => { void apagarTodos(); });
    qs('#btn-memorizar')?.addEventListener('click', () => { void memorizarPlantilla(); });
    qs('#btn-restaurar')?.addEventListener('click', () => { void restaurarPlantilla(); });
    qs('#btn-nuevo')?.addEventListener('click', () => editar(null));
  }

  return { mount };
}
