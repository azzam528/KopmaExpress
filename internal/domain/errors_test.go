package domain

import (
	"net/http"
	"testing"
)

func TestAppErrors(t *testing.T) {
	// 1. NotFoundError
	errNF := NewNotFoundError("Barang tidak ditemukan")
	if errNF.HTTPStatus != http.StatusNotFound || errNF.Code != CodeNotFound {
		t.Errorf("unexpected not found error: %+v", errNF)
	}

	// 2. DuplicateError
	errDup := NewDuplicateError("Kode barang sudah terdaftar")
	if errDup.HTTPStatus != http.StatusConflict || errDup.Code != CodeDuplicate {
		t.Errorf("unexpected duplicate error: %+v", errDup)
	}

	// 3. ValidationError
	errVal := NewValidationError("Validasi gagal", ValidationErrorDetail{
		Field:   "harga",
		Message: "harus >= 0",
	})
	if errVal.HTTPStatus != http.StatusUnprocessableEntity || errVal.Code != CodeValidationError {
		t.Errorf("unexpected validation error: %+v", errVal)
	}
	details, ok := errVal.Details.([]ValidationErrorDetail)
	if !ok || len(details) != 1 || details[0].Field != "harga" {
		t.Errorf("unexpected validation details: %+v", errVal.Details)
	}

	// 4. InsufficientStockError
	errStock := NewInsufficientStockError("Stok tidak mencukupi", InsufficientStockDetail{
		ProductCode: "BRG-0001",
		Requested:   10,
		Available:   3,
	})
	if errStock.HTTPStatus != http.StatusConflict || errStock.Code != CodeInsufficientStock {
		t.Errorf("unexpected stock error: %+v", errStock)
	}
}
