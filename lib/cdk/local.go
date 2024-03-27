package cdk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"

	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

func TmpPath(acct ymlparser.Account, filePath string) string {
	hasher := sha256.New()
	hasher.Write([]byte(filePath))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return path.Join("telophasedirs", fmt.Sprintf("cdk-tmp%s-%s", acct.AccountID, hashString))
}
