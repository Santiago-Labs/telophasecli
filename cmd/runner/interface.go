package runner

import (
	"os/exec"

	"github.com/santiago-labs/telophasecli/resource"
)

type ConsoleUI interface {
	Print(string, resource.Account)
	RunCmd(*exec.Cmd, resource.Account) error
	Start()
}
