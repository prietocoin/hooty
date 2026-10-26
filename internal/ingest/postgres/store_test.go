package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

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
		WithArgs(anyArgs(25)...).
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
		WithArgs(anyArgs(25)...).
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
