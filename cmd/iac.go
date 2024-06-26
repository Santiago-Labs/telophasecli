package cmd

import (
	"context"
	"fmt"
	"sync"

	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/santiago-labs/telophasecli/resourceoperation"
)

func runIAC(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	cmd int,
	accts []resource.Account,
) error {
	var wg sync.WaitGroup

	var once sync.Once
	var retError error

	for i := range accts {
		wg.Add(1)
		go func(acct resource.Account) {
			defer wg.Done()
			if !acct.IsProvisioned() {
				consoleUI.Print(fmt.Sprintf("skipping account: %s because it hasn't been provisioned yet", acct.AccountName), acct)
				return
			}

			ops, err := resourceoperation.CollectAccountOps(ctx, consoleUI, cmd, &acct, stacks)
			if err != nil {
				panic(oops.Wrapf(err, "error collecting account ops for acct: %s", acct.AccountID))
			}

			if len(ops) == 0 {
				consoleUI.Print("No stacks to deploy\n", acct)
				return
			}

			for _, op := range ops {
				if err := op.Call(ctx); err != nil {
					once.Do(func() {
						retError = err
					})
					consoleUI.Print(fmt.Sprintf("%v", err), acct)
					return
				}
			}
		}(accts[i])
	}

	wg.Wait()

	return retError
}
func contains(e string, s []string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
