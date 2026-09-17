package main

// smartpickclient.go — cliente HTTP fino pro facade do FB_SMARTPICK
// (mesmo racional do farolclient.go: AD-2/AD-3 da espinha — FB_CEREBRO é
// o único gateway multi-módulo, nunca recalcula, só repassa). Criado em
// 17/09/2026 junto com a migração combinada em 16/09 (o Painel Executivo
// chamava o SmartPick direto com SMARTPICK_API_KEY — agora só o
// FB_CEREBRO tem esse token).

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type smartPickClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newSmartPickClient() (*smartPickClient, error) {
	base := strings.TrimSpace(os.Getenv("SMARTPICK_BASE_URL"))
	if base == "" {
		base = "https://smartpick.fbtax.cloud"
	}
	token := strings.TrimSpace(os.Getenv("SMARTPICK_GATEWAY_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("SMARTPICK_GATEWAY_TOKEN não configurado")
	}
	return &smartPickClient{
		baseURL: strings.TrimRight(base, "/"),
		token:   token,
		http:    &http.Client{Timeout: 20 * time.Second},
	}, nil
}

// historicoCalibragem repassa pro GET /api/relatorios/historico-calibragem
// do FB_SMARTPICK — proxy puro (status+corpo crus), sem reformatar nada.
func (c *smartPickClient) historicoCalibragem(cdID, anoInicio string) (status int, body []byte, err error) {
	q := url.Values{}
	q.Set("cd_id", cdID)
	if anoInicio != "" {
		q.Set("ano_inicio", anoInicio)
	}
	reqURL := c.baseURL + "/api/relatorios/historico-calibragem?" + q.Encode()
	return proxyGet(c.http, reqURL, "X-API-Key", c.token)
}

// centrosDistribuicao repassa pro GET /api/relatorios/centros-distribuicao
// do FB_SMARTPICK — lista de CDs (nome → cd_id) pra quem for chamar
// historicoCalibragem não precisar adivinhar o cd_id numérico.
func (c *smartPickClient) centrosDistribuicao() (status int, body []byte, err error) {
	reqURL := c.baseURL + "/api/relatorios/centros-distribuicao"
	return proxyGet(c.http, reqURL, "X-API-Key", c.token)
}
