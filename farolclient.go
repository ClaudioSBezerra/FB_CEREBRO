package main

// farolclient.go — cliente HTTP fino pro facade do FB_FAROL (AD-3 da
// espinha de arquitetura: Objetivos por Indústria é dado do FB_FAROL,
// este serviço NUNCA recalcula isso, só repassa a chamada). Token próprio
// (FAROL_GATEWAY_TOKEN), distinto do token que os agentes usam pra chamar
// o FB_FAROL direto (AD-6 — revogar um não derruba o outro).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type farolClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newFarolClient() (*farolClient, error) {
	base := strings.TrimSpace(os.Getenv("FAROL_BASE_URL"))
	if base == "" {
		base = "https://farol.fbtax.cloud"
	}
	token := strings.TrimSpace(os.Getenv("FAROL_GATEWAY_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("FAROL_GATEWAY_TOKEN não configurado")
	}
	return &farolClient{
		baseURL: strings.TrimRight(base, "/"),
		token:   token,
		http:    &http.Client{Timeout: 20 * time.Second},
	}, nil
}

// enviarObjetivosIndustriaEmail repassa a chamada pro
// GET /api/farol-jc/objetivos-industria-email do FB_FAROL — mesmos
// parâmetros, sem tocar em cálculo nenhum aqui (AD-3).
func (c *farolClient) enviarObjetivosIndustriaEmail(industria, periodo, fluxo, email string) (map[string]any, error) {
	q := url.Values{}
	q.Set("industria", industria)
	q.Set("periodo", periodo)
	q.Set("email", email)
	if fluxo != "" {
		q.Set("fluxo", fluxo)
	}
	reqURL := c.baseURL + "/api/farol-jc/objetivos-industria-email?" + q.Encode()

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro chamando FB_FAROL: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro lendo resposta do FB_FAROL: %w", err)
	}

	var out map[string]any
	if jsonErr := json.Unmarshal(body, &out); jsonErr != nil {
		return nil, fmt.Errorf("FB_FAROL devolveu corpo não-JSON (status %d): %s", resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		if msg, ok := out["error"].(string); ok {
			return nil, fmt.Errorf("FB_FAROL: %s", msg)
		}
		return nil, fmt.Errorf("FB_FAROL devolveu status %d", resp.StatusCode)
	}
	return out, nil
}

// industrias/objetivosIndustria/comparativoFechamento — proxy puro
// (status+corpo crus) pras rotas de leitura do FB_FAROL, migradas pra cá
// em 17/09/2026 (plano combinado de 16/09: FB_CEREBRO vira o único
// gateway que os agentes de IA chamam — AD-2 da espinha — em vez de
// FAROL_MCP_TOKEN espalhado em cada agente). Devolve o corpo exatamente
// como o FAROL respondeu (inclusive em erro), sem reformatar — quem
// decide o formato de erro é o FAROL, este serviço só repassa.
func (c *farolClient) industrias() (status int, body []byte, err error) {
	return proxyGet(c.http, c.baseURL+"/api/farol-jc/industrias", "Authorization", "Bearer "+c.token)
}

func (c *farolClient) objetivosIndustria(industria, periodo, fluxo string) (status int, body []byte, err error) {
	q := url.Values{}
	q.Set("industria", industria)
	q.Set("periodo", periodo)
	if fluxo != "" {
		q.Set("fluxo", fluxo)
	}
	reqURL := c.baseURL + "/api/farol-jc/objetivos-industria?" + q.Encode()
	return proxyGet(c.http, reqURL, "Authorization", "Bearer "+c.token)
}

func (c *farolClient) comparativoFechamento(industria, periodo string) (status int, body []byte, err error) {
	q := url.Values{}
	q.Set("industria", industria)
	q.Set("periodo", periodo)
	reqURL := c.baseURL + "/api/farol-jc/comparativo-fechamento?" + q.Encode()
	return proxyGet(c.http, reqURL, "Authorization", "Bearer "+c.token)
}
