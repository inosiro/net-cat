package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func resetTestFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	port := os.Getenv("HELPER_PORT")
	switch os.Getenv("HELPER_CASE") {
	case "usage_too_many_args":
		os.Args = []string{"TCPChat", "1111", "2222"}
		resetTestFlags()
		main()
	case "server_graceful":
		os.Args = []string{"TCPChat", port}
		resetTestFlags()
		main()
	case "client_banned":
		os.Args = []string{"TCPChat", "-c", "-s", "127.0.0.1", port}
		resetTestFlags()
		main()
	case "client_prompt_flow":
		os.Args = []string{"TCPChat", "-c", "-s", "127.0.0.1", port}
		resetTestFlags()
		main()
	case "client_default_port":
		os.Args = []string{"TCPChat", "-c", "-s", "127.0.0.1"}
		resetTestFlags()
		main()
	case "client_ui_mode":
		os.Args = []string{"TCPChat", "-c", "-ui", "-s", "127.0.0.1", port}
		resetTestFlags()
		main()
	case "client_dial_fail":
		os.Args = []string{"TCPChat", "-c", "-s", "127.0.0.1", port}
		resetTestFlags()
		main()
	case "client_banner_read_fail":
		os.Args = []string{"TCPChat", "-c", "-s", "127.0.0.1", port}
		resetTestFlags()
		main()
	case "server_with_output":
		os.Args = []string{"TCPChat", "-o", os.Getenv("HELPER_OUTPUT_FILE"), port}
		resetTestFlags()
		main()
	case "server_output_open_fail":
		os.Args = []string{"TCPChat", "-o", os.Getenv("HELPER_OUTPUT_FILE"), port}
		resetTestFlags()
		main()
	case "server_listen_fail":
		os.Args = []string{"TCPChat", "abc"}
		resetTestFlags()
		main()
	default:
		os.Exit(2)
	}

	os.Exit(0)
}

func startMockTCPServer(t *testing.T, bindAddr string, handler func(net.Conn)) (net.Listener, string) {
	t.Helper()

	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		t.Fatalf("failed to start mock TCP server: %v", err)
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		handler(conn)
	}()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		ln.Close()
		t.Fatalf("failed to parse listener address: %v", err)
	}

	return ln, port
}

func newHelperCmd(t *testing.T, helperCase string, port string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"HELPER_CASE="+helperCase,
		"HELPER_PORT="+port,
	)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	return cmd, &output
}

func newHelperCmdWithOutputFile(t *testing.T, helperCase string, port string, outputFile string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()

	cmd, output := newHelperCmd(t, helperCase, port)
	cmd.Env = append(cmd.Env, "HELPER_OUTPUT_FILE="+outputFile)
	return cmd, output
}

func TestMainUsageTooManyArgs(t *testing.T) {
	cmd, output := newHelperCmd(t, "usage_too_many_args", "")
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid args")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d; output: %s", exitErr.ExitCode(), output.String())
	}

	if !strings.Contains(output.String(), "[USAGE]: ./TCPChat $port") {
		t.Fatalf("expected usage output, got: %s", output.String())
	}
}

func TestMainServerModeGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		ln.Close()
		t.Fatalf("failed to parse reserved port: %v", err)
	}
	ln.Close()

	cmd, output := newHelperCmd(t, "server_graceful", port)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		if strings.Contains(output.String(), "Starting TCP-Chat server on port "+port) {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("server did not start in time; output: %s", output.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("server helper exited with error: %v; output: %s", err, output.String())
	}

	if !strings.Contains(output.String(), "Server stopped gracefully") {
		t.Fatalf("expected graceful shutdown log, got: %s", output.String())
	}
}

func TestMainClientModeBannedBanner(t *testing.T) {
	ln, port := startMockTCPServer(t, "127.0.0.1:0", func(conn net.Conn) {
		defer conn.Close()
		_, _ = conn.Write([]byte("You are banned from this server for 1 minute.\n"))
	})
	defer ln.Close()

	cmd, output := newHelperCmd(t, "client_banned", port)
	if err := cmd.Run(); err != nil {
		t.Fatalf("client helper exited with error: %v; output: %s", err, output.String())
	}

	got := output.String()
	if !strings.Contains(got, "Connected to server") {
		t.Fatalf("expected connection confirmation, got: %s", got)
	}
	if !strings.Contains(got, "You are banned") {
		t.Fatalf("expected ban message in output, got: %s", got)
	}
}

