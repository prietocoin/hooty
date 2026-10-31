/**
 * @file directorioState.js
 * @description Estado y acciones del directorio de socios.
 */

import { obtenerDirectorioNormalizado } from '../services/directorioService.js';
import { alternarEstadoSocioWA } from '../services/socioEstadoService.js';
import { alternarHerenciaSocio } from '../services/socioHerenciaService.js';
import { prepararEdicionSocio, guardarConfiguracionSocio } from '../services/socioConfigModalService.js';
import { apiUrl } from '../../../../../shared/js/api-base.js';
import {
  apagarTodosLosSocios,
  memorizarSociosVigentes,
  restaurarSociosVigentes,
  eliminarSocioDelDirectorio
} from '../services/directorioAccionesService.js';
import { LISTA_MONEDAS_ACTIVAS, obtenerInfoMonedasMaestra } from '../../../shared/constants/listaMonedasActivas.js';
import { notificarSociosActualizados } from '../../../../../shared/js/sync.js';
import {
  construirCatalogoSocios,
  establecerCatalogoSocios
} from '../../comprobantes/services/comprobantesNaturalezaService.js';

export function createDirectorioState() {
  return {
    directorio: [],
    socios: [],
    busquedaDirectorio: '',
    socioConfigEdit: null,
    modalConfigSocioAbierto: false,
    modalCarteleraAbierto: false,
    carteleraPreviewUrl: '',
    carteleraPreviewNombre: '',
    listaMonedasActivas: LISTA_MONEDAS_ACTIVAS,
    infoMonedasMaestra: obtenerInfoMonedasMaestra(),

    get directorioFiltrado() {
      if (!this.busquedaDirectorio) return this.directorio;
      const q = this.busquedaDirectorio.toLowerCase();
      return this.directorio.filter(s =>
        (s.nombre && s.nombre.toLowerCase().includes(q)) ||
        (s.socio && String(s.socio).toLowerCase().includes(q))
      );
    },

    async cargarDirectorio() {
      this.directorio = await obtenerDirectorioNormalizado() || [];
      // El catálogo de naturaleza se reconstruye con cada lectura del
      // directorio: es la fuente de moneda_natural y tabla_monedas.
      establecerCatalogoSocios(construirCatalogoSocios(this.directorio));
    },
    async toggleEstadoSocio(socio) {
      await alternarEstadoSocioWA(socio);
      await this.cargarDirectorio();
    },
    async toggleHerenciaSocio(socio) {
      await alternarHerenciaSocio(socio);
      await this.cargarDirectorio();
    },
    async toggleDashSocio(socio) {
      const borrador = prepararEdicionSocio(socio, this.infoMonedasMaestra);
      borrador.mostrar_dashboard = !borrador.mostrar_dashboard;
      await guardarConfiguracionSocio(borrador);
      await this.cargarDirectorio();
    },
    async apagarTodosSocios() {
      await apagarTodosLosSocios();
      await this.cargarDirectorio();
    },
    async guardarVigentes() {
      await memorizarSociosVigentes();
    },
    async restaurarVigentes() {
      await restaurarSociosVigentes();
      await this.cargarDirectorio();
    },
    crearNuevoSocio() {
      this.socioConfigEdit = prepararEdicionSocio({
        nombre: '',
        rol: 'SOCIO',
        moneda_base: 'USDT',
        saldo_inicial: 0,
        mostrar: { tasas: true, dashboard: true },
        monedas: {}
      }, this.infoMonedasMaestra);
      this.modalConfigSocioAbierto = true;
    },
    async eliminarSocioDirectorio(nombre) {
      if (!await eliminarSocioDelDirectorio(nombre)) return;
      await this.cargarDirectorio();
    },
    abrirConfigSocio(socio) {
      this.socioConfigEdit = prepararEdicionSocio(socio, this.infoMonedasMaestra);
      this.modalConfigSocioAbierto = true;
    },
    async verCarteleraSocio(socio) {
      const nombre = String(socio?.nombre || socio?.nombre_socio || '').trim();
      if (!nombre) return;
      this.cerrarCarteleraSocio();
      this.carteleraPreviewNombre = nombre;
      this.modalCarteleraAbierto = true;
      try {
        const response = await fetch(apiUrl(`/api/v1/tasas/preview/${encodeURIComponent(nombre)}`), {
          cache: 'no-store'
        });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const blob = await response.blob();
        this.carteleraPreviewUrl = URL.createObjectURL(blob);
      } catch (error) {
        console.error('[directorio cartelera ❌]', error);
        this.modalCarteleraAbierto = false;
        alert('No se pudo generar la cartelera: ' + error.message);
      }
    },
    cerrarCarteleraSocio() {
      if (this.carteleraPreviewUrl) URL.revokeObjectURL(this.carteleraPreviewUrl);
      this.carteleraPreviewUrl = '';
      this.modalCarteleraAbierto = false;
    },
    async guardarConfigSocioModal() {
      await guardarConfiguracionSocio(this.socioConfigEdit);
      this.modalConfigSocioAbierto = false;
      // El guardado puede cambiar tipo/polar_D/polar_P: la proyección de esta
      // misma vista debe recalcularse, y las demás pestañas deben enterarse.
      await this.cargarDirectorio();
      if (this.cargarComprobantes) await this.cargarComprobantes();
      notificarSociosActualizados('comprobantes');
    }
  };
}
