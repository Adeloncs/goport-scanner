# goport-scanner

[![CI](https://github.com/Adeloncs/goport-scanner/actions/workflows/ci.yml/badge.svg)](https://github.com/Adeloncs/goport-scanner/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.24%2B-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Scanner de portas TCP paralelo escrito em Go, usando concorrência idiomática:
**Goroutines + Channels no padrão Worker Pool**, com `sync.WaitGroup` para
coordenação e fechamento seguro dos canais.

## Recursos

- Escaneamento concorrente com pool de workers configurável
- `net.DialTimeout` para evitar conexões presas (timeout configurável)
- Parser de portas flexível: intervalos, listas e combinações
- Saída ordenada com identificação de serviços comuns
- Progresso ao vivo no terminal
- Zero dependências externas (apenas a biblioteca padrão)

## Requisitos

- Go 1.24 ou superior

## Instalação

```bash
git clone <url-do-repositorio>
cd goport-scanner
go build -o tcp-scanner .
```

Ou execute diretamente sem compilar:

```bash
go run . -host 127.0.0.1 -ports 1-1024
```

## Uso

```bash
tcp-scanner [flags]
```

### Flags

| Flag       | Descrição                                    | Padrão      |
|------------|----------------------------------------------|-------------|
| `-host`    | IP ou domínio alvo                           | `127.0.0.1` |
| `-ports`   | Portas a escanear (intervalo, lista ou mix)  | `1-1024`    |
| `-workers` | Número de goroutines simultâneas             | `100`       |
| `-timeout` | Timeout de conexão em milissegundos          | `1000`      |

### Formatos de `-ports`

| Formato       | Exemplo          | Significado                    |
|---------------|------------------|--------------------------------|
| Intervalo     | `1-1024`         | Portas de 1 a 1024             |
| Lista         | `80,443,8080`    | Apenas essas portas            |
| Combinação    | `22,80,1000-2000`| Lista e intervalos misturados  |

Portas duplicadas são removidas e o resultado é sempre ordenado.

## Exemplos

```bash
# Scan padrão (portas 1-1024) no localhost
go run . -host 127.0.0.1

# Escanear portas específicas de um host remoto
go run . -host scanme.nmap.org -ports 22,80,443,8080

# Range grande com mais workers e timeout menor
go run . -host 192.168.0.1 -ports 1-65535 -workers 500 -timeout 300
```

### Saída de exemplo

```
Escaneando 127.0.0.1 — 1024 porta(s), 100 worker(s), timeout 300ms
Progresso: 1024/1024 (4 aberta[s])

Concluído em 3ms

Portas abertas (4):
  PORTA    SERVIÇO
  -----    -------
  80       http
  139      -
  445      smb
  631      -
```

## Como funciona

```
main → [jobs] → workers (N) → [results] → coletor/ordenador → stdout
         │                          │
         └ produtor fecha jobs      └ goroutine: wg.Wait(); close(results)
```

1. O canal `jobs` (com buffer) recebe todas as portas a escanear.
2. `N` workers consomem de `jobs`, tentam `net.DialTimeout` e publicam
   `ScanResult{Port, Open}` no canal `results`.
3. Uma goroutine produtora envia as portas e fecha `jobs`.
4. Uma goroutine dedicada aguarda `wg.Wait()` e então fecha `results` — garantindo
   que o canal de resultados só feche depois que todos os workers terminarem.
5. A goroutine principal coleta os resultados, ordena e imprime as portas abertas.

Apenas conexões TCP bem-sucedidas classificam a porta como **aberta**.

## Testes

```bash
# Testes com o detector de race conditions ativo
go test -race ./...

# Com saída detalhada
go test -race -v ./...
```

Os testes cobrem o parser de portas (`parsePorts`) e a lógica de conexão do worker
(porta aberta via listener real e porta fechada).

## Aviso legal

Use esta ferramenta apenas em sistemas que você possui ou tem autorização
explícita para testar. Escanear portas de terceiros sem permissão pode ser ilegal.
