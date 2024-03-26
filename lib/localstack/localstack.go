package localstack

import "os"

func UsingLocalStack() bool {
	if os.Getenv("LOCALSTACK") != "" {
		return true
	}
	return false
}

func CdkCmd() string {
	if UsingLocalStack() {
		return "cdklocal"
	}
	return "cdk"
}

func TfCmd() string {
	if UsingLocalStack() {
		return "tflocal"
	}
	return "terraform"
}
