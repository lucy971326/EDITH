package images

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

const signedURLLifetime = 5 * time.Minute

type cosClient interface {
	signPut(context.Context, string) (string, error)
	signGet(context.Context, string) (string, error)
	head(context.Context, string) (objectInfo, error)
	delete(context.Context, string) error
}

type objectInfo struct {
	mimeType  string
	sizeBytes int64
}

type cosStore struct {
	client              *cos.Client
	secretID, secretKey string
}

func newCOSClient(config Config) (*cosStore, error) {
	endpoint, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region))
	if err != nil {
		return nil, fmt.Errorf("parse COS endpoint: %w", err)
	}
	return &cosStore{client: cos.NewClient(&cos.BaseURL{BucketURL: endpoint}, &http.Client{Transport: &cos.AuthorizationTransport{SecretID: config.SecretID, SecretKey: config.SecretKey}}), secretID: config.SecretID, secretKey: config.SecretKey}, nil
}

func (c *cosStore) signPut(ctx context.Context, key string) (string, error) {
	value, err := c.client.Object.GetPresignedURL(ctx, http.MethodPut, key, c.secretID, c.secretKey, signedURLLifetime, nil)
	if err != nil {
		return "", fmt.Errorf("sign image upload URL: %w", err)
	}
	return value.String(), nil
}
func (c *cosStore) signGet(ctx context.Context, key string) (string, error) {
	value, err := c.client.Object.GetPresignedURL(ctx, http.MethodGet, key, c.secretID, c.secretKey, signedURLLifetime, nil)
	if err != nil {
		return "", fmt.Errorf("sign image read URL: %w", err)
	}
	return value.String(), nil
}
func (c *cosStore) head(ctx context.Context, key string) (objectInfo, error) {
	response, err := c.client.Object.Head(ctx, key, nil)
	if err != nil {
		return objectInfo{}, err
	}
	size, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return objectInfo{}, fmt.Errorf("read uploaded image size: %w", err)
	}
	return objectInfo{mimeType: response.Header.Get("Content-Type"), sizeBytes: size}, nil
}
func (c *cosStore) delete(ctx context.Context, key string) error {
	_, err := c.client.Object.Delete(ctx, key, nil)
	return err
}
