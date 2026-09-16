package main

// panels.go — montagem de HTML dos painéis de Cobertura e Faturado por
// Fornecedor (AD-7: gerado server-side, entregue por e-mail). Funções
// PURAS (recebem os dados já buscados, devolvem string) — testáveis sem
// banco nem rede (AD-9). Mesma paleta visual do resto da plataforma
// (teal #1B6660, verde #2C6E49, âmbar #8a6a2e, vermelho #A34A1B).

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

func escHTML(s string) string { return html.EscapeString(s) }

func brlValor(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	inteiro := int64(v)
	centavos := int64((v-float64(inteiro))*100 + 0.5)
	s := fmt.Sprintf("%d", inteiro)
	var partes []string
	for len(s) > 3 {
		partes = append([]string{s[len(s)-3:]}, partes...)
		s = s[:len(s)-3]
	}
	partes = append([]string{s}, partes...)
	out := "R$ " + strings.Join(partes, ".") + fmt.Sprintf(",%02d", centavos)
	if neg {
		return "-" + out
	}
	return out
}

// ─── Painel de Cobertura ────────────────────────────────────────────────────

func construirEmailCobertura(res Resumo, linhas []Cobertura, supervisorFiltro string) (assunto, texto, htmlBody string) {
	assunto = "Farol — Cobertura de Clientes sem Compra"
	if supervisorFiltro != "" {
		assunto += " (SUPV " + supervisorFiltro + ")"
	}

	// já vem ordenado por dias_sem_comprar DESC (query); cap na exibição.
	limite := linhas
	resto := 0
	if len(limite) > 40 {
		resto = len(limite) - 40
		limite = limite[:40]
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<div style="font-family:Arial,Helvetica,sans-serif;color:#1a1a1a;max-width:680px">`)
	fmt.Fprintf(&b, `<p style="margin:0 0 4px;font-size:13px;color:#667">Farol de Vendas · Cobertura de Clientes</p>`)
	fmt.Fprintf(&b, `<h2 style="margin:0 0 6px;font-size:20px">Clientes sem compra</h2>`)

	fmt.Fprintf(&b, `<div style="background:#f6f7f6;border-left:3px solid #1B6660;padding:14px 18px;margin-bottom:12px">
<div style="font-size:26px;font-weight:bold">%d clientes</div>
<div style="font-size:13px;color:#556;margin-top:4px"><span style="color:#A34A1B;font-weight:bold">%d</span> críticos (≥25 dias sem comprar) · <span style="color:#8a6a2e;font-weight:bold">%d</span> em atenção (15-24 dias) · %d RCAs afetados</div></div>`,
		res.TotalClientes, res.TotalCritico, res.TotalAtencao, res.TotalRcas)

	if len(limite) > 0 {
		fmt.Fprintf(&b, `<h3 style="font-size:15px;margin:22px 0 8px">Clientes por dias sem comprar</h3>`)
		b.WriteString(`<table cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;font-size:14px">`)
		for _, c := range limite {
			cor := "#8a6a2e"
			if c.DiasSemComprar >= 25 {
				cor = "#A34A1B"
			}
			nome := c.Fantasia
			if nome == "" {
				nome = c.NomeCli
			}
			fmt.Fprintf(&b, `<tr>
<td style="padding:9px 0;border-bottom:1px solid #eef1f0">%s
  <div style="color:#667;font-size:12.5px;margin-top:2px">SUPV %s · RCA %s (%s)</div></td>
<td style="padding:9px 0;border-bottom:1px solid #eef1f0;text-align:right;white-space:nowrap;font-weight:bold;color:%s">%d dias</td>
</tr>`, escHTML(nome), escHTML(c.NomeSupervisor), escHTML(c.NomeRca), escHTML(c.CodRca), cor, c.DiasSemComprar)
		}
		b.WriteString(`</table>`)
		if resto > 0 {
			fmt.Fprintf(&b, `<p style="margin:8px 0 0;color:#889;font-size:12.5px">+ %d clientes adicionais sem compra (consulte a API pra lista completa).</p>`, resto)
		}
	}

	b.WriteString(`<hr style="border:0;border-top:1px solid #e3e6e5;margin:26px 0 12px">
<p style="color:#889;font-size:12px;line-height:1.6;margin:0">Classificação: vermelho = 25 dias ou mais sem comprar; âmbar = 15 a 24 dias.</p></div>`)

	htmlBody = b.String()

	var t strings.Builder
	fmt.Fprintf(&t, "Cobertura de Clientes sem Compra\n\n")
	fmt.Fprintf(&t, "Total: %d clientes, %d criticos, %d em atencao, %d RCAs afetados\n", res.TotalClientes, res.TotalCritico, res.TotalAtencao, res.TotalRcas)
	texto = t.String()

	return assunto, texto, htmlBody
}

