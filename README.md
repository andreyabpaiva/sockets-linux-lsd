# Sockets Linux - Distributed Systems Laboratory

## Structure

```
trabalho/
  server_fork/        
  server_threads/     
  server_select/      
  server_epoll/    
  client_test/       
  compile_server/    
  client_gui.py       
  stress_test.sh      
```

## Compilation

All Go binaries share the same module:

```bash
cd trabalho
go build ./server_fork
go build ./server_threads
GOOS=linux go build ./server_select  # requires Linux header
GOOS=linux go build ./server_epoll   # requires Linux header
go build ./compile_server
go build ./client_test
```

The graphical client requires Python 3 with Tkinter:

```bash
python3 trabalho/client_gui.py
```

## Execution of echo servers

Each server accepts the `-port` parameter. Example:

```bash
go run ./trabalho/server_threads -port=5001
```

The Go client performs a quick test:

```bash
go run ./trabalho/client_test -host=127.0.0.1 -port=5001 -message="ping"
```
**Note:** the `select` and `epoll` variants have `//go:build linux`, therefore they need to be compiled/run on Linux or with `GOOS=linux`.

## Stress test

With the server running, execute:

```bash
bash trabalho/stress_test.sh 127.0.0.1 5001 2000 "load-test" 400
```
Monitor the logs to note `EMFILE` errors, connection refusals, or crashes, and record the approximate limits (fork ≈500–1500, goroutines ≈500–2000, select ≈1024 descriptors, epoll ≥10,000).

## Remote build workflow

Start the server:

```bash
go run ./trabalho/compile_server -port=6000
```
