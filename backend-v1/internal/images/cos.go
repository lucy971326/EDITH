package images

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/tencentyun/cos-go-sdk-v5"
)

type cosStore interface {
	signPut(ctx context.Context, objectKey string) (string, error)
	signGet(ctx context.Context, objectKey string) (string, error)
	head(ctx context.Context, objectKey string) (objectInfo, error)
}

type objectInfo struct {
	MimeType  string
	SizeBytes int64
}

type client struct {
	client    *cos.Client
	secretID  string
	secretKey string
}

func newCOSStore(config Config) (*client, error) {
	endpoint, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region))
	if err != nil {
		return nil, fmt.Errorf("parse COS endpoint: %w", err)
	}
	return &client{
		client: cos.NewClient(&cos.BaseURL{BucketURL: endpoint}, &http.Client{
			Transport: &cos.AuthorizationTransport{SecretID: config.SecretID, SecretKey: config.SecretKey},
		}),
		secretID:  config.SecretID,
		secretKey: config.SecretKey,
	}, nil
}

func (c *client) signPut(ctx context.Context, objectKey string) (string, error) {
	url, err := c.client.Object.GetPresignedURL(ctx, http.MethodPut, objectKey, c.secretID, c.secretKey, SignedURLLifetime, nil)
	if err != nil {
		return "", fmt.Errorf("sign image upload URL: %w", err)
	}
	return url.String(), nil
}

func (c *client) signGet(ctx context.Context, objectKey string) (string, error) {
	url, err := c.client.Object.GetPresignedURL(ctx, http.MethodGet, objectKey, c.secretID, c.secretKey, SignedURLLifetime, nil)
	if err != nil {
		return "", fmt.Errorf("sign image read URL: %w", err)
	}
	return url.String(), nil
}

func (c *client) head(ctx context.Context, objectKey string) (objectInfo, error) {
	response, err := c.client.Object.Head(ctx, objectKey, nil)
	if err != nil {
		return objectInfo{}, err
	}
	size, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return objectInfo{}, fmt.Errorf("read uploaded image size: %w", err)
	}
	return objectInfo{MimeType: response.Header.Get("Content-Type"), SizeBytes: size}, nil
}
