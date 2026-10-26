package api

import "net/http"

// CORS envuelve el mux principal con los headers de lectura y resuelve los
// preflight sin registrar patrones de método en ServeMux.
//
// Go 1.22+ rechaza un patrón como "OPTIONS /api/v1/" cuando convive con rutas
// más específicas del mismo subárbol (por ejemplo /api/v1/webhooks/hooty).
// Responder OPTIONS en middleware evita esa ambigüedad y cubre todos los
// endpoints, incluidos los webhooks.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
