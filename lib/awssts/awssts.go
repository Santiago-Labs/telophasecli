package awssts

import (
	"strings"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/santiago-labs/telophasecli/lib/localstack"
)

func SetEnvironCreds(currEnv []string,
	creds *sts.Credentials,
	awsRegion *string) []string {
	var newEnv []string

	for _, e := range currEnv {
		if creds != nil {
			if strings.Contains(e, "AWS_ACCESS_KEY_ID=") ||
				strings.Contains(e, "AWS_SECRET_ACCESS_KEY=") ||
				strings.Contains(e, "AWS_SESSION_TOKEN=") {
				continue
			}
		}

		if awsRegion != nil && strings.Contains(e, "AWS_REGION=") {
			continue
		}

		newEnv = append(newEnv, e)
	}

	if creds != nil {
		newEnv = append(newEnv,
			"AWS_ACCESS_KEY_ID="+*creds.AccessKeyId,
			"AWS_SECRET_ACCESS_KEY="+*creds.SecretAccessKey,
			"AWS_SESSION_TOKEN="+*creds.SessionToken,
		)
	}

	if awsRegion != nil {
		newEnv = append(newEnv, *awsRegion)
	}

	if localstack.UsingLocalStack() {
		// We need to set this to true for localstack so that tflocal will use
		// the AWS key for the proper account.
		newEnv = append(newEnv, "CUSTOMIZE_ACCESS_KEY=true")
	}

	return newEnv
}
