package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultCompilePort = 6000
	maxRequestBytes    = 512 * 1024
	maxSourceBytes     = 128 * 1024
	compileTimeout     = 15 * time.Second
	runTimeout         = 5 * time.Second
)

type compileRequest struct {
	Language string   `json:"language"`
	Source   string   `json:"source"`
	Args     []string `json:"args"`
}

type compileResponse struct {
	CompileStdout string `json:"compile_stdout"`
	CompileStderr string `json:"compile_stderr"`
	RunStdout     string `json:"run_stdout"`
	RunStderr     string `json:"run_stderr"`
	ExitCode      int    `json:"exit_code"`
	Error         string `json:"error"`
	BinaryBase64  string `json:"binary_base64"`
}

var compilePort = flag.Int("port", defaultCompilePort, "TCP port for the compilation server")

func main() {
	flag.Parse()

	addr := fmt.Sprintf(":%d", *compilePort)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}
	defer listener.Close()

	log.Printf("[compiler] listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handleCompileConn(conn)
	}
}

func handleCompileConn(conn net.Conn) {
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(compileTimeout + runTimeout + 5*time.Second)); err != nil {
		log.Printf("deadline error: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	payload, err := reader.ReadBytes('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			log.Printf("read payload error: %v", err)
		}
		return
	}
	if len(payload) > maxRequestBytes {
		writeResponse(conn, compileResponse{Error: "request too large"})
		return
	}

	var request compileRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		writeResponse(conn, compileResponse{Error: fmt.Sprintf("invalid JSON: %v", err)})
		return
	}

	response := processCompileRequest(request)
	writeResponse(conn, response)
}

func processCompileRequest(request compileRequest) compileResponse {
	if !strings.EqualFold(request.Language, "c") {
		return compileResponse{Error: "only C language is supported in this prototype"}
	}
	if len(request.Source) == 0 {
		return compileResponse{Error: "source code is empty"}
	}
	if len(request.Source) > maxSourceBytes {
		return compileResponse{Error: fmt.Sprintf("source code exceeds %d bytes", maxSourceBytes)}
	}

	tempDir, err := os.MkdirTemp("", "compile-server-*")
	if err != nil {
		return compileResponse{Error: fmt.Sprintf("failed to create temp dir: %v", err)}
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "submission.c")
	binaryPath := filepath.Join(tempDir, "submission.out")

	if err := os.WriteFile(sourcePath, []byte(request.Source), 0o644); err != nil {
		return compileResponse{Error: fmt.Sprintf("failed to write source file: %v", err)}
	}

	compileStdout, compileStderr, compileErr := runCommand(compileTimeout, tempDir, "gcc", "-std=c11", "-O0", "-Wall", "-Wextra", sourcePath, "-o", binaryPath)
	if compileErr != nil {
		return compileResponse{
			CompileStdout: compileStdout,
			CompileStderr: compileStderr,
			ExitCode:      exitCode(compileErr),
			Error:         "compilation failed",
		}
	}

	runArgs := append([]string{binaryPath}, request.Args...)
	runStdout, runStderr, runErr := runCommand(runTimeout, tempDir, runArgs[0], runArgs[1:]...)

	response := compileResponse{
		CompileStdout: compileStdout,
		CompileStderr: compileStderr,
		RunStdout:     runStdout,
		RunStderr:     runStderr,
		ExitCode:      exitCode(runErr),
	}

	if runErr != nil {
		response.Error = runErr.Error()
		return response
	}

	binaryBytes, err := os.ReadFile(binaryPath)
	if err == nil {
		response.BinaryBase64 = base64.StdEncoding.EncodeToString(binaryBytes)
	} else {
		log.Printf("failed to read binary for base64 encoding: %v", err)
	}

	return response
}

func runCommand(timeout time.Duration, workDir string, name string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	cmd.Env = []string{
		"PATH=/usr/bin:/bin:/usr/local/bin",
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	if ctx.Err() == context.DeadlineExceeded {
		return stdoutStr, stderrStr, fmt.Errorf("%s timed out after %s", name, timeout)
	}

	return stdoutStr, stderrStr, err
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return -1
}

func writeResponse(conn net.Conn, resp compileResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to marshal response: %v", err)
		return
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
