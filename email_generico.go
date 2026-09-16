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
// relay de e-mail pra qualquer destinatário. Pra não ficar engessado
// exigindo redeploy a cada pessoa nova da empresa (decisão do Claudio,
// 16/09/2026): uma entrada começando com "@" libera o DOMÍNIO inteiro
// (ex: "@ferreiracosta.com.br" aceita qualquer endereço desse domínio,
// sem precisar listar cada pessoa) — e-mails pessoais (gmail, hotmail)
// continuam precisando estar na lista exata, já que não dá pra confiar
// no domínio de um provedor público.

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

// emailPermitido aceita dois formatos de entrada na allowlist: e-mail exato
// ("fulano@x.com") ou domínio inteiro prefixado com "@" ("@x.com", libera
// qualquer endereço desse domínio).
func emailPermitido(email string, permitidos []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, p := range permitidos {
		if strings.HasPrefix(p, "@") {
			if strings.HasSuffix(email, p) {
				return true
			}
			continue
		}
		if p == email {
			return true
		}
	}
	return false
}

// parseDestinatarios aceita 1+ e-mails separados por vírgula no mesmo campo
// `email` (ex: "a@x.com,b@y.com") — permite mandar o mesmo resumo pra vários
// destinatários numa só chamada, sem mudar o formato do request. Remove
// espaços e duplicatas (case-insensitive), preservando a ordem de entrada.
func parseDestinatarios(campo string) []string {
	vistos := map[string]bool{}
	var out []string
	for _, e := range strings.Split(campo, ",") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		chave := strings.ToLower(e)
		if vistos[chave] {
			continue
		}
		vistos[chave] = true
		out = append(out, e)
	}
	return out
}

// destinatariosNaoAutorizados devolve, da lista pedida, só os que NÃO estão
// na allowlist — usado pra rejeitar a chamada inteira (nunca manda só pra
// parte autorizada e ignora o resto em silêncio) e dizer exatamente qual
// endereço bloqueou.
func destinatariosNaoAutorizados(destinatarios, permitidos []string) []string {
	var naoAutorizados []string
	for _, d := range destinatarios {
		if !emailPermitido(d, permitidos) {
			naoAutorizados = append(naoAutorizados, d)
		}
	}
	return naoAutorizados
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
	req.Assunto = strings.TrimSpace(req.Assunto)
	destinatarios := parseDestinatarios(req.Email)
	if len(destinatarios) == 0 || req.Assunto == "" || req.CorpoHTML == "" {
		writeJSONErro(w, http.StatusBadRequest, "campos obrigatórios: email, assunto, corpo_html")
		return
	}
	permitidos := emailsPermitidos()
	if len(permitidos) == 0 {
		writeJSONErro(w, http.StatusServiceUnavailable, "EMAILS_PERMITIDOS não configurado neste serviço")
		return
	}
	if bloqueados := destinatariosNaoAutorizados(destinatarios, permitidos); len(bloqueados) > 0 {
		writeJSONErro(w, http.StatusForbidden, "destinatário não autorizado: "+strings.Join(bloqueados, ", "))
		return
	}
	corpoTexto := req.CorpoTexto
	if corpoTexto == "" {
		corpoTexto = "Este e-mail contém conteúdo em HTML. Abra em um cliente compatível."
	}
	if err := sendHTMLReport(destinatarios, req.Assunto, corpoTexto, req.CorpoHTML); err != nil {
		writeJSONErro(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"enviado": true, "para": destinatarios})
}
