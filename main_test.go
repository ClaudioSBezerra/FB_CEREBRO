package main

import (
	"net/http/httptest"
	"testing"
)

func TestLimiteDaQuery(t *testing.T) {
	casos := []struct {
		url  string
		want int
	}{
		{"/x", 100},
		{"/x?limit=5", 5},
		{"/x?limit=0", 100},
		{"/x?limit=abc", 100},
		{"/x?all=true", 1_000_000},
		{"/x?limit=5&all=true", 1_000_000},
	}
	for _, c := range casos {
		r := httptest.NewRequest("GET", c.url, nil)
		got := limiteDaQuery(r, 100, 1_000_000)
		if got != c.want {
			t.Errorf("limiteDaQuery(%q) = %d, want %d", c.url, got, c.want)
		}
	}
}

func TestResolverAnosELimitPadrao(t *testing.T) {
	r := httptest.NewRequest("GET", "/x", nil)
	anoAnterior, anoAtual, mesLimite, limit := resolverAnosELimit(r)
	if anoAtual-anoAnterior != 1 {
		t.Errorf("ano_atual - ano_anterior = %d, want 1", anoAtual-anoAnterior)
	}
	if limit != 200 {
		t.Errorf("limit padrão = %d, want 200", limit)
	}
	if mesLimite < 1 || mesLimite > 12 {
		t.Errorf("mesLimite = %d, want 1..12", mesLimite)
	}
}

func TestResolverAnosELimitExplicito(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?ano_anterior=2024&ano_atual=2026&all=true", nil)
	anoAnterior, anoAtual, _, limit := resolverAnosELimit(r)
	if anoAnterior != 2024 || anoAtual != 2026 {
		t.Errorf("anos = %d,%d, want 2024,2026", anoAnterior, anoAtual)
	}
	if limit != 100_000 {
		t.Errorf("limit com all=true = %d, want 100000", limit)
	}
}

func TestAuthOK(t *testing.T) {
	t.Setenv("API_TOKEN", "abc123")
	ok := httptest.NewRequest("GET", "/x", nil)
	ok.Header.Set("Authorization", "Bearer abc123")
	if !authOK(ok) {
		t.Error("esperava autorizado com token correto")
	}

	semToken := httptest.NewRequest("GET", "/x", nil)
	if authOK(semToken) {
		t.Error("esperava NÃO autorizado sem header")
	}

	tokenErrado := httptest.NewRequest("GET", "/x", nil)
	tokenErrado.Header.Set("Authorization", "Bearer errado")
	if authOK(tokenErrado) {
		t.Error("esperava NÃO autorizado com token errado")
	}
}

func TestAuthOKSemAPITokenConfigurado(t *testing.T) {
	t.Setenv("API_TOKEN", "")
	r := httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("Authorization", "Bearer ")
	if authOK(r) {
		t.Error("sem API_TOKEN configurado, nenhuma request deveria autorizar")
	}
}
