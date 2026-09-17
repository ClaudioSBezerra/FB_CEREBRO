# FB_CEREBRO

Gateway de agregação da plataforma FB (JC Distribuição) — antes um
binário sem git em `/opt/cerebro-jc` no host de produção, agora trazido
pra este repositório com CI/CD, seguindo a espinha de arquitetura em
`FB_FAROL/_bmad-output/planning-artifacts/architecture/architecture-FB_FAROL-2026-09-16/ARCHITECTURE-SPINE.md`.

## O que faz

Lê Postgres do FB_FAROL direto (dívida técnica nomeada, ver AD-3 da
espinha) pros painéis de Cobertura e Faturado por Fornecedor, e repassa
(proxy HTTP puro, nunca recalcula) pro FB_FAROL o painel de Objetivos por
Indústria e pro FB_SMARTPICK o histórico de calibragem.

**É o único gateway multi-módulo (AD-2 da espinha, migração concluída em
17/09/2026):** nenhum agente de IA deveria mais chamar FB_FAROL ou
FB_SMARTPICK direto — todo mundo passa por aqui. Isso centraliza token,
allowlist e auditoria num lugar só, em vez de espalhar `FAROL_MCP_TOKEN`/
`SMARTPICK_API_KEY` em cada agente do Paperclip.

**Cobertura é servida de um snapshot em memória**, não ao vivo: a query
de Cobertura/Resumo varre 2 anos de `vendas_faturadas`/`vendas_transmitidas`
sem filtro de data (precisa do histórico inteiro pra saber "há quantos
dias o cliente não compra"), o que a torna lenta demais pra responder por
request (confirmado em produção: 40s+ de I/O real). Como este serviço só
tem acesso de **leitura** ao Postgres (`cerebro_readonly`), não dá pra
persistir um snapshot em tabela nova — por isso o cache vive na memória
do processo (`cache_cobertura.go`): recalculado no boot e depois 1x por
dia, servindo "a fotografia de ontem" instantaneamente pras rotas
`/resumo`, `/cobertura` e `/cobertura-email`. Mesmo princípio já usado
por Faturado por Fornecedor (lê `agg_fat_v01_l0_mes`, pré-agregado) e por
Objetivos por Indústria (proxy pro snapshot diário do FAROL). Enquanto o
snapshot inicial não termina de carregar (alguns segundos após o boot),
essas 3 rotas respondem `503` em vez de servir dado zerado.

## Rotas

Todas (exceto `/painel` e `/health`) exigem `Authorization: Bearer <API_TOKEN>`.

| Rota | O que faz |
| --- | --- |
| `GET /resumo` | 4 números de cobertura (JSON) |
| `GET /cobertura?supervisor=&all=true` | lista de clientes sem compra (JSON) |
| `GET /cobertura-email?email=&supervisor=` | monta e envia o painel de Cobertura por e-mail |
| `GET /faturado-fornecedor?all=true` | faturado ano×ano por fornecedor (JSON) |
| `GET /faturado-fornecedor-email?email=` | monta e envia o painel de Faturado por e-mail |
| `GET /objetivos-industria-email?industria=&periodo=&email=&fluxo=` | repassa pro FB_FAROL (`/api/farol-jc/objetivos-industria-email`) |
| `GET /industrias` | repassa pro FB_FAROL (`/api/farol-jc/industrias`) |
| `GET /objetivos-industria?industria=&periodo=&fluxo=` | repassa pro FB_FAROL (`/api/farol-jc/objetivos-industria`), sem e-mail — números crus |
| `GET /comparativo-fechamento?industria=&periodo=` | repassa pro FB_FAROL (`/api/farol-jc/comparativo-fechamento`) |
| `GET /historico-calibragem?cd_id=&ano_inicio=` | repassa pro FB_SMARTPICK (`/api/relatorios/historico-calibragem`) |
| `POST /enviar-email` `{email, assunto, corpo_html, corpo_texto}` | envio genérico de e-mail, pra agentes sem painel próprio (ex: resumo diário do "Monitor do CEO") — `email` aceita 1+ destinatários separados por vírgula; todos precisam estar na allowlist `EMAILS_PERMITIDOS`, senão a chamada inteira é rejeitada |
| `GET /noticias-investimento?query=&max_results=` | busca notícias reais (API REST do Tavily, sem MCP) — devolve título/URL/trecho/data crus, pro agente resumir sem inventar |
| `GET /painel` | dashboard HTML ao vivo, sem auth (link direto) |
| `GET /health` | health-check, sem auth |

## Variáveis de ambiente

| Nome | Obrigatória | Descrição |
| --- | --- | --- |
| `DATABASE_URL` | sim | Postgres do FB_FAROL (mesma string já em uso hoje) |
| `API_TOKEN` | sim | token que os agentes de IA usam pra chamar este serviço |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USER` / `SMTP_PASSWORD` / `SMTP_FROM` | sim, pras rotas `*-email` | mesma conta Hostinger já usada pelo FB_FAROL |
| `FAROL_BASE_URL` | não (default `https://farol.fbtax.cloud`) | base do FB_FAROL pro proxy de Objetivos por Indústria |
| `FAROL_GATEWAY_TOKEN` | sim, pra `/objetivos-industria-email` | token PRÓPRIO deste serviço pra chamar o FB_FAROL — **precisa ser criado e cadastrado como `FAROL_GATEWAY_TOKEN` também no FB_FAROL** (env var separada de `FAROL_MCP_TOKEN`, ver AD-6 da espinha) |
| `EMAILS_PERMITIDOS` | sim, pra `/enviar-email` | lista de destinatários autorizados, separados por vírgula — aceita e-mail exato (`claudio.bezerra@ferreiracosta.com.br`) OU domínio inteiro prefixado com `@` (`@ferreiracosta.com.br`, libera qualquer pessoa desse domínio sem precisar redeployar a cada contratação). E-mails pessoais (gmail/hotmail) precisam estar na lista exata — domínio de provedor público nunca deve ser liberado. Sem essa trava, o `API_TOKEN` vazado viraria relay de e-mail pra qualquer destinatário. Exemplo: `@ferreiracosta.com.br,@fbtechia.com,claudiosousadebezerra@gmail.com,claudio_bezerra@hotmail.com` |
| `TAVILY_API_KEY` | sim, pra `/noticias-investimento` | key da API REST do Tavily (`tvly-...`) — usada porque o sandbox do agente Paperclip não tem busca web confiável nem MCP disponível nessa instância |
| `SMARTPICK_BASE_URL` | não (default `https://smartpick.fbtax.cloud`) | base do FB_SMARTPICK pro proxy de histórico de calibragem |
| `SMARTPICK_GATEWAY_TOKEN` | sim, pra `/historico-calibragem` | precisa ter o MESMO valor do `SMARTPICK_API_KEY` cadastrado no FB_SMARTPICK — nome diferente só pra manter o padrão AD-6 (token por caller class), mas hoje só o FB_CEREBRO chama esse endpoint |
| `PORT` | não (default `8090`) | porta HTTP |

## Testes

```
go test ./...
```

42 testes, sem banco nem rede real (AD-9 da espinha) — a lógica pura
(formatação de e-mail, montagem de HTML, parsing de parâmetros, allowlist)
é validada sem infraestrutura nenhuma.

## Deploy

Rodando em produção via Coolify (docker-compose), deploy automático a
partir do `main` (webhook do GitHub configurado). Pra adicionar uma env
var nova (ex: `EMAILS_PERMITIDOS`), cadastrar no app Coolify e redeployar
— o `docker-compose.yml` já repassa qualquer `${VAR}` referenciada nele
pro container.
