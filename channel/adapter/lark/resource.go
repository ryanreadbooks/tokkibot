package lark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"

	imv1 "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/ryanreadbooks/tokkibot/pkg/audio"
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

// upload image
func (a *LarkAdapter) uploadMessageResourceImage(
	ctx context.Context,
	data []byte,
) (string, error) {
	req := imv1.NewCreateImageReqBuilder().
		Body(
			imv1.NewCreateImageReqBodyBuilder().
				ImageType("message").
				Image(bytes.NewReader(data)).
				Build(),
		).
		Build()

	resp, err := a.cli.Im.V1.Image.Create(ctx, req)
	if err != nil {
		return "", err
	}

	if !resp.Success() {
		return "", fmt.Errorf("failed to upload message resource: %s", resp.ErrorResp())
	}

	return *resp.Data.ImageKey, nil
}

type uploadFileType string

const (
	uploadFileTypeOpus   uploadFileType = "opus"
	uploadFileTypeMp4    uploadFileType = "mp4"
	uploadFileTypePdf    uploadFileType = "pdf"
	uploadFileTypeDoc    uploadFileType = "doc"
	uploadFileTypeXls    uploadFileType = "xls"
	uploadFileTypePpt    uploadFileType = "ppt"
	uploadFileTypeStream uploadFileType = "stream"
)

type messageResourceFileExtra struct {
	audioDuration int
}

func (a *LarkAdapter) uploadMessageResourceFile(
	ctx context.Context,
	fileType uploadFileType,
	fileName string,
	data []byte,
	extra *messageResourceFileExtra,
) (string, error) {
	bodyBuilder := imv1.NewCreateFileReqBodyBuilder().
		FileType(string(fileType)).
		FileName(fileName).
		File(bytes.NewReader(data))
	if fileType == uploadFileTypeOpus {
		if extra != nil {
			bodyBuilder.Duration(extra.audioDuration)
		}

		// try to convert to opus format
		// See: https://open.feishu.cn/document/server-docs/im-v1/file/create
		opusData, _, err := audio.ConvertToOpus(ctx, data, fileName)
		if err == nil {
			bodyBuilder.File(bytes.NewReader(opusData))
		} else {
			// log only
			slog.ErrorContext(ctx, "failed to convert audio to opus format", "error", err)
		}
	}

	body := bodyBuilder.Build()
	req := imv1.NewCreateFileReqBuilder().
		Body(body).
		Build()

	// 发起请求
	resp, err := a.cli.Im.V1.File.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("failed to upload message resource: %s", resp.ErrorResp())
	}

	return *resp.Data.FileKey, nil
}
