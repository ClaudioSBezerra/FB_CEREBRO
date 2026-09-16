package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmailsPermitidos(t *testing.T) {
	t.Setenv("EMAILS_PERMITIDOS", "  Claudio@x.com , jose@y.com,, jose@y.com ")
	got := emailsPermitidos()
	want := []string{"claudio@x.com", "jose@y.com", "jose@y.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEmailsPermitidosVazio(t *testing.T) {
	t.Setenv("EMAILS_PERMITIDOS", "")
	if got := emailsPermitidos(); got != nil {
		t.Errorf("esperava nil, got %v", got)
	}
}

func TestEmailPermitidoCaseInsensitive(t *testing.T) {
	permitidos := []string{"claudio@x.com"}
	if !emailPermitido("Claudio@X.com", permitidos) {
		t.Error("esperava permitido (case-insensitive)")
	}
	if emailPermitido("outro@x.com", permitidos) {
		t.Error("esperava NÃO permitido")
	}
}

func TestEmailPermitidoPorDominio(t *testing.T) {
	permitidos := []string{"@ferreiracosta.com.br", "claudiosousadebezerra@gmail.com"}
	if !emailPermitido("qualquer.pessoa@ferreiracosta.com.br", permitidos) {
		t.Error("esperava permitido: qualquer endereço do domínio liberado")
	}
	if !emailPermitido("Outra.Pessoa@Ferreiracosta.com.br", permitidos) {
		t.Error("esperava permitido (case-insensitive) por domínio")
	}
	if !emailPermitido("claudiosousadebezerra@gmail.com", permitidos) {
		t.Error("esperava permitido: e-mail exato ainda funciona")
	}
	if emailPermitido("outro@gmail.com", permitidos) {
		t.Error("gmail.com não está liberado por domínio — só o endereço exato cadastrado")
	}
	if emailPermitido("x@naoferreiracosta.com.br", permitidos) {
		t.Error("domínio parecido não deveria bypassar o sufixo real")
	}
}

func TestEnviarEmailHandlerSemAuth(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	r := httptest.NewRequest("POST", "/enviar-email", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	enviarEmailHandler(rec, r)
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestEnviarEmailHandlerMetodoErrado(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	r := httptest.NewRequest("GET", "/enviar-email", nil)
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	enviarEmailHandler(rec, r)
	if rec.Code != 405 {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestEnviarEmailHandlerCamposObrigatorios(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	r := httptest.NewRequest("POST", "/enviar-email", strings.NewReader(`{"email":"x@y.com"}`))
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	enviarEmailHandler(rec, r)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 (faltam assunto/corpo_html)", rec.Code)
	}
}

func TestEnviarEmailHandlerSemAllowlistConfigurada(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("EMAILS_PERMITIDOS", "")
	body := `{"email":"x@y.com","assunto":"a","corpo_html":"<p>b</p>"}`
	r := httptest.NewRequest("POST", "/enviar-email", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	enviarEmailHandler(rec, r)
	if rec.Code != 503 {
		t.Errorf("status = %d, want 503 (EMAILS_PERMITIDOS ausente)", rec.Code)
	}
}

func TestEnviarEmailHandlerDestinatarioNaoAutorizado(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("EMAILS_PERMITIDOS", "permitido@x.com")
	body := `{"email":"outro@x.com","assunto":"a","corpo_html":"<p>b</p>"}`
	r := httptest.NewRequest("POST", "/enviar-email", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	enviarEmailHandler(rec, r)
	if rec.Code != 403 {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestParseDestinatariosMultiplosComEspacosEDuplicatas(t *testing.T) {
	got := parseDestinatarios(" a@x.com, B@Y.com ,a@x.com, ,b@y.com")
	want := []string{"a@x.com", "B@Y.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDestinatariosNaoAutorizadosIdentificaSoOsBloqueados(t *testing.T) {
	permitidos := []string{"a@x.com", "b@y.com"}
	bloqueados := destinatariosNaoAutorizados([]string{"a@x.com", "outro@z.com", "b@y.com"}, permitidos)
	if len(bloqueados) != 1 || bloqueados[0] != "outro@z.com" {
		t.Errorf("bloqueados = %v, want [outro@z.com]", bloqueados)
	}
}

func TestEnviarEmailHandlerMultiplosDestinatariosUmNaoAutorizadoRejeitaTudo(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("EMAILS_PERMITIDOS", "a@x.com,b@y.com")
	body := `{"email":"a@x.com,outro@z.com","assunto":"a","corpo_html":"<p>b</p>"}`
	r := httptest.NewRequest("POST", "/enviar-email", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	enviarEmailHandler(rec, r)
	if rec.Code != 403 {
		t.Errorf("status = %d, want 403 (rejeita a chamada inteira, não manda só pros autorizados)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "outro@z.com") {
		t.Errorf("body = %q, esperava citar o destinatário bloqueado", rec.Body.String())
	}
}

func TestEnviarEmailHandlerMultiplosDestinatariosTodosAutorizadosSemSMTP(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("EMAILS_PERMITIDOS", "a@x.com,b@y.com")
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_PASSWORD", "")
	body := `{"email":"a@x.com,b@y.com","assunto":"a","corpo_html":"<p>b</p>"}`
	r := httptest.NewRequest("POST", "/enviar-email", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	enviarEmailHandler(rec, r)
	// passa dos dois pela allowlist e só falha no SMTP (sem rede real em teste,
	// AD-9) — prova que a lista inteira foi validada antes de tentar enviar.
	if rec.Code != 502 {
		t.Errorf("status = %d, want 502 (SMTP não configurado)", rec.Code)
	}
}

func TestEnviarEmailHandlerAutorizadoSemSMTP(t *testing.T) {
	t.Setenv("API_TOKEN", "tok")
	t.Setenv("EMAILS_PERMITIDOS", "permitido@x.com")
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_PASSWORD", "")
	body := `{"email":"permitido@x.com","assunto":"a","corpo_html":"<p>b</p>"}`
	r := httptest.NewRequest("POST", "/enviar-email", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	enviarEmailHandler(rec, r)
	// destinatário passa na allowlist, mas sem SMTP configurado o envio real
	// falha — prova que a validação chegou até sendHTMLReport sem tentar rede.
	if rec.Code != 502 {
		t.Errorf("status = %d, want 502 (SMTP não configurado)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SMTP não configurado") {
		t.Errorf("body = %q, esperava menção a SMTP não configurado", rec.Body.String())
	}
}
