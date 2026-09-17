package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "embed"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Cobertura struct {
	CodSupervisor  string `json:"cod_supervisor"`
	NomeSupervisor string `json:"nome_supervisor"`
	CodRca         string `json:"cod_rca"`
	NomeRca        string `json:"nome_rca"`
	Cnpj           string `json:"cnpj"`
	NomeCli        string `json:"nome_cli"`
	Fantasia       string `json:"fantasia"`
	UltimaVenda    string `json:"ultima_venda"`
	DiasSemComprar int    `json:"dias_sem_comprar"`
}

type Resumo struct {
	TotalClientes int    `json:"total_clientes"`
	TotalRcas     int    `json:"total_rcas"`
	TotalCritico  int    `json:"total_critico"`
	TotalAtencao  int    `json:"total_atencao"`
	AtualizadoEm  string `json:"atualizado_em,omitempty"`
}

type FornecedorRow struct {
	CodFornec      string  `json:"cod_fornec"`
	NomeFornec     string  `json:"nome_fornec"`
	VendaAnterior  float64 `json:"venda_anterior"`
	VendaAtual     float64 `json:"venda_atual"`
	PctVenda       int     `json:"pct_venda"`
	ClientesAtivos int     `json:"clientes_ativos"`
	PositAnterior  int     `json:"posit_anterior"`
	PositAtual     int     `json:"posit_atual"`
	PctPosAtual    int     `json:"pct_pos_atual"`
	MixMedio       float64 `json:"mix_medio"`
}

type FaturadoResp struct {
	AnoAnterior  int             `json:"ano_anterior"`
	AnoAtual     int             `json:"ano_atual"`
	Total        FornecedorRow   `json:"total"`
	Fornecedores []FornecedorRow `json:"fornecedores"`
}

//go:embed painel-fornecedor.html
var painelHTML []byte

var pool *pgxpool.Pool

const empresaJC = "ac91bee4-eb9c-4497-bf3b-7eea30e7d4fd"

const query = `
WITH movimento AS (
  SELECT v.cnpj, v.nome_cli, v.fantasia, v.cod_rca, v.nome_rca,
         v.cod_supervisor, v.nome_supervisor,
         v.data_faturamento AS data_evento
  FROM vendas_faturadas v
  WHERE v.qt > 0 AND v.empresa_id = $1::uuid
  UNION ALL
  SELECT v.cnpj, v.nome_cli, v.fantasia, v.cod_rca, v.nome_rca,
         v.cod_supervisor, v.nome_supervisor,
         v.data_transmissao AS data_evento
  FROM vendas_transmitidas v
  WHERE v.qt > 0 AND v.empresa_id = $1::uuid
),
carteira AS (
  SELECT cnpj, cod_rca,
         MAX(nome_cli) AS nome_cli, MAX(fantasia) AS fantasia,
         MAX(nome_rca) AS nome_rca, MAX(cod_supervisor) AS cod_supervisor,
         MAX(nome_supervisor) AS nome_supervisor,
         MAX(data_evento) AS ultima_venda
  FROM movimento
  GROUP BY cnpj, cod_rca
)
SELECT cod_supervisor, nome_supervisor, cod_rca, nome_rca,
       cnpj, nome_cli, fantasia, ultima_venda::text,
       (CURRENT_DATE - ultima_venda)::int AS dias_sem_comprar
FROM carteira
WHERE ultima_venda < CURRENT_DATE - 15
  AND ($2 = '' OR cod_supervisor = $2)
ORDER BY dias_sem_comprar DESC
LIMIT $3;
`

const resumoQuery = `
WITH movimento AS (
  SELECT v.cnpj, v.cod_rca,
         v.data_faturamento AS data_evento
  FROM vendas_faturadas v
  WHERE v.qt > 0 AND v.empresa_id = $1::uuid
  UNION ALL
  SELECT v.cnpj, v.cod_rca,
         v.data_transmissao AS data_evento
  FROM vendas_transmitidas v
  WHERE v.qt > 0 AND v.empresa_id = $1::uuid
),
carteira AS (
  SELECT cnpj, cod_rca, MAX(data_evento) AS ultima_venda
  FROM movimento
  GROUP BY cnpj, cod_rca
),
risco AS (
  SELECT cod_rca, (CURRENT_DATE - ultima_venda)::int AS dias
  FROM carteira
  WHERE ultima_venda < CURRENT_DATE - 15
)
SELECT
  COUNT(*)                                       AS total_clientes,
  COUNT(DISTINCT cod_rca)                        AS total_rcas,
  COUNT(*) FILTER (WHERE dias >= 25)             AS total_critico,
  COUNT(*) FILTER (WHERE dias BETWEEN 15 AND 24) AS total_atencao
FROM risco;
`

