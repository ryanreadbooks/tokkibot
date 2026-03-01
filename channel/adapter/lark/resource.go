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

func (a *LarkAdapter) downloadImage(ctx context.Context, messageId, imageKey string) ([]byte, error) {
	req := imv1.NewGetMessageResourceReqBuilder().
		MessageId(messageId).
		Type("image").
		FileKey(imageKey).
		Build()

	resp, err := a.cli.Im.V1.MessageResource.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		return nil, fmt.Errorf("failed to get resource: %s", resp.ErrorResp())
	}

	data, err := io.ReadAll(resp.File)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (a *LarkAdapter) downloadFile(ctx context.Context, fileKey string) ([]byte, error) {
	req := imv1.NewGetFileReqBuilder().FileKey(fileKey).Build()

	resp, err := a.cli.Im.V1.File.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("failed to get file: %s", resp.ErrorResp())
	}

	data, err := io.ReadAll(resp.File)
	if err != nil {
		return nil, err
	}

	return data, nil
}
