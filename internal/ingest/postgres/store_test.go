package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"

	"hooty/internal/domain"
)

func validComp() domain.Comprobante {
	return domain.Comprobante{
		HashLargo:      "8a3ef79788979e878129e00ded237dcd",
		HashCorto:      "8A3EF797",
		Instancia:      "hooty",
		MontoRecibo:    100,
		MonedaRecibo:   "USD",
		Banco:          "BANCO",
		Titular:        "TITULAR",
		Referencia:     "REF",
		URLImagen:      "https://cdn.example/imagen.jpg",
		Grupo1:         "120363000000000001@g.us",
		Grupo2:         "120363000000000002@g.us",
		Caption1:       "uno",
		Caption2:       "dos",
		Socio1:         "CHASAN",
		Socio2:         "NELSY",
		Tasa1:          2.5,
		Tasa2:          3.5,
		Monto1:         40,
		Monto2:         28.57,
		ME1:            100,
		ME2:            100,
		LoteTasa:       "T052",
		FechaOperacion: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
		CreadoEn:       time.Date(2026, 10, 25, 10, 1, 0, 0, time.UTC),
		ActualizadoEn:  time.Date(2026, 10, 25, 10, 1, 0, 0, time.UTC),
	}
}

func TestGuardarComprobanteYMarcarExigeDosFilas(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()
	store, err := New(mock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO comprobantes").
		WithArgs(anyArgs(26)...).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mock.ExpectExec("UPDATE impactos").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mock.ExpectRollback()

	err = store.GuardarComprobanteYMarcar(context.Background(), validComp(), []int64{1, 2})
	if !errors.Is(err, ErrPairNotReserved) {
		t.Fatalf("se esperaba ErrPairNotReserved, obtuve %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func anyArgs(count int) []any {
	args := make([]any, count)
	for index := range args {
		args[index] = pgxmock.AnyArg()
	}
	return args
}

func TestGuardarComprobanteYMarcarCommitConDosFilas(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()
	store, err := New(mock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO comprobantes").
		WithArgs(anyArgs(26)...).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mock.ExpectExec("UPDATE impactos").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 2"))
	mock.ExpectCommit()

	if err := store.GuardarComprobanteYMarcar(context.Background(), validComp(), []int64{1, 2}); err != nil {
		t.Fatalf("GuardarComprobanteYMarcar: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReleasePairLimpiaReservaNoProcesada(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()
	store, err := New(mock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE impactos").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 2"))
	mock.ExpectCommit()

	if err := store.ReleasePair(context.Background(), []int64{1, 2}); err != nil {
		t.Fatalf("ReleasePair: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrarImpactoReintentaParLiberado(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()
	store, err := New(mock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	received := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	impacto := domain.Impacto{
		HashLargo:  "hash",
		HashCorto:  "HASH",
		IDGrupo:    "222@g.us",
		Instancia:  "hooty",
		RecibidoEn: received,
	}
	columns := []string{"id", "hash_largo", "hash_corto", "id_grupo", "caption", "instancia", "procesado", "recibido_en", "pareja_id"}

	mock.ExpectBegin()
	mock.ExpectExec("pg_advisory_xact_lock").
		WithArgs("hooty:hash").
		WillReturnResult(pgconn.NewCommandTag("SELECT 1"))
	mock.ExpectQuery("INSERT INTO impactos").
		WithArgs(anyArgs(6)...).
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("SELECT id, hash_largo").
		WithArgs("hash", "222@g.us").
		WillReturnRows(pgxmock.NewRows(columns).
			AddRow(int64(2), "hash", "HASH", "222@g.us", "segundo", "hooty", false, received, nil))
	mock.ExpectQuery("FOR UPDATE SKIP LOCKED").
		WithArgs("hash", "222@g.us").
		WillReturnRows(pgxmock.NewRows(columns).
			AddRow(int64(1), "hash", "HASH", "111@g.us", "primero", "hooty", false, received.Add(-time.Minute), nil))
	mock.ExpectExec("UPDATE impactos SET pareja_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mock.ExpectExec("UPDATE impactos SET pareja_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))
	mock.ExpectCommit()

	registro, err := store.RegistrarImpacto(context.Background(), impacto)
	if err != nil {
		t.Fatalf("RegistrarImpacto: %v", err)
	}
	if registro.Status != domain.StatusPairReady || len(registro.Pair) != 2 {
		t.Fatalf("registro inesperado: %+v", registro)
	}
	if registro.Pair[0].ID != 1 || registro.Pair[1].ID != 2 {
		t.Fatalf("orden del par = %d,%d", registro.Pair[0].ID, registro.Pair[1].ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuardarComprobanteRechazaIDsInvalidos(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()
	store, err := New(mock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := store.GuardarComprobanteYMarcar(context.Background(), validComp(), []int64{1, 1}); err == nil {
		t.Fatal("ids repetidos deben fallar antes de abrir transacción")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
