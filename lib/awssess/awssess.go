package awssess

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
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
