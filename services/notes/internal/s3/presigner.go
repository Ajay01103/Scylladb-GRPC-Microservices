package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Presigner struct {
	client *s3.Client
	bucket string
}

func New(client *s3.Client, bucket string) *Presigner {
	return &Presigner{client: client, bucket: bucket}
}

func (p *Presigner) PutObject(ctx context.Context, key string, ttl time.Duration) (string, int64, error) {
	req, err := s3.NewPresignClient(p.client).PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return "", 0, fmt.Errorf("presign put object: %w", err)
	}
	expiresAt := time.Now().Add(ttl).Unix()
	return req.URL, expiresAt, nil
}

func (p *Presigner) GetObject(ctx context.Context, key string, ttl time.Duration) (string, int64, error) {
	req, err := s3.NewPresignClient(p.client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return "", 0, fmt.Errorf("presign get object: %w", err)
	}
	expiresAt := time.Now().Add(ttl).Unix()
	return req.URL, expiresAt, nil
}