func TestMainClientModeStartupPromptFlow(t *testing.T) {
	ln, port := startMockTCPServer(t, "127.0.0.1:0", func(conn net.Conn) {
		defer conn.Close()

		_, _ = conn.Write([]byte("Welcome from mock server\n" + namePrompt))

		r := bufio.NewReader(conn)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = r.ReadString('\n')
		_, _ = r.ReadString('\n')

		_, _ = conn.Write([]byte("Goodbye!\n"))
	})
	defer ln.Close()

	cmd, output := newHelperCmd(t, "client_prompt_flow", port)
	cmd.Stdin = strings.NewReader("alice\n/leave\n")
	if err := cmd.Run(); err != nil {
		t.Fatalf("client helper exited with error: %v; output: %s", err, output.String())
	}

	if !strings.Contains(output.String(), namePrompt) {
		t.Fatalf("expected startup prompt in output, got: %s", output.String())
	}
}

func TestMainClientModeDefaultPort8989(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:8989")
	if err != nil {
		t.Skipf("default port 8989 unavailable on this machine: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = fmt.Fprintln(conn, "You are banned from this server for 1 minute.")
	}()

	cmd, output := newHelperCmd(t, "client_default_port", "")
	if err := cmd.Run(); err != nil {
		t.Fatalf("default-port helper exited with error: %v; output: %s", err, output.String())
	}

	if !strings.Contains(output.String(), "You are banned") {
		t.Fatalf("expected output from server on default port 8989, got: %s", output.String())
	}
}

func TestMainClientModeUIPath(t *testing.T) {
	cmd, output := newHelperCmd(t, "client_ui_mode", "8989")
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected ui mode to fail in non-tty test environment")
	}

	got := output.String()
	if !strings.Contains(got, "Failed to create UI") && !strings.Contains(got, "UI error") {
		t.Fatalf("expected UI failure output, got: %s", got)
	}
}

func TestMainClientModeDialFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		ln.Close()
		t.Fatalf("failed to parse reserved port: %v", err)
	}
	ln.Close()

	cmd, output := newHelperCmd(t, "client_dial_fail", port)
	err = cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit for dial failure")
	}

	if !strings.Contains(output.String(), "Connection failed") {
		t.Fatalf("expected dial failure output, got: %s", output.String())
	}
}

func TestMainClientModeBannerReadFailure(t *testing.T) {
	ln, port := startMockTCPServer(t, "127.0.0.1:0", func(conn net.Conn) {
		_ = conn.Close()
	})
	defer ln.Close()

	cmd, output := newHelperCmd(t, "client_banner_read_fail", port)
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit for startup banner read failure")
	}

	if !strings.Contains(output.String(), "Failed to read banner") {
		t.Fatalf("expected startup banner failure output, got: %s", output.String())
	}
}

func TestMainServerModeWithOutputFlag(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		ln.Close()
		t.Fatalf("failed to parse reserved port: %v", err)
	}
	ln.Close()

	logFile := t.TempDir() + "/server.log"
	cmd, output := newHelperCmdWithOutputFile(t, "server_with_output", port, logFile)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("server helper exited with error: %v; output: %s", err, output.String())
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("expected output log file to be created: %v", err)
	}
	if !strings.Contains(string(data), "Starting TCP-Chat server on port "+port) {
		t.Fatalf("expected startup log line in output file, got: %s", string(data))
	}
}

func TestMainServerModeOutputOpenFailure(t *testing.T) {
	cmd, output := newHelperCmdWithOutputFile(t, "server_output_open_fail", "8989", "/definitely/not/real/path/server.log")
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid output file path")
	}

	if !strings.Contains(output.String(), "Error opening file") {
		t.Fatalf("expected output-file open error, got: %s", output.String())
	}
}

func TestMainServerModeListenFailure(t *testing.T) {
	cmd, output := newHelperCmd(t, "server_listen_fail", "")
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit for listen failure")
	}

	if !strings.Contains(output.String(), "Server error") {
		t.Fatalf("expected listen failure output, got: %s", output.String())
	}
}
