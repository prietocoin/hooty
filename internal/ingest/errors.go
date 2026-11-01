package ingest

import "errors"

// ErrManualType evita que un reprocesamiento sobrescriba una edición manual.
var ErrManualType = errors.New("ingest: el comprobante tiene tipo de operación manual")

// ErrComprobanteNotFound indica que el hash no existe en comprobantes.
var ErrComprobanteNotFound = errors.New("ingest: comprobante no encontrado")
