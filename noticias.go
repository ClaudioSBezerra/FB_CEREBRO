package main

// noticias.go — busca de notícias reais via API REST do Tavily (sem MCP).
// O Paperclip não expõe (nessa instância) a tela de governança de MCP
// (Applications/Connections/Profiles) pra dar acesso de busca real ao
// agente "Monitor do CEO", e a busca nativa do adapter não trouxe
// resultado usável no sandbox dele. Este serviço já tem rede de saída
// confiável (mesma usada pro SMTP), então faz a busca aqui — o agente só
// recebe os resultados crus (título, URL, trecho, data) e resume a partir
// deles, nunca inventando o que não veio nesta resposta.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type noticiaResultado struct {
	Titulo      string `json:"titulo"`
	URL         string `json:"url"`
	Resumo      string `json:"resumo"`
	PublicadoEm string `json:"publicado_em,omitempty"`
}

type noticiasResp struct {
	Consulta   string             `json:"consulta"`
	Resultados []noticiaResultado `json:"resultados"`
}

type tavilyReq struct {
	Query      string `json:"query"`
	Topic      string `json:"topic"`
	TimeRange  string `json:"time_range"`
	MaxResults int    `json:"max_results"`
}

type tavilyResultado struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	Content       string `json:"content"`
	PublishedDate string `json:"published_date"`
}

type tavilyResp struct {
	Results []tavilyResultado `json:"results"`
}

// buscarNoticiasTavily chama a API REST do Tavily (api.tavily.com/search,
// não é MCP). Falha explícita e sem tentar rede se a key não estiver
// configurada — mesma convenção do sendHTMLReport em email.go.
func buscarNoticiasTavily(query string, maxResults int) (*tavilyResp, error) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY não configurada neste serviço")
	}
	body, err := json.Marshal(tavilyReq{
		Query:      query,
		Topic:      "news",
		TimeRange:  "day",
		MaxResults: maxResults,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro chamando Tavily: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily retornou %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var tr tavilyResp
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("erro decodificando resposta do Tavily: %w", err)
	}
	return &tr, nil
}

const consultaPadraoNoticias = "notícias investimentos mercado financeiro Brasil e mundo"

func noticiasInvestimentoHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = consultaPadraoNoticias
	}
	maxResults := 10
	if v := r.URL.Query().Get("max_results"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 20 {
			maxResults = n
		}
	}
	tr, err := buscarNoticiasTavily(query, maxResults)
	if err != nil {
		writeJSONErro(w, http.StatusBadGateway, err.Error())
		return
	}
	out := noticiasResp{Consulta: query, Resultados: []noticiaResultado{}}
	for _, res := range tr.Results {
		out.Resultados = append(out.Resultados, noticiaResultado{
			Titulo:      res.Title,
			URL:         res.URL,
			Resumo:      res.Content,
			PublicadoEm: res.PublishedDate,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
