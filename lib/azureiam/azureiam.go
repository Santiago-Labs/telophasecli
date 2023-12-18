package azureiam

import (
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/lib/copyutil"
)

const configDirKey = "AZURE_CONFIG_DIR"

// SetEnviron sets the AZURE_CONFIG_DIR environment variable to a newly created
// directory and handles the environment so that any subsequent azure calls with
// the returned environment default to the passed in subscriptionID.
func SetEnviron(currEnv []string,
	subscriptionID string) ([]string, error) {
	home := homePath(currEnv)

	azureConfigDir := path.Join(os.Getenv("HOME"), ".azure")

	var newEnv []string
	for _, e := range currEnv {
		if strings.Contains(e, configDirKey+"=") {
			azureConfigDir = strings.Split(e, "=")[1]
			continue
		}

		newEnv = append(newEnv, e)
	}

	subscriptionPath := path.Join(home, ".telophasedirs", "azureiam", subscriptionID)
	err := os.MkdirAll(subscriptionPath, os.ModePerm)
	if err != nil {
		return nil, oops.Wrapf(err, "creating subscription path %s", subscriptionPath)
	}

	newEnv = append(newEnv, configDirKey+"="+subscriptionPath)

	if err := copyutil.CopyDirectory(azureConfigDir, subscriptionPath); err != nil {
		return nil, oops.Wrapf(err, "copying azure config dir to %s", subscriptionPath)
	}

	cmd := exec.Command("az", "account", "set", "--subscription", subscriptionID)
	cmd.Env = newEnv
	if _, err := cmd.CombinedOutput(); err != nil {
		return nil, oops.Wrapf(err, "setting default subscription to %s", subscriptionID)
	}

	return newEnv, nil
}

func homePath(currEnv []string) string {
	home := os.Getenv("HOME")
	for _, e := range currEnv {
		if strings.Contains(e, "HOME=") {
			home = strings.Split(e, "=")[1]
			continue
		}
	}

	return home
}