// faturadoFornecedorQuery só traz as métricas SOMÁVEIS entre meses
// (liquido, mix) — bounded por mes <= $5 pros dois anos, senão o ano
// anterior (já fechado) soma os 12 meses contra só os meses já
// decorridos do ano atual (achado 17/09/2026: "período anterior
// completo" inflava a venda/base de comparação sem avisar).
//
// positivados/base_cli NÃO entram aqui: são fotos DO MÊS (ver
// zai_farol.go, "NÃO SOMÁVEL"), somar 12 linhas conta o mesmo cliente
// uma vez por mês E por fornecedor — é o que explicava Positivação
// Atual = 3,1 milhões e Base Ativa = 224 mil no /painel (real: base
// ~47 mil, positivação ~39 mil, batendo com o FAROL). Essas duas
// métricas vêm de positivacaoQuery, direto de vendas_faturadas/
// vendas_transmitidas com COUNT(DISTINCT cnpj).
const faturadoFornecedorQuery = `
WITH ant AS (
    SELECT
        cod_fornec,
        MAX(nome_fornec)                       AS nome_fornec,
        SUM(liquido)                           AS venda_anterior,
        ROUND(AVG(NULLIF(mix,0))::numeric, 1)  AS mix_medio
    FROM farol.agg_fat_v01_l0_mes
    WHERE empresa_id = $1::uuid AND ano = $2 AND mes <= $5
    GROUP BY cod_fornec
),
atu AS (
    SELECT
        cod_fornec,
        MAX(nome_fornec)                       AS nome_fornec,
        SUM(liquido)                           AS venda_atual,
        ROUND(AVG(NULLIF(mix,0))::numeric, 1)  AS mix_medio
    FROM farol.agg_fat_v01_l0_mes
    WHERE empresa_id = $1::uuid AND ano = $3 AND mes <= $5
    GROUP BY cod_fornec
)
SELECT
    COALESCE(a.cod_fornec,  b.cod_fornec)                      AS cod_fornec,
    COALESCE(b.nome_fornec, a.nome_fornec)                     AS nome_fornec,
    COALESCE(a.venda_anterior, 0)                              AS venda_anterior,
    COALESCE(b.venda_atual,    0)                              AS venda_atual,
    CASE WHEN COALESCE(a.venda_anterior, 0) > 0
         THEN ROUND((COALESCE(b.venda_atual,0)/a.venda_anterior*100)::numeric,0)::int
         ELSE 0 END                                            AS pct_venda,
    COALESCE(b.mix_medio, a.mix_medio, 0)                     AS mix_medio
FROM ant a
FULL OUTER JOIN atu b USING (cod_fornec)
ORDER BY venda_atual DESC NULLS LAST
LIMIT $4
`

// positivacaoQuery calcula positivação (clientes distintos que
// compraram) e base ativa direto das tabelas cruas, por fornecedor e
// pro total (GROUPING SETS evita escanear 2x). "Base Ativa" aqui é
// definida como clientes distintos com movimento no período ATUAL
// (compartilhada entre todas as linhas, mesmo comportamento visto no
// próprio FAROL) — é uma aproximação razoável, não é bit-a-bit igual
// ao "rolling 12 meses" que o FAROL usa internamente (farol_v2_api.go),
// mas elimina o double-counting sem reimplementar aquela lógica aqui.
const positivacaoQuery = `
WITH movimento AS (
    SELECT v.cnpj, v.cod_fornec, v.data_faturamento AS data_evento
    FROM vendas_faturadas v
    WHERE v.qt > 0 AND v.empresa_id = $1::uuid AND v.cod_fornec <> ''
      AND v.data_faturamento BETWEEN $2::date AND $5::date
    UNION ALL
    SELECT v.cnpj, v.cod_fornec, v.data_transmissao AS data_evento
    FROM vendas_transmitidas v
    WHERE v.qt > 0 AND v.empresa_id = $1::uuid AND v.cod_fornec <> ''
      AND v.data_transmissao BETWEEN $2::date AND $5::date
)
SELECT
    COALESCE(cod_fornec, '') AS cod_fornec,
    COUNT(DISTINCT cnpj) FILTER (WHERE data_evento BETWEEN $2::date AND $3::date) AS posit_anterior,
    COUNT(DISTINCT cnpj) FILTER (WHERE data_evento BETWEEN $4::date AND $5::date) AS posit_atual
FROM movimento
GROUP BY GROUPING SETS ((cod_fornec), ())
`

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func authOK(r *http.Request) bool {
	token := os.Getenv("API_TOKEN")
	auth := r.Header.Get("Authorization")
	return token != "" && auth == "Bearer "+token
}

