package abs

import (
	".."
	"bytes"
	"compress/gzip"
	"github.com/Azure/azure-sdk-for-go/storage"
	"math/rand"
	"strconv"
	"time"
)

// Azure Queue Storage configuration file.
type Config struct {
	Account    string `json:"account"`
	AccessKey  string `json:"access_key"`
	Container  string `json:"container"`
	BlobPath   string `json:"blob_path"`
	BufferSize int    `json:"buffer_size"`
}

// Checks is it valid or not
func (c *Config) IsValid() bool {
	return c.Account != "" && c.AccessKey != "" && c.BlobPath != "" && c.Container != ""
}

// Create a new StorageClient object based on a configuration file.
func (c *Config) NewClient() (dialects.StorageClient, error) {
	serviceClient, err := storage.NewBasicClient(c.Account, c.AccessKey)
	if err != nil {
		return nil, err
	}
	return &BlobStorage{
		Account:   c.Account,
		AccessKey: c.AccessKey,
		BlobPath:  c.BlobPath,
		Container: c.Container,
		Client:    serviceClient.GetBlobService()}, nil
}

// Azure Queue Storage dialect.
type BlobStorage struct {
	Account   string
	AccessKey string
	Container string
	BlobPath  string
	Client    storage.BlobStorageClient
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Generates an `n` length random string.
func (c *BlobStorage) RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// It is a buffered storage.
func (c *BlobStorage) IsBufferedStorage() bool {
	return true
}

// Get a random name for the blob
func (c *BlobStorage) GetRandomBlobPath() string {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	blobName := timestamp + "-" + c.RandStringBytes(20) + ".json.gz"
	blobPath := c.BlobPath + blobName
	return blobPath
}

// Compress the given string
func (c *BlobStorage) Compress(msg *string) (*bytes.Buffer, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(*msg)); err != nil {
		return &b, err
	}
	if err := gz.Flush(); err != nil {
		return &b, err
	}
	if err := gz.Close(); err != nil {
		return &b, err
	}
	return &b, nil
}

// Send a single Event into the Azure Queue Storage.
func (c *BlobStorage) Save(msg *string) error {
	buffer, err := c.Compress(msg)
	if err != nil {
		return err
	}
	if err := c.Client.CreateBlockBlobFromReader(c.Container, c.GetRandomBlobPath(),
		uint64(buffer.Len()), bytes.NewReader(buffer.Bytes()), nil); err != nil {
		return err
	}
	return nil
}