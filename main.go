// Command tcp-scanner é um scanner de portas TCP paralelo que usa o padrão
// Worker Pool (goroutines + channels) para escanear um alvo de forma concorrente.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ScanResult carrega o resultado do escaneamento de uma única porta.
type ScanResult struct {
	Port int
	Open bool
}

// commonServices mapeia portas conhecidas para o nome do serviço, usado apenas
// para deixar a saída final mais informativa.
var commonServices = map[int]string{
	20: "ftp-data", 21: "ftp", 22: "ssh", 23: "telnet", 25: "smtp",
	53: "dns", 80: "http", 110: "pop3", 143: "imap", 443: "https",
	445: "smb", 993: "imaps", 995: "pop3s", 3306: "mysql", 3389: "rdp",
	5432: "postgres", 6379: "redis", 8080: "http-alt", 8443: "https-alt",
	27017: "mongodb",
}

// parsePorts converte uma especificação de portas em uma lista ordenada e sem
// duplicatas. Aceita intervalos ("1-1024"), listas ("80,443,8080") e
// combinações ("22,80,1000-2000"). Retorna erro em qualquer entrada inválida.
func parsePorts(spec string) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("especificação de portas vazia")
	}

	set := make(map[int]struct{})
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err := parsePort(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, fmt.Errorf("intervalo inválido %q: %w", part, err)
			}
			end, err := parsePort(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("intervalo inválido %q: %w", part, err)
			}
			if start > end {
				return nil, fmt.Errorf("intervalo inválido %q: início maior que o fim", part)
			}
			for p := start; p <= end; p++ {
				set[p] = struct{}{}
			}
			continue
		}

		p, err := parsePort(part)
		if err != nil {
			return nil, err
		}
		set[p] = struct{}{}
	}

	if len(set) == 0 {
		return nil, fmt.Errorf("nenhuma porta válida em %q", spec)
	}

	ports := make([]int, 0, len(set))
	for p := range set {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports, nil
}

// parsePort valida uma única porta no intervalo TCP válido (1-65535).
func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("porta inválida %q", s)
	}
	if p < 1 || p > 65535 {
		return 0, fmt.Errorf("porta fora do intervalo (1-65535): %d", p)
	}
	return p, nil
}

// worker consome portas do canal jobs, tenta uma conexão TCP com timeout e
// envia o resultado para o canal results. Chama wg.Done ao terminar.
func worker(host string, timeout time.Duration, jobs <-chan int, results chan<- ScanResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for port := range jobs {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			results <- ScanResult{Port: port, Open: false}
			continue
		}
		conn.Close()
		results <- ScanResult{Port: port, Open: true}
	}
}

func main() {
	host := flag.String("host", "127.0.0.1", "IP ou domínio alvo")
	portsSpec := flag.String("ports", "1-1024", "portas a escanear (ex: \"1-1024\" ou \"80,443,8080\")")
	workers := flag.Int("workers", 100, "número de goroutines simultâneas")
	timeoutMs := flag.Int("timeout", 1000, "timeout de conexão em milissegundos")
	flag.Parse()

	ports, err := parsePorts(*portsSpec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro ao interpretar -ports: %v\n", err)
		os.Exit(1)
	}

	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "erro: -workers deve ser >= 1")
		os.Exit(1)
	}
	// Não adianta ter mais workers do que portas.
	numWorkers := *workers
	if numWorkers > len(ports) {
		numWorkers = len(ports)
	}
	timeout := time.Duration(*timeoutMs) * time.Millisecond

	fmt.Printf("Escaneando %s — %d porta(s), %d worker(s), timeout %s\n",
		*host, len(ports), numWorkers, timeout)

	jobs := make(chan int, numWorkers)
	results := make(chan ScanResult)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(*host, timeout, jobs, results, &wg)
	}

	// Produtor: envia todas as portas e fecha jobs para que os workers parem.
	go func() {
		for _, p := range ports {
			jobs <- p
		}
		close(jobs)
	}()

	// Fecha results somente depois que todos os workers terminarem, garantindo
	// que o range abaixo não bloqueie nem leia de um canal fechado cedo demais.
	go func() {
		wg.Wait()
		close(results)
	}()

	start := time.Now()
	var open []int
	scanned := 0
	total := len(ports)
	for res := range results {
		scanned++
		if res.Open {
			open = append(open, res.Port)
		}
		fmt.Printf("\rProgresso: %d/%d (%d aberta[s])", scanned, total, len(open))
	}
	fmt.Println()

	sort.Ints(open)

	fmt.Printf("\nConcluído em %s\n", time.Since(start).Round(time.Millisecond))
	if len(open) == 0 {
		fmt.Println("Nenhuma porta aberta encontrada.")
		return
	}

	fmt.Printf("\nPortas abertas (%d):\n", len(open))
	fmt.Println("  PORTA    SERVIÇO")
	fmt.Println("  -----    -------")
	for _, p := range open {
		service := commonServices[p]
		if service == "" {
			service = "-"
		}
		fmt.Printf("  %-8d %s\n", p, service)
	}
}