// buscarResumo/buscarCobertura extraídas dos handlers JSON originais pra
// serem reusadas também pelos handlers de e-mail novos (coberturaEmailHandler)
// sem duplicar SQL nem lógica de scan.

func buscarResumo(ctx context.Context) (Resumo, error) {
	var res Resumo
	err := pool.QueryRow(ctx, resumoQuery, empresaJC).
		Scan(&res.TotalClientes, &res.TotalRcas, &res.TotalCritico, &res.TotalAtencao)
	return res, err
}

func buscarCobertura(ctx context.Context, supervisor string, limit int) ([]Cobertura, error) {
	rows, err := pool.Query(ctx, query, empresaJC, supervisor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []Cobertura{}
	for rows.Next() {
		var c Cobertura
		if err := rows.Scan(&c.CodSupervisor, &c.NomeSupervisor, &c.CodRca,
			&c.NomeRca, &c.Cnpj, &c.NomeCli, &c.Fantasia, &c.UltimaVenda,
			&c.DiasSemComprar); err != nil {
			log.Println("scan error:", err)
			continue
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func resumoHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	snap := obterCoberturaSnapshot()
	if snap.AtualizadoEm.IsZero() {
		writeJSONErro(w, http.StatusServiceUnavailable, "snapshot de cobertura ainda carregando, tente novamente em instantes")
		return
	}
	res := snap.Resumo
	res.AtualizadoEm = snap.AtualizadoEm.Format(time.RFC3339)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func limiteDaQuery(r *http.Request, padrao, capoTodos int) int {
	limit := padrao
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	}
	if r.URL.Query().Get("all") == "true" {
		limit = capoTodos
	}
	return limit
}

func coberturaHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	snap := obterCoberturaSnapshot()
	if snap.AtualizadoEm.IsZero() {
		writeJSONErro(w, http.StatusServiceUnavailable, "snapshot de cobertura ainda carregando, tente novamente em instantes")
		return
	}
	supervisor := r.URL.Query().Get("supervisor")
	limit := limiteDaQuery(r, 100, 1_000_000)
	results := filtrarPorSupervisorELimit(snap.Linhas, supervisor, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// coberturaEmailHandler monta o Painel de Cobertura (resumo + lista) e
// manda por e-mail — AD-7 da espinha: entrega de painel é sempre
// server-side, o agente de IA só chama esta rota e confirma o envio.
func coberturaEmailHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		writeJSONErro(w, http.StatusBadRequest, "parâmetro obrigatório: email")
		return
	}
	supervisor := r.URL.Query().Get("supervisor")

	snap := obterCoberturaSnapshot()
	if snap.AtualizadoEm.IsZero() {
		writeJSONErro(w, http.StatusServiceUnavailable, "snapshot de cobertura ainda carregando, tente novamente em instantes")
		return
	}
	res := snap.Resumo
	res.AtualizadoEm = snap.AtualizadoEm.Format(time.RFC3339)
	linhas := filtrarPorSupervisorELimit(snap.Linhas, supervisor, 1_000_000)

	assunto, texto, htmlBody := construirEmailCobertura(res, linhas, supervisor)
	if err := sendHTMLReport([]string{email}, assunto, texto, htmlBody); err != nil {
		writeJSONErro(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"enviado": true, "para": email, "total_clientes": res.TotalClientes})
}

// resolverAnosELimit / buscarFaturadoFornecedor extraídas do handler
// original pra reuso pelo handler de e-mail novo (mesmo racional de
// buscarResumo/buscarCobertura acima).
//
// mesLimite bound os dois anos no mesmo número de meses decorridos —
// sem isso o ano anterior (já fechado) somava os 12 meses contra só os
// meses já decorridos do ano atual (achado 17/09/2026). iniAnt/fimAnt/
// iniAtu/fimAtu são os mesmos recortes em data exata (dia-a-dia),
// usados só pela positivacaoQuery — o agregado mensal (venda) não tem
// granularidade de dia, mas as tabelas cruas têm.
func resolverAnosELimit(r *http.Request) (anoAnterior, anoAtual, mesLimite, limit int, iniAnt, fimAnt, iniAtu, fimAtu time.Time) {
	hoje := time.Now()
	anoAtual = hoje.Year()
	anoAnterior = anoAtual - 1
	if v := r.URL.Query().Get("ano_anterior"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 2020 && n < anoAtual {
			anoAnterior = n
		}
	}
	if v := r.URL.Query().Get("ano_atual"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= anoAnterior {
			anoAtual = n
		}
	}
	mesLimite = int(hoje.Month())
	limit = limiteDaQuery(r, 200, 100_000)

	iniAtu = time.Date(anoAtual, 1, 1, 0, 0, 0, 0, time.UTC)
	fimAtu = time.Date(anoAtual, hoje.Month(), hoje.Day(), 0, 0, 0, 0, time.UTC)
	iniAnt = time.Date(anoAnterior, 1, 1, 0, 0, 0, 0, time.UTC)
	fimAnt = time.Date(anoAnterior, hoje.Month(), hoje.Day(), 0, 0, 0, 0, time.UTC)
	return
}

func buscarFaturadoFornecedor(ctx context.Context, anoAnterior, anoAtual, mesLimite, limit int, iniAnt, fimAnt, iniAtu, fimAtu time.Time) (FaturadoResp, error) {
	rows, err := pool.Query(ctx, faturadoFornecedorQuery, empresaJC, anoAnterior, anoAtual, limit, mesLimite)
	if err != nil {
		return FaturadoResp{}, err
	}
	defer rows.Close()
	fornecedorPorCod := map[string]*FornecedorRow{}
	var ordem []string
	for rows.Next() {
		f := &FornecedorRow{}
		if err := rows.Scan(
			&f.CodFornec, &f.NomeFornec,
			&f.VendaAnterior, &f.VendaAtual, &f.PctVenda,
			&f.MixMedio,
		); err != nil {
			log.Println("faturado scan error:", err)
			continue
		}
		fornecedorPorCod[f.CodFornec] = f
		ordem = append(ordem, f.CodFornec)
	}
	if err := rows.Err(); err != nil {
		return FaturadoResp{}, err
	}

	// positivação/base ativa vêm de uma query separada (distinct count
	// nas tabelas cruas) — nunca soma/máx dos agregados mensais, ver
	// comentário de positivacaoQuery.
	positRows, err := pool.Query(ctx, positivacaoQuery, empresaJC, iniAnt, fimAnt, iniAtu, fimAtu)
	if err != nil {
		return FaturadoResp{}, err
	}
	defer positRows.Close()
	var totalPositAnterior, totalPositAtual int
	for positRows.Next() {
		var codFornec string
		var positAnterior, positAtual int
		if err := positRows.Scan(&codFornec, &positAnterior, &positAtual); err != nil {
			log.Println("positivacao scan error:", err)
			continue
		}
		if codFornec == "" {
			// linha do GROUPING SETS () — total geral, não um fornecedor.
			totalPositAnterior, totalPositAtual = positAnterior, positAtual
			continue
		}
		if f, ok := fornecedorPorCod[codFornec]; ok {
			f.PositAnterior, f.PositAtual = positAnterior, positAtual
		}
	}
	if err := positRows.Err(); err != nil {
		return FaturadoResp{}, err
	}

	// Base ativa é compartilhada entre todas as linhas (mesma definição
	// que o próprio FAROL mostra nessa tela: um denominador só, não um
	// recorte por fornecedor).
	for _, codFornec := range ordem {
		f := fornecedorPorCod[codFornec]
		f.ClientesAtivos = totalPositAtual
		if totalPositAtual > 0 {
			f.PctPosAtual = int(math.Round(float64(f.PositAtual) / float64(totalPositAtual) * 100))
		}
	}

	fornecedores := make([]FornecedorRow, 0, len(ordem))
	for _, codFornec := range ordem {
		fornecedores = append(fornecedores, *fornecedorPorCod[codFornec])
	}

	var total FornecedorRow
	total.CodFornec = "TOTAL"
	total.NomeFornec = "TOTAL"
	total.ClientesAtivos = totalPositAtual
	total.PositAnterior = totalPositAnterior
	total.PositAtual = totalPositAtual
	var mixSum float64
	var mixCount int
	for _, f := range fornecedores {
		total.VendaAnterior += f.VendaAnterior
		total.VendaAtual += f.VendaAtual
		if f.MixMedio > 0 {
			mixSum += f.MixMedio
			mixCount++
		}
	}
	if total.VendaAnterior > 0 {
		total.PctVenda = int(math.Round(total.VendaAtual / total.VendaAnterior * 100))
	}
	if totalPositAtual > 0 {
		total.PctPosAtual = int(math.Round(float64(total.PositAtual) / float64(totalPositAtual) * 100))
	}
	if mixCount > 0 {
		total.MixMedio = math.Round(mixSum/float64(mixCount)*10) / 10
	}
	return FaturadoResp{
		AnoAnterior:  anoAnterior,
		AnoAtual:     anoAtual,
		Total:        total,
		Fornecedores: fornecedores,
	}, nil
}

func faturadoFornecedorHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	anoAnterior, anoAtual, mesLimite, limit, iniAnt, fimAnt, iniAtu, fimAtu := resolverAnosELimit(r)
	resp, err := buscarFaturadoFornecedor(context.Background(), anoAnterior, anoAtual, mesLimite, limit, iniAnt, fimAnt, iniAtu, fimAtu)
	if err != nil {
		log.Println("faturado-fornecedor query error:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// faturadoFornecedorEmailHandler monta o Painel de Faturado e manda por
// e-mail — AD-7.
func faturadoFornecedorEmailHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		writeJSONErro(w, http.StatusBadRequest, "parâmetro obrigatório: email")
		return
	}
	anoAnterior, anoAtual, mesLimite, _, iniAnt, fimAnt, iniAtu, fimAtu := resolverAnosELimit(r)
	resp, err := buscarFaturadoFornecedor(context.Background(), anoAnterior, anoAtual, mesLimite, 100_000, iniAnt, fimAnt, iniAtu, fimAtu)
	if err != nil {
		log.Println("faturado-fornecedor query error:", err)
		writeJSONErro(w, http.StatusInternalServerError, "erro buscando faturado por fornecedor")
		return
	}
	assunto, texto, htmlBody := construirEmailFaturado(resp)
	if err := sendHTMLReport([]string{email}, assunto, texto, htmlBody); err != nil {
		writeJSONErro(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"enviado": true, "para": email, "ano_anterior": anoAnterior, "ano_atual": anoAtual})
}

// objetivosIndustriaEmailHandler repassa a chamada pro FB_FAROL (AD-3 —
// nunca recalcula, só encaminha).
func objetivosIndustriaEmailHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	industria, periodo, email := q.Get("industria"), q.Get("periodo"), strings.TrimSpace(q.Get("email"))
	if industria == "" || periodo == "" || email == "" {
		writeJSONErro(w, http.StatusBadRequest, "parâmetros obrigatórios: industria, periodo, email")
		return
	}
	client, err := newFarolClient()
	if err != nil {
		log.Println("farol client error:", err)
		writeJSONErro(w, http.StatusInternalServerError, "FAROL_GATEWAY_TOKEN não configurado neste serviço")
		return
	}
	out, err := client.enviarObjetivosIndustriaEmail(industria, periodo, q.Get("fluxo"), email)
	if err != nil {
		writeJSONErro(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// escreverProxy repassa status+corpo crus do módulo de origem — usado
// pelas 4 rotas de proxy abaixo (industrias, objetivos-industria,
// comparativo-fechamento, historico-calibragem). Nunca reformata o
// corpo: se o módulo de origem errou, o erro dele chega intacto.
func escreverProxy(w http.ResponseWriter, status int, body []byte, proxyErr error) {
	if proxyErr != nil {
		writeJSONErro(w, http.StatusBadGateway, proxyErr.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// industriasHandler / objetivosIndustriaHandler / comparativoFechamentoHandler
// — proxy puro pro FB_FAROL. Migrados pra cá em 17/09/2026 (plano de
// 16/09: FB_CEREBRO vira o único gateway — AD-2 da espinha — em vez de
// cada agente ter o FAROL_MCP_TOKEN próprio).
func industriasHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	client, err := newFarolClient()
	if err != nil {
		writeJSONErro(w, http.StatusInternalServerError, "FAROL_GATEWAY_TOKEN não configurado neste serviço")
		return
	}
	status, body, err := client.industrias()
	escreverProxy(w, status, body, err)
}

func objetivosIndustriaHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	industria, periodo := q.Get("industria"), q.Get("periodo")
	if industria == "" || periodo == "" {
		writeJSONErro(w, http.StatusBadRequest, "parâmetros obrigatórios: industria, periodo")
		return
	}
	client, err := newFarolClient()
	if err != nil {
		writeJSONErro(w, http.StatusInternalServerError, "FAROL_GATEWAY_TOKEN não configurado neste serviço")
		return
	}
	status, body, err := client.objetivosIndustria(industria, periodo, q.Get("fluxo"))
	escreverProxy(w, status, body, err)
}

func comparativoFechamentoHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	industria, periodo := q.Get("industria"), q.Get("periodo")
	if industria == "" || periodo == "" {
		writeJSONErro(w, http.StatusBadRequest, "parâmetros obrigatórios: industria, periodo")
		return
	}
	client, err := newFarolClient()
	if err != nil {
		writeJSONErro(w, http.StatusInternalServerError, "FAROL_GATEWAY_TOKEN não configurado neste serviço")
		return
	}
	status, body, err := client.comparativoFechamento(industria, periodo)
	escreverProxy(w, status, body, err)
}

// historicoCalibragemHandler — proxy puro pro FB_SMARTPICK, mesmo
// racional acima.
func historicoCalibragemHandler(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	cdID := r.URL.Query().Get("cd_id")
	if cdID == "" {
		writeJSONErro(w, http.StatusBadRequest, "parâmetro obrigatório: cd_id")
		return
	}
	client, err := newSmartPickClient()
	if err != nil {
		writeJSONErro(w, http.StatusInternalServerError, "SMARTPICK_GATEWAY_TOKEN não configurado neste serviço")
		return
	}
	status, body, err := client.historicoCalibragem(cdID, r.URL.Query().Get("ano_inicio"))
	escreverProxy(w, status, body, err)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := pool.Ping(context.Background()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "erro", "detalhe": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func writeJSONErro(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func painelHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(painelHTML)
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL não definida")
	}
	if os.Getenv("API_TOKEN") == "" {
		log.Fatal("API_TOKEN não definida")
	}
	var err error
	pool, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatal("erro conectando ao banco: ", err)
	}
	defer pool.Close()

	iniciarAtualizacaoPeriodicaCobertura()

	http.HandleFunc("/cobertura", withCORS(coberturaHandler))
	http.HandleFunc("/resumo", withCORS(resumoHandler))
	http.HandleFunc("/faturado-fornecedor", withCORS(faturadoFornecedorHandler))
	http.HandleFunc("/painel", painelHandler)
	http.HandleFunc("/cobertura-email", withCORS(coberturaEmailHandler))
	http.HandleFunc("/faturado-fornecedor-email", withCORS(faturadoFornecedorEmailHandler))
	http.HandleFunc("/objetivos-industria-email", withCORS(objetivosIndustriaEmailHandler))
	http.HandleFunc("/industrias", withCORS(industriasHandler))
	http.HandleFunc("/objetivos-industria", withCORS(objetivosIndustriaHandler))
	http.HandleFunc("/comparativo-fechamento", withCORS(comparativoFechamentoHandler))
	http.HandleFunc("/historico-calibragem", withCORS(historicoCalibragemHandler))
	http.HandleFunc("/enviar-email", withCORS(enviarEmailHandler))
	http.HandleFunc("/noticias-investimento", withCORS(noticiasInvestimentoHandler))
	http.HandleFunc("/health", healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	log.Println("cerebro-jc API ouvindo na porta", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
