package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewFarolClientSemToken(t *testing.T) {
	t.Setenv("FAROL_GATEWAY_TOKEN", "")
	if _, err := newFarolClient(); err == nil {
		t.Fatal("esperava erro sem FAROL_GATEWAY_TOKEN")
	}
}

func TestEnviarObjetivosIndustriaEmailSucesso(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer segredo-gateway" {
			t.Errorf("Authorization = %q, want Bearer segredo-gateway", got)
		}
		if r.URL.Query().Get("industria") != "Unilever HC" {
			t.Errorf("industria = %q", r.URL.Query().Get("industria"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"enviado":true,"industria":"UNILEVER HC","periodo":"2026-08"}`))
	}))
	defer srv.Close()

	t.Setenv("FAROL_GATEWAY_TOKEN", "segredo-gateway")
	t.Setenv("FAROL_BASE_URL", srv.URL)

	client, err := newFarolClient()
	if err != nil {
		t.Fatalf("newFarolClient: %v", err)
	}
	out, err := client.enviarObjetivosIndustriaEmail("Unilever HC", "2026-08", "", "x@x.com")
	if err != nil {
		t.Fatalf("enviarObjetivosIndustriaEmail: %v", err)
	}
	if enviado, _ := out["enviado"].(bool); !enviado {
		t.Errorf("esperava enviado=true, veio %v", out)
	}
}

func TestEnviarObjetivosIndustriaEmailErroDoFarol(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"industria ambigua"}`))
	}))
	defer srv.Close()

	t.Setenv("FAROL_GATEWAY_TOKEN", "segredo-gateway")
	t.Setenv("FAROL_BASE_URL", srv.URL)

	client, _ := newFarolClient()
	_, err := client.enviarObjetivosIndustriaEmail("Unilever", "2026-08", "", "x@x.com")
	if err == nil {
		t.Fatal("esperava erro repassado do FB_FAROL")
	}
	if got := err.Error(); got != "FB_FAROL: industria ambigua" {
		t.Errorf("erro = %q", got)
	}
}
