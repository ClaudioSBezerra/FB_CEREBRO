package main

import "testing"

func TestBrlValor(t *testing.T) {
	casos := []struct {
		in   float64
		want string
	}{
		{0, "R$ 0,00"},
		{1234.5, "R$ 1.234,50"},
		{1234567.89, "R$ 1.234.567,89"},
		{-108.58, "-R$ 108,58"},
		{7.7, "R$ 7,70"},
	}
	for _, c := range casos {
		got := brlValor(c.in)
		if got != c.want {
			t.Errorf("brlValor(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestConstruirEmailCoberturaCoresPorFaixa(t *testing.T) {
	res := Resumo{TotalClientes: 2, TotalRcas: 2, TotalCritico: 1, TotalAtencao: 1}
	linhas := []Cobertura{
		{NomeSupervisor: "SUPV A", NomeRca: "RCA 1", CodRca: "1", Fantasia: "Cliente Crítico", DiasSemComprar: 30},
		{NomeSupervisor: "SUPV B", NomeRca: "RCA 2", CodRca: "2", Fantasia: "Cliente Atenção", DiasSemComprar: 18},
	}
	_, texto, htmlBody := construirEmailCobertura(res, linhas, "")

	if !containsAll(htmlBody, "Cliente Crítico", "#A34A1B", "Cliente Atenção", "#8a6a2e") {
		t.Errorf("HTML não classificou as cores esperadas por faixa de dias:\n%s", htmlBody)
	}
	if !containsAll(texto, "2 clientes", "1", "2") {
		t.Errorf("texto puro não tem os números esperados:\n%s", texto)
	}
}

func TestConstruirEmailCoberturaCapaListaGrande(t *testing.T) {
	res := Resumo{TotalClientes: 50}
	var linhas []Cobertura
	for i := 0; i < 50; i++ {
		linhas = append(linhas, Cobertura{Fantasia: "Cliente", DiasSemComprar: 50 - i})
	}
	_, _, htmlBody := construirEmailCobertura(res, linhas, "")
	if !containsAll(htmlBody, "+ 10 clientes adicionais") {
		t.Errorf("esperava nota de '+10 clientes adicionais' (50 linhas, cap de 40):\n%s", htmlBody)
	}
}

func TestConstruirEmailFaturadoOrdenaPorVendaAtualDesc(t *testing.T) {
	resp := FaturadoResp{
		AnoAnterior: 2025, AnoAtual: 2026,
		Total: FornecedorRow{VendaAtual: 300, PctVenda: 100},
		Fornecedores: []FornecedorRow{
			{NomeFornec: "Fornecedor Pequeno", VendaAtual: 50, PctVenda: 60},
			{NomeFornec: "Fornecedor Grande", VendaAtual: 250, PctVenda: 120},
		},
	}
	_, _, htmlBody := construirEmailFaturado(resp)

	posGrande := indexOf(htmlBody, "Fornecedor Grande")
	posPequeno := indexOf(htmlBody, "Fornecedor Pequeno")
	if posGrande < 0 || posPequeno < 0 || posGrande > posPequeno {
		t.Errorf("Fornecedor Grande (venda maior) deveria aparecer antes de Fornecedor Pequeno")
	}
	if !containsAll(htmlBody, "#2C6E49", "#A34A1B") {
		t.Errorf("esperava cor verde (>=100%%) e vermelha (<70%%) presentes:\n%s", htmlBody)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexOf(s, sub) < 0 {
			return false
		}
	}
	return true
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
