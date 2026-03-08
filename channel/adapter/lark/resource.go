package lark

import (
	"context"
	"fmt"
	"io"

	imv1 "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func wrapResourceKey(key string) string {
	return fmt.Sprintf("lark_%s", key)
}

func (a *LarkAdapter) downloadMessageResource(ctx context.Context, messageId, resourceType, resourceKey string) ([]byte, error) {
	req := imv1.NewGetMessageResourceReqBuilder().
		MessageId(messageId).
		Type(resourceType).
		FileKey(resourceKey).
		Build()

	resp, err := a.cli.Im.V1.MessageResource.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		return nil, fmt.Errorf("failed to get message resource: %s", resp.ErrorResp())
	}

	data, err := io.ReadAll(resp.File)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (a *LarkAdapter) downloadMessageResourceImage(ctx context.Context, messageId, imageKey string) ([]byte, error) {
	return a.downloadMessageResource(ctx, messageId, "image", imageKey)
}

func (a *LarkAdapter) downloadMessageResourceFile(ctx context.Context, messageId, fileKey string) ([]byte, error) {
	return a.downloadMessageResource(ctx, messageId, "file", fileKey)
}