// ─── Painel de Faturado por Fornecedor ──────────────────────────────────────

func construirEmailFaturado(resp FaturadoResp) (assunto, texto, htmlBody string) {
	assunto = fmt.Sprintf("Farol — Faturado por Fornecedor (%d × %d)", resp.AnoAnterior, resp.AnoAtual)

	fornecedores := make([]FornecedorRow, len(resp.Fornecedores))
	copy(fornecedores, resp.Fornecedores)
	sort.Slice(fornecedores, func(i, j int) bool { return fornecedores[i].VendaAtual > fornecedores[j].VendaAtual })

	corPct := func(pct int) string {
		switch {
		case pct >= 100:
			return "#2C6E49"
		case pct >= 80:
			return "#8a6a2e"
		case pct >= 70:
			return "#C46A1B"
		default:
			return "#A34A1B"
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<div style="font-family:Arial,Helvetica,sans-serif;color:#1a1a1a;max-width:680px">`)
	fmt.Fprintf(&b, `<p style="margin:0 0 4px;font-size:13px;color:#667">Farol de Vendas · Faturado por Fornecedor</p>`)
	fmt.Fprintf(&b, `<h2 style="margin:0 0 6px;font-size:20px">%d × %d</h2>`, resp.AnoAnterior, resp.AnoAtual)

	fmt.Fprintf(&b, `<div style="background:#f6f7f6;border-left:3px solid #1B6660;padding:14px 18px;margin-bottom:12px">
<div style="font-size:13px;color:#556">TOTAL GERAL</div>
<div style="font-size:26px;font-weight:bold;margin-top:2px">%s <span style="font-size:16px;font-weight:normal;color:%s">(%d%%)</span></div>
<div style="font-size:13px;color:#556;margin-top:4px">%d clientes ativos · %d%% positivados · mix médio %.1f</div></div>`,
		brlValor(resp.Total.VendaAtual), corPct(resp.Total.PctVenda), resp.Total.PctVenda,
		resp.Total.ClientesAtivos, resp.Total.PctPosAtual, resp.Total.MixMedio)

	if len(fornecedores) > 0 {
		fmt.Fprintf(&b, `<h3 style="font-size:15px;margin:22px 0 8px">Fornecedores</h3>`)
		b.WriteString(`<table cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;font-size:14px">`)
		for _, f := range fornecedores {
			fmt.Fprintf(&b, `<tr>
<td style="padding:9px 0;border-bottom:1px solid #eef1f0">%s
  <div style="color:#667;font-size:12.5px;margin-top:2px">%d clientes ativos · %d%% positivados</div></td>
<td style="padding:9px 0;border-bottom:1px solid #eef1f0;text-align:right;white-space:nowrap;font-weight:bold;color:%s">%s (%d%%)</td>
</tr>`, escHTML(f.NomeFornec), f.ClientesAtivos, f.PctPosAtual, corPct(f.PctVenda), brlValor(f.VendaAtual), f.PctVenda)
		}
		b.WriteString(`</table>`)
	}

	b.WriteString(`<hr style="border:0;border-top:1px solid #e3e6e5;margin:26px 0 12px">
<p style="color:#889;font-size:12px;line-height:1.6;margin:0">Percentual = venda atual ÷ venda do ano anterior. ≥100% verde, ≥80% âmbar, ≥70% laranja, abaixo vermelho.</p></div>`)

	htmlBody = b.String()

	var t strings.Builder
	fmt.Fprintf(&t, "Faturado por Fornecedor %d x %d\n\n", resp.AnoAnterior, resp.AnoAtual)
	fmt.Fprintf(&t, "TOTAL GERAL: %s (%d%%)\n", brlValor(resp.Total.VendaAtual), resp.Total.PctVenda)
	texto = t.String()

	return assunto, texto, htmlBody
}
