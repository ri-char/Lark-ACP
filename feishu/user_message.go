package feishu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/logger"
)

type ImgResourcePair struct {
	ImageKey string
	MessageId string
}

type UserMsgBuffer struct {
	imageBuffer []ImgResourcePair
	mu sync.Mutex
}

func (b*UserMsgBuffer) AddImage(key ImgResourcePair) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.imageBuffer = append(b.imageBuffer, key)
}

func (b*UserMsgBuffer) GetAndClearImages() []ImgResourcePair {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := b.imageBuffer
	b.imageBuffer = nil
	return result
}

func FeishuMsgToPrompt(ctx context.Context, msg *UserMsgBuffer, supportImagePrompt bool, msgId *string, msgType, content string) ([]acpsdk.ContentBlock, error) {
	var result []acpsdk.ContentBlock
	switch msgType {
	case "text":
		var textContent struct {
			Text string `json:"text"`
		}
		images := msg.GetAndClearImages()
		if len(images) != 0 {
			for _, image := range images {
				imageReader, contentType, err := GetImageInMessage(ctx, image.ImageKey, image.MessageId)
				if err != nil {
					logger.Warn("FeishuMsgToPrompt get image data error", "err", err)
					continue
				}
				var base64Image bytes.Buffer
				b64Encoder := base64.NewEncoder(base64.RawStdEncoding, &base64Image)
				_, err = io.Copy(b64Encoder, imageReader)
				if err != nil {
					logger.Warn("FeishuMsgToPrompt read image data error", "err", err)
					continue
				}

				result = append(result, acpsdk.ImageBlock(base64Image.String(), contentType))
			}
		}

		if err := json.Unmarshal([]byte(content), &textContent); err == nil {
			content = textContent.Text
			result = append(result, acpsdk.TextBlock(content))
		}
	case "image":
		if !supportImagePrompt {
			return nil, fmt.Errorf("当前Agent不支持图片")
		}
		if msgId == nil {
			return nil, fmt.Errorf("图片消息，但是message id为空")
		}
		var imageContent struct {
			ImageKey string `json:"image_key"`
		}
		if err := json.Unmarshal([]byte(content), &imageContent); err == nil {
			msg.AddImage(ImgResourcePair{
				ImageKey: imageContent.ImageKey,
				MessageId: *msgId,
			})
		}
	default:
		return nil, fmt.Errorf("无法处理消息类型%s", msgType)
	}
	return result, nil

}
