package awssess

import (
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/samsarahq/go/oops"
)

func DefaultSession(cfgs ...*aws.Config) (*session.Session, error) {
	if os.Getenv("LOCALSTACK") != "" {
		cfg := aws.NewConfig()
		cfg.Endpoint = aws.String("http://localhost:4566")
		cfgs = append(cfgs, cfg)
	}

	sess, err := session.NewSession(cfgs...)
	if err != nil {
		return nil, oops.Wrapf(err, "new session")
	}
	return sess, nil
}

func AssumeRole(svc *sts.STS, input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	if os.Getenv("LOCALSTACK") != "" {
		// Localstack doesn't handle IAM checks so let everything through.
		return &sts.AssumeRoleOutput{
			Credentials: &sts.Credentials{
				// The accessKeyId needs to equal the target accountID for operations to happen on multiple accounts.

				AccessKeyId:     aws.String(RoleARNToAccountID(*input.RoleArn)),
				SecretAccessKey: aws.String("fake"),
				SessionToken:    aws.String("fake"),
			},
		}, nil
	}

	result, err := svc.AssumeRole(input)
	if err != nil {
		return nil, oops.Wrapf(err, "assume role")
	}
	return result, nil
}

// RoleARN to accountID.
func RoleARNToAccountID(roleARN string) string {
	parts := strings.Split(roleARN, ":")
	if len(parts) < 4 {
		return ""
	}

	return parts[4]
}
