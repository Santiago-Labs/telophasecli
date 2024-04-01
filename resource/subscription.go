package resource

type Subscription struct {
	SubscriptionName string   `yaml:"Name"`
	Account          *Account `yaml:"Account"`
}
