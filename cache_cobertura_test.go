package main

import "testing"

func TestFiltrarPorSupervisorELimitSemFiltro(t *testing.T) {
	linhas := []Cobertura{
		{CodSupervisor: "S1", Cnpj: "1"},
		{CodSupervisor: "S2", Cnpj: "2"},
		{CodSupervisor: "S1", Cnpj: "3"},
	}
	got := filtrarPorSupervisorELimit(linhas, "", 10)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
}

func TestFiltrarPorSupervisorELimitComSupervisor(t *testing.T) {
	linhas := []Cobertura{
		{CodSupervisor: "S1", Cnpj: "1"},
		{CodSupervisor: "S2", Cnpj: "2"},
		{CodSupervisor: "S1", Cnpj: "3"},
	}
	got := filtrarPorSupervisorELimit(linhas, "S1", 10)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, c := range got {
		if c.CodSupervisor != "S1" {
			t.Errorf("linha com supervisor %q vazou no filtro de S1", c.CodSupervisor)
		}
	}
}

func TestFiltrarPorSupervisorELimitRespeitaLimit(t *testing.T) {
	linhas := []Cobertura{
		{CodSupervisor: "S1", Cnpj: "1"},
		{CodSupervisor: "S1", Cnpj: "2"},
		{CodSupervisor: "S1", Cnpj: "3"},
	}
	got := filtrarPorSupervisorELimit(linhas, "", 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (respeitando limit)", len(got))
	}
}

func TestObterCoberturaSnapshotAntesDeQualquerAtualizacao(t *testing.T) {
	// snapshot global de pacote: só garante que o zero-value não quebra
	// (AtualizadoEm.IsZero() é o sinal usado pelos handlers pra recusar
	// servir enquanto o snapshot inicial ainda não carregou).
	snap := obterCoberturaSnapshot()
	if !snap.AtualizadoEm.IsZero() && snap.UltimoErro == "" {
		t.Skip("snapshot já foi populado por outro teste neste processo — nada a verificar aqui")
	}
}
