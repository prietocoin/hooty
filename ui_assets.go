// Package hooty expone los assets embebidos del frontend.
//
// La UI vive en el directorio ui/ del repositorio y se compila dentro del
// binario con //go:embed. El binario es autocontenido: no necesita un volumen
// ni un servidor estático externo en producción.
package hooty

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:ui
var uiAssets embed.FS

// UIHandler devuelve el servidor de archivos de la UI embebida.
//
// El handler se monta en "/" y queda detrás de las rutas más específicas de
// healthz, webhooks y /api/v1/*. La raíz redirige a /comprobantes/.
func UIHandler() (http.Handler, error) {
	sub, err := fs.Sub(uiAssets, "ui")
	if err != nil {
		return nil, err
	}
	return &staticUI{
		fsys:       http.FS(sub),
		fileServer: http.FileServer(http.FS(sub)),
	}, nil
}

type staticUI struct {
	fsys       http.FileSystem
	fileServer http.Handler
}

func (h *staticUI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path == "/" {
		http.Redirect(w, r, "/comprobantes/", http.StatusFound)
		return
	}

	// Los documentos HTML no se cachean para que un despliegue nuevo se vea de
	// inmediato; los assets llevan ?v= y pueden cachearse.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasSuffix(r.URL.Path, "/") || strings.HasSuffix(r.URL.Path, ".html") {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}

	h.fileServer.ServeHTTP(w, r)
}
