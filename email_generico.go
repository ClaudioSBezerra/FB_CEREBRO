package main

// email_generico.go — endpoint genérico de envio de e-mail, pra agentes de
// IA que não têm um painel FB específico (ex: "Monitor do CEO" mandando
// resumo diário de notícias de investimento). Reaproveita o mesmo SMTP já
// validado nos e-mails de painel (email.go) em vez de cada agente ter que
// resolver entrega de e-mail por conta própria — e evita o mecanismo de
// anexo do Paperclip, que já se mostrou instável (ver AD-7 da espinha).
//
// Allowlist por variável de ambiente (EMAILS_PERMITIDOS, separado por
// vírgula): o token que autentica este endpoint é o mesmo compartilhado
// entre vários agentes Paperclip — sem essa trava, um token vazado vira
// relay de e-mail pra qualquer destinatário.

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type enviarEmailReq struct {
	Email      string `json:"email"`
	Assunto    string `json:"assunto"`
	CorpoHTML  string `json:"corpo_html"`
	CorpoTexto string `json:"corpo_texto"`
}

func emailsPermitidos() []string {
	raw := os.Getenv("EMAILS_PERMITIDOS")
	if raw == "" {
		return nil
	}
	var out []string
	for _, e := range strings.Split(raw, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

func emailPermitido(email string, permitidos []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, p := range permitidos {
		if p == email {
			return true
		}
	}
	return false
}

// enviarEmailHandler aceita POST {email, assunto, corpo_html, corpo_texto}
// — diferente das rotas *-email de painel (que têm formato fixo), esta é
// de propósito geral: o conteúdo é montado pelo próprio agente chamador.
func enviarEmailHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONErro(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req enviarEmailReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErro(w, http.StatusBadRequest, "corpo inválido: "+err.Error())
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Assunto = strings.TrimSpace(req.Assunto)
	if req.Email == "" || req.Assunto == "" || req.CorpoHTML == "" {
		writeJSONErro(w, http.StatusBadRequest, "campos obrigatórios: email, assunto, corpo_html")
		return
	}
	permitidos := emailsPermitidos()
	if len(permitidos) == 0 {
		writeJSONErro(w, http.StatusServiceUnavailable, "EMAILS_PERMITIDOS não configurado neste serviço")
		return
	}
	if !emailPermitido(req.Email, permitidos) {
		writeJSONErro(w, http.StatusForbidden, "destinatário não autorizado")
		return
	}
	corpoTexto := req.CorpoTexto
	if corpoTexto == "" {
		corpoTexto = "Este e-mail contém conteúdo em HTML. Abra em um cliente compatível."
	}
	if err := sendHTMLReport([]string{req.Email}, req.Assunto, corpoTexto, req.CorpoHTML); err != nil {
		writeJSONErro(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"enviado": true, "para": req.Email})
}
