package runner

import (
	"os/exec"

	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

type ConsoleUI interface {
	Print(string, ymlparser.Account)
	RunCmd(*exec.Cmd, ymlparser.Account) error
	PostProcess()
}
