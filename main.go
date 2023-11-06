package main

import (
	"os/exec"

	"github.com/santiago-labs/telophasecli/cmd"
)

func main() {
	cmdStr := "cdk"
	cdkArgs := []string{"--version"}

	cmdCdk := exec.Command(cmdStr, cdkArgs...)
	if err := cmdCdk.Run(); err != nil {
		panic("install cdk before running telophasecli. You can install by running `npm install -g aws-cdk`")
	}
	cmd.Execute()
}
