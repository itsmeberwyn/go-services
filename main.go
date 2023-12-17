package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

var (
	luaCmds   []*exec.Cmd
	luaCmdsMu sync.Mutex
)

func spawnLuaDaemon() (io.Writer, io.Reader) {
	luaCmd := exec.Command("luajit", "bot-service.lua")
	stdout, err := luaCmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating stdout pipe:", err)
		os.Exit(1)
	}

	stdin, err := luaCmd.StdinPipe()
	if err != nil {
		fmt.Println("Error creating stdin pipe:", err)
		os.Exit(1)
	}

	if err := luaCmd.Start(); err != nil {
		fmt.Println("Error starting lua service:", err)
		os.Exit(1)
	}

	luaCmdsMu.Lock()
	luaCmds = append(luaCmds, luaCmd)
	luaCmdsMu.Unlock()

	// for termination of connection
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func(cmd *exec.Cmd) {
		<-sigCh
		fmt.Printf("Received termination signal. Stopping lua service %d.\n", cmd.Process.Pid)
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()

		luaCmdsMu.Lock()
		defer luaCmdsMu.Unlock()

		// remove the service from the list of cmd
		for i, c := range luaCmds {
			if c == cmd {
				luaCmds = append(luaCmds[:i], luaCmds[i+1:]...)
				break
			}
		}
	}(luaCmd)

	return stdin, stdout
}

func main() {
	// Spawn two Lua daemons as an example
	stdins := make([]io.Writer, 0)
	stdouts := make([]io.Reader, 0)
	for i := 0; i < 2; i++ {
		stdin, stdout := spawnLuaDaemon()
		stdins = append(stdins, stdin)
		stdouts = append(stdouts, stdout)
	}

	// run go service
	for {
		luaCmdsMu.Lock()
		if len(luaCmds) == 0 {
			fmt.Println("No lua daemons running. Exiting.")
			luaCmdsMu.Unlock()
			os.Exit(0)
		}
		luaCmdsMu.Unlock()

		fmt.Print("Enter command for lua service (e.g., '/test'): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')

		for _, stdin := range stdins {
			io.WriteString(stdin, input)
		}

		luaCmdsMu.Lock()
		for i, cmd := range luaCmds {
			scanner := bufio.NewScanner(stdouts[i])
			if scanner.Scan() {
				result := scanner.Text()
				if result != "" {
					fmt.Printf("Result from lua %d: %s\n", cmd.Process.Pid, result)
				}
			}
		}
		luaCmdsMu.Unlock()
	}
}
