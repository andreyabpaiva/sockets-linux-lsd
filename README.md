# sockets-linux-lsd

Este repositório reúne as entregas do trabalho “Sockets em Linux”. Todos os servidores e utilitários foram implementados em Go, e o cliente gráfico para compilação remota foi escrito em Python (Tkinter).

## Estrutura

```
trabalho/
  server_fork/        # servidor echo baseado em fork
  server_threads/     # servidor echo usando uma goroutine por conexão
  server_select/      # servidor echo usando select(2)
  server_epoll/       # servidor echo usando epoll(7)
  client_test/        # cliente CLI para testes rápidos
  compile_server/     # servidor de compilação/execução remoto (C)
  client_gui.py       # cliente Tkinter para o compile_server
  stress_test.sh      # script para saturar os servidores
```

## Compilação

Todos os binários Go compartilham o mesmo módulo:

```bash
cd trabalho
go build ./server_fork
go build ./server_threads
GOOS=linux go build ./server_select  # requer cabeçalhos Linux
GOOS=linux go build ./server_epoll   # requer cabeçalhos Linux
go build ./compile_server
go build ./client_test
```

O cliente gráfico exige Python 3 com Tkinter:

```bash
python3 trabalho/client_gui.py
```

## Execução dos servidores echo

Cada servidor aceita o parâmetro `-port`. Exemplo:

```bash
go run ./trabalho/server_threads -port=5001
```

O cliente Go faz um teste rápido:

```bash
go run ./trabalho/client_test -host=127.0.0.1 -port=5001 -message="ping"
```

> **Observação:** as variantes `select` e `epoll` possuem `//go:build linux`, logo precisam ser compiladas/rodadas em Linux ou com `GOOS=linux`.

## Teste de estresse

Com o servidor no ar, execute:

```bash
bash trabalho/stress_test.sh 127.0.0.1 5001 2000 "load-test" 400
```

Acompanhe os logs para notar `EMFILE`, recusas de conexão ou travamentos e registre os limites aproximados (fork ≈500–1500, goroutines ≈500–2000, select ≈1024 descritores, epoll ≥10.000).

## Workflow de compilação remota

Inicie o servidor:

```bash
go run ./trabalho/compile_server -port=6000
```

Depois abra o cliente (`client_gui.py`), cole ou digite o código C e clique em **Run Code**. Ele recebe os diagnósticos de compilação, a saída do programa e permite salvar o binário retornado via **Download Binary**, tudo via JSON sobre TCP.
