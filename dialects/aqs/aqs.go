package aqs

import (
	".."
	"github.com/Azure/azure-sdk-for-go/storage"
)

// Azure Queue Storage configuration file.
type Config struct {
	Account   string `json:"account"`
	AccessKey string `json:"access_key"`
	QueueName string `json:"queue_name"`
}

// Checks is it valid or not
func (c *Config) IsValid() bool {
	return c.Account != "" && c.AccessKey != "" && c.QueueName != ""
}

// Create a new StorageClient object based on a configuration file.
func (c *Config) NewClient() (dialects.StorageClient, error) {
	serviceClient, err := storage.NewBasicClient(c.Account, c.AccessKey)
	if err != nil {
		return nil, err
	}
	return &QueueStorage{
		Account:   c.Account,
		AccessKey: c.AccessKey,
		QueueName: c.QueueName,
		Client:    serviceClient.GetQueueService()}, nil
}

// Azure Queue Storage dialect.
type QueueStorage struct {
	Account   string
	AccessKey string
	QueueName string
	Client    storage.QueueServiceClient
}

// It is a buffered storage.
func (c *QueueStorage) IsBufferedStorage() bool {
	return false
}

// Send a single Event into the Azure Queue Storage.
func (c *QueueStorage) Save(msg *string) error {
	if err := c.Client.PutMessage(c.QueueName, *msg, storage.PutMessageParameters{}); err != nil {
		return err
	}
	return nil
}
