package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssecrets "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	awstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type awsProvider struct {
	client *awssecrets.Client
}

func (p *awsProvider) Put(ctx context.Context, ref, value string) error {
	_, err := p.client.CreateSecret(ctx, &awssecrets.CreateSecretInput{
		Name:         aws.String(ref),
		SecretString: aws.String(value),
	})
	if err == nil {
		return nil
	}

	var exists *awstypes.ResourceExistsException
	if errors.As(err, &exists) {
		_, putErr := p.client.PutSecretValue(ctx, &awssecrets.PutSecretValueInput{
			SecretId:     aws.String(ref),
			SecretString: aws.String(value),
		})
		if putErr != nil {
			return fmt.Errorf("put aws secret value: %w", putErr)
		}
		return nil
	}

	return fmt.Errorf("create aws secret: %w", err)
}

func (p *awsProvider) Get(ctx context.Context, ref string) (string, error) {
	out, err := p.client.GetSecretValue(ctx, &awssecrets.GetSecretValueInput{SecretId: aws.String(ref)})
	if err != nil {
		return "", fmt.Errorf("get aws secret value: %w", err)
	}
	if out.SecretString == nil {
		return "", errors.New("aws secret has no string value")
	}
	return *out.SecretString, nil
}

func (p *awsProvider) Delete(ctx context.Context, ref string) error {
	_, err := p.client.DeleteSecret(ctx, &awssecrets.DeleteSecretInput{
		SecretId:                   aws.String(ref),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("delete aws secret: %w", err)
	}
	return nil
}
