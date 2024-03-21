package awssts

import (
	"strings"
)

func SetEnviron(currEnv []string,
	accessKeyID,
	secretAccessKey,
	sessionToken string) []string {
	var newEnv []string
	for _, e := range currEnv {
		if strings.Contains(e, "AWS_ACCESS_KEY_ID=") ||
			strings.Contains(e, "AWS_SECRET_ACCESS_KEY=") ||
			strings.Contains(e, "AWS_SESSION_TOKEN=") {
			continue
		}

		newEnv = append(newEnv, e)
	}

	newEnv = append(newEnv,
		"AWS_ACCESS_KEY_ID="+accessKeyID,
		"AWS_SECRET_ACCESS_KEY="+secretAccessKey,
		"AWS_SESSION_TOKEN="+sessionToken,
		"AWS_REGION="+"us-west-2")

	return newEnv
}
