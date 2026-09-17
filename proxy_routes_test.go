package main

// proxy_routes_test.go — testa as rotas de proxy pro FAROL/SmartPick sem
// rede real (AD-9): só os caminhos de validação que nunca chegam a
// discar uma conexão (auth, parâmetro obrigatório, token não configurado).

import (
	"net/http/httptest"
	"testing"
)

func TestIndustriasHandlerSemAuth(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	r := httptest.NewRequest("GET", "/industrias", nil)
	rec := httptest.NewRecorder()
	industriasHandler(rec, r)
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestIndustriasHandlerSemFarolGatewayToken(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("FAROL_GATEWAY_TOKEN", "")
	r := httptest.NewRequest("GET", "/industrias", nil)
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	industriasHandler(rec, r)
	if rec.Code != 500 {
		t.Errorf("status = %d, want 500 (FAROL_GATEWAY_TOKEN ausente)", rec.Code)
	}
}

func TestObjetivosIndustriaHandlerFaltamParametros(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	r := httptest.NewRequest("GET", "/objetivos-industria?industria=UNILEVER+HC", nil)
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	objetivosIndustriaHandler(rec, r)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 (falta periodo)", rec.Code)
	}
}

func TestComparativoFechamentoHandlerFaltamParametros(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	r := httptest.NewRequest("GET", "/comparativo-fechamento", nil)
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	comparativoFechamentoHandler(rec, r)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHistoricoCalibragemHandlerSemCdID(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	r := httptest.NewRequest("GET", "/historico-calibragem", nil)
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	historicoCalibragemHandler(rec, r)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 (falta cd_id)", rec.Code)
	}
}

func TestHistoricoCalibragemHandlerSemSmartPickGatewayToken(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("SMARTPICK_GATEWAY_TOKEN", "")
	r := httptest.NewRequest("GET", "/historico-calibragem?cd_id=2", nil)
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	historicoCalibragemHandler(rec, r)
	if rec.Code != 500 {
		t.Errorf("status = %d, want 500 (SMARTPICK_GATEWAY_TOKEN ausente)", rec.Code)
	}
}
