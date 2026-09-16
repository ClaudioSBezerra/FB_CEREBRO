package main

// cache_cobertura.go — snapshot em memória do Painel de Cobertura.
//
// Achado em produção (16/09/2026): a query de Cobertura/Resumo varre
// vendas_faturadas/vendas_transmitidas SEM filtro de data (precisa do
// histórico inteiro pra saber "há quantos dias o cliente não compra") —
// com os 2 anos de retenção do FAROL, isso virou uma varredura pesada
// (confirmado no pg_stat_activity: 40s+ de I/O real, não trava de rede).
// Faturado por Fornecedor e Objetivos por Indústria já são rápidos porque
// LEEM de agregados pré-calculados (agg_fat_v01_l0_mes / snapshot diário
// do FAROL) — aqui aplicamos o mesmo princípio: calcula 1x, serve da
// memória depois. Este serviço só tem acesso de LEITURA ao Postgres
// (cerebro_readonly), por isso o snapshot fica em memória do processo,
// não numa tabela nova.
//
// "De ontem" o suficiente: atualiza no boot e depois 1x por dia — não
// precisa ser em tempo real pra responder "quem está sem comprar".

import (
	"context"
	"log"
	"sync"
	"time"
)

type coberturaSnapshot struct {
	Resumo       Resumo
	Linhas       []Cobertura
	AtualizadoEm time.Time
	UltimoErro   string
}

var (
	coberturaMu    sync.RWMutex
	coberturaAtual coberturaSnapshot
)

func obterCoberturaSnapshot() coberturaSnapshot {
	coberturaMu.RLock()
	defer coberturaMu.RUnlock()
	return coberturaAtual
}

// atualizarCoberturaSnapshot roda a consulta pesada UMA VEZ e substitui o
// snapshot inteiro — nunca serve resultado parcial. Timeout generoso (5min)
// pra nunca travar o processo pra sempre, mesmo se o banco estiver muito
// lento num dia ruim.
func atualizarCoberturaSnapshot() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t0 := time.Now()
	res, err := buscarResumo(ctx)
	if err != nil {
		log.Println("[snapshot-cobertura] erro no resumo:", err)
		coberturaMu.Lock()
		coberturaAtual.UltimoErro = err.Error()
		coberturaMu.Unlock()
		return
	}
	linhas, err := buscarCobertura(ctx, "", 1_000_000)
	if err != nil {
		log.Println("[snapshot-cobertura] erro na lista:", err)
		coberturaMu.Lock()
		coberturaAtual.UltimoErro = err.Error()
		coberturaMu.Unlock()
		return
	}

	coberturaMu.Lock()
	coberturaAtual = coberturaSnapshot{Resumo: res, Linhas: linhas, AtualizadoEm: time.Now(), UltimoErro: ""}
	coberturaMu.Unlock()

	log.Printf("[snapshot-cobertura] atualizado: %d clientes, %d linhas, em %v", res.TotalClientes, len(linhas), time.Since(t0))
}

// iniciarAtualizacaoPeriodicaCobertura atualiza no boot (em background,
// pra não atrasar o /health do processo) e depois 1x por dia.
func iniciarAtualizacaoPeriodicaCobertura() {
	go atualizarCoberturaSnapshot()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			atualizarCoberturaSnapshot()
		}
	}()
}

// filtrarESupervisorELimit aplica em memória o mesmo filtro que a query
// original fazia no banco (supervisor + limit) — o snapshot guarda TUDO,
// o filtro vira barato (slice em memória) em vez de reconsultar o banco.
func filtrarPorSupervisorELimit(linhas []Cobertura, supervisor string, limit int) []Cobertura {
	out := make([]Cobertura, 0, len(linhas))
	for _, c := range linhas {
		if supervisor != "" && c.CodSupervisor != supervisor {
			continue
		}
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	return out
}
