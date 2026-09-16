# FB_CEREBRO

Gateway de agregação da plataforma FB (JC Distribuição) — antes um
binário sem git em `/opt/cerebro-jc` no host de produção, agora trazido
pra este repositório com CI/CD, seguindo a espinha de arquitetura em
`FB_FAROL/_bmad-output/planning-artifacts/architecture/architecture-FB_FAROL-2026-09-16/ARCHITECTURE-SPINE.md`.

## O que faz

Lê Postgres do FB_FAROL direto (dívida técnica nomeada, ver AD-3 da
espinha) pros painéis de Cobertura e Faturado por Fornecedor, e repassa
(proxy HTTP puro, nunca recalcula) pro FB_FAROL o painel de Objetivos por
Indústria.

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
| `PORT` | não (default `8090`) | porta HTTP |

## Testes

```
go test ./...
```

16 testes, sem banco nem rede real (AD-9 da espinha) — a lógica pura
(formatação de e-mail, montagem de HTML, parsing de parâmetros) é
validada sem infraestrutura nenhuma.

## Deploy (pendente — precisa de ação humana, ver abaixo)

Este repo está pronto (código, testes, Dockerfile validado localmente
com `docker build .` rodando vet+test+build), mas **falta**:

1. **Criar o repositório no GitHub** (`ClaudioSBezerra/FB_CEREBRO`,
   vazio, sem README/licença — já tem tudo aqui). Não consegui fazer
   isso sozinho nesta sessão (só tenho a chave SSH que empurra pra repo
   já existente, não autenticação pra criar um novo).
   ```
   git remote add origin git@github.com:ClaudioSBezerra/FB_CEREBRO.git
   git push -u origin main
   ```
2. **Criar o app no Coolify** (git-based, apontando pro repo acima) —
   mesma forma que o `farol-api` já está configurado.
3. **Cadastrar as env vars** da tabela acima no novo app Coolify —
   reaproveitar os mesmos valores de `DATABASE_URL`/`API_TOKEN`/`SMTP_*`
   que já existem no processo manual antigo, e gerar um `FAROL_GATEWAY_TOKEN`
   novo (`openssl rand -hex 32`).
4. **Cadastrar esse mesmo `FAROL_GATEWAY_TOKEN`** no serviço `api` do
   FB_FAROL (Coolify), pra ele aceitar a chamada deste Gateway (o código
   do lado FB_FAROL pra isso já foi commitado e pusheado — só falta a
   env var).
5. Seguir o plano de corte (AD-10 da espinha): subir o novo app Coolify
   numa porta/rota alternativa primeiro, validar, só então trocar o
   rótulo Traefik de `cerebro-jc-api.fbtechia.com` pro novo container, e
   manter o processo manual antigo **parado, não removido**, por um
   tempo de segurança antes de descartar.
