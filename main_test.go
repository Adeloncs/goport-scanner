package main

import (
	"net"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    []int
		wantErr bool
	}{
		{name: "intervalo simples", spec: "1-5", want: []int{1, 2, 3, 4, 5}},
		{name: "lista", spec: "80,443,8080", want: []int{80, 443, 8080}},
		{name: "combinação", spec: "22,80,100-102", want: []int{22, 80, 100, 101, 102}},
		{name: "porta única", spec: "443", want: []int{443}},
		{name: "com espaços", spec: " 80 , 443 ", want: []int{80, 443}},
		{name: "duplicatas removidas e ordenadas", spec: "80,22,80,22", want: []int{22, 80}},
		{name: "sobreposição de intervalos", spec: "1-3,2-4", want: []int{1, 2, 3, 4}},
		{name: "limite superior", spec: "65535", want: []int{65535}},

		{name: "vazio", spec: "", wantErr: true},
		{name: "só vírgulas", spec: ",,", wantErr: true},
		{name: "porta zero", spec: "0", wantErr: true},
		{name: "acima do limite", spec: "65536", wantErr: true},
		{name: "início maior que fim", spec: "10-5", wantErr: true},
		{name: "texto não numérico", spec: "abc", wantErr: true},
		{name: "intervalo com texto", spec: "1-abc", wantErr: true},
		{name: "negativo", spec: "-5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePorts(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePorts(%q) esperava erro, retornou %v", tt.spec, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePorts(%q) erro inesperado: %v", tt.spec, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePorts(%q) = %v, quer %v", tt.spec, got, tt.want)
			}
		})
	}
}

// scanSingle executa um worker para uma única porta e devolve o resultado,
// exercitando o mesmo caminho de concorrência usado em produção.
func scanSingle(host string, port int, timeout time.Duration) ScanResult {
	jobs := make(chan int, 1)
	results := make(chan ScanResult, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go worker(host, timeout, jobs, results, &wg)

	jobs <- port
	close(jobs)
	wg.Wait()
	close(results)

	return <-results
}

func TestWorkerOpenPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("não foi possível abrir listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	res := scanSingle("127.0.0.1", port, time.Second)

	if !res.Open {
		t.Errorf("porta %d deveria estar aberta, mas foi classificada como fechada", port)
	}
	if res.Port != port {
		t.Errorf("resultado com porta %d, esperava %d", res.Port, port)
	}
}

func TestWorkerClosedPort(t *testing.T) {
	// Abre e fecha um listener imediatamente para obter uma porta livre.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("não foi possível abrir listener: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	res := scanSingle("127.0.0.1", port, 500*time.Millisecond)

	if res.Open {
		t.Errorf("porta %d deveria estar fechada, mas foi classificada como aberta", port)
	}
}
