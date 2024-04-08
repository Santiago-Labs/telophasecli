package runner

import (
	"bufio"
	"fmt"
	"os/exec"
	"sync"

	"github.com/santiago-labs/telophasecli/lib/colors"
	"github.com/santiago-labs/telophasecli/resource"
)

func NewSTDOut() ConsoleUI {
	return &stdOut{
		coloredId: make(map[string]string),
		lock:      sync.Mutex{},
	}
}

type stdOut struct {
	coloredId map[string]string
	lock      sync.Mutex
}

func (s *stdOut) ColoredId(acct resource.Account) string {
	s.lock.Lock()
	defer s.lock.Unlock()

	coloredId, ok := s.coloredId[acct.ID()]
	if !ok {
		colorFunc := colors.DeterministicColorFunc(acct.AccountID)
		if acct.AccountName != "" {
			coloredId = colorFunc(fmt.Sprintf("[Account: %s (%s)]", acct.ID(), acct.AccountName))
		} else {
			coloredId = colorFunc("[Account: " + acct.ID() + "]")
		}
		s.coloredId[acct.ID()] = coloredId
	}
	return coloredId
}

// RunCmd takes the command and acct and runs it while prepending the
// coloredAccountID from stderr and stdout and printing it.
func (s *stdOut) RunCmd(cmd *exec.Cmd, acct resource.Account) error {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("[ERROR] %s %v", s.ColoredId(acct), err)
	}
	stdoutScanner := bufio.NewScanner(stdoutPipe)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("[ERROR] %s %v", s.ColoredId(acct), err)
	}
	stderrScanner := bufio.NewScanner(stderrPipe)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[ERROR] %s %v", s.ColoredId(acct), err)
	}

	var scannerWg sync.WaitGroup
	scannerWg.Add(2)
	scanF := func(scanner *bufio.Scanner, _ string) {
		defer scannerWg.Done()
		for scanner.Scan() {
			fmt.Printf("%s %s\n", s.ColoredId(acct), scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("[ERROR] %s %v\n", s.ColoredId(acct), err)
			return
		}
	}

	go scanF(stdoutScanner, "stdout")
	go scanF(stderrScanner, "stderr")
	scannerWg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("[ERROR] %s %v", s.ColoredId(acct), err)
	}

	return nil
}

func (s *stdOut) Print(msg string, acct resource.Account) {
	fmt.Printf("%s %v\n", s.ColoredId(acct), msg)
}

func (s *stdOut) Start() {}
