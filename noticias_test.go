package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuscarNoticiasTavilySemAPIKey(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	_, err := buscarNoticiasTavily("qualquer coisa", 5)
	if err == nil {
		t.Fatal("esperava erro sem TAVILY_API_KEY configurada")
	}
	if !strings.Contains(err.Error(), "TAVILY_API_KEY") {
		t.Errorf("erro = %q, esperava menção a TAVILY_API_KEY", err.Error())
	}
}

func TestNoticiasInvestimentoHandlerSemAuth(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	r := httptest.NewRequest("GET", "/noticias-investimento", nil)
	rec := httptest.NewRecorder()
	noticiasInvestimentoHandler(rec, r)
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestNoticiasInvestimentoHandlerSemTavilyKey(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("TAVILY_API_KEY", "")
	r := httptest.NewRequest("GET", "/noticias-investimento", nil)
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	noticiasInvestimentoHandler(rec, r)
	// autorizado, mas sem TAVILY_API_KEY o handler falha ANTES de tentar
	// rede — prova que a validação chega até buscarNoticiasTavily sem
	// dial nenhum (AD-9: sem rede real em teste).
	if rec.Code != 502 {
		t.Errorf("status = %d, want 502 (TAVILY_API_KEY ausente)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "TAVILY_API_KEY") {
		t.Errorf("body = %q, esperava menção a TAVILY_API_KEY", rec.Body.String())
	}
}

func TestNoticiasInvestimentoHandlerMaxResultsInvalidoUsaDefault(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("TAVILY_API_KEY", "")
	r := httptest.NewRequest("GET", "/noticias-investimento?max_results=abc&query=teste", nil)
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	// não crasha com max_results inválido — só cai no mesmo erro de
	// TAVILY_API_KEY ausente (prova que o parsing não quebrou antes disso).
	noticiasInvestimentoHandler(rec, r)
	if rec.Code != 502 {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}
