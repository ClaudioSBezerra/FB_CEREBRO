package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestB64MatchesStandardEncoding(t *testing.T) {
	casos := []string{
		"",
		"a",
		"ab",
		"abc",
		"Farol — Objetivos por Indústria",
		"assunto com espaço e acentuação çãõ",
	}
	for _, s := range casos {
		got := b64(s)
		want := base64.StdEncoding.EncodeToString([]byte(s))
		if got != want {
			t.Errorf("b64(%q) = %q, want %q", s, got, want)
		}
	}
}

func TestBuildMIMEMessageHasBothParts(t *testing.T) {
	msg := buildMIMEMessage("de@x.com", []string{"para@x.com"}, "Assunto", "texto puro", "<b>html</b>")
	if !strings.Contains(msg, "Content-Type: text/plain") {
		t.Error("mensagem sem parte text/plain")
	}
	if !strings.Contains(msg, "Content-Type: text/html") {
		t.Error("mensagem sem parte text/html")
	}
	if !strings.Contains(msg, "texto puro") {
		t.Error("corpo texto ausente")
	}
	if !strings.Contains(msg, "<b>html</b>") {
		t.Error("corpo html ausente")
	}
	if !strings.Contains(msg, "To: para@x.com") {
		t.Error("destinatário ausente no cabeçalho")
	}
}

func TestSendHTMLReportSemDestinatario(t *testing.T) {
	if err := sendHTMLReport(nil, "a", "b", "c"); err == nil {
		t.Error("esperava erro com destinatário vazio")
	}
}

func TestSendHTMLReportSemSMTPConfigurado(t *testing.T) {
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_PASSWORD", "")
	err := sendHTMLReport([]string{"x@x.com"}, "a", "b", "c")
	if err == nil {
		t.Fatal("esperava erro sem SMTP_USER/SMTP_PASSWORD")
	}
	if !strings.Contains(err.Error(), "SMTP não configurado") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
}
