package engine

import (
	"encoding/base64"
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	maxImagesPerRun = 5
	maxImageBytes   = 10 << 20 // 单张图片解码后上限 10 MiB
)

// allowedImageMIMEs 是后端允许的图片格式白名单。
var allowedImageMIMEs = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// decodeImageDataURL 解析 data:<mime>;base64,<payload>，返回 mime 与原始字节。
func decodeImageDataURL(dataURL string) (mime string, data []byte, err error) {
	payload, ok := strings.CutPrefix(dataURL, "data:")
	if !ok {
		return "", nil, ErrInvalidImage
	}
	header, encoded, ok := strings.Cut(payload, ",")
	if !ok || !strings.HasSuffix(header, ";base64") {
		return "", nil, ErrInvalidImage
	}
	mime = strings.TrimSuffix(header, ";base64")
	if !strings.HasPrefix(mime, "image/") {
		return "", nil, ErrInvalidImage
	}
	data, err = base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", nil, ErrInvalidImage
	}
	return mime, data, nil
}

// validateImages 检查图片数量、格式和解码后大小。
func validateImages(images []ImageInput) error {
	if len(images) > maxImagesPerRun {
		return fmt.Errorf("%w: at most %d images per message", ErrInvalidImage, maxImagesPerRun)
	}
	for _, image := range images {
		mime, data, err := decodeImageDataURL(image.DataURL)
		if err != nil {
			return err
		}
		if !allowedImageMIMEs[mime] {
			return fmt.Errorf("%w: unsupported image type %q", ErrInvalidImage, mime)
		}
		if len(data) > maxImageBytes {
			return fmt.Errorf("%w: image %q exceeds %d MiB", ErrInvalidImage, image.Name, maxImageBytes>>20)
		}
	}
	return nil
}

// buildUserMessage 构造带文本和图片的用户消息；图片以原始字节注入 ContentParts。
func buildUserMessage(input RunInput) (model.Message, error) {
	message := model.NewUserMessage(input.Message)
	for _, image := range input.Images {
		mime, data, err := decodeImageDataURL(image.DataURL)
		if err != nil {
			return model.Message{}, err
		}
		// detail 留空让供应商使用默认值：OpenAI 默认 auto，MiniMax 默认 default，
		// 避免写死某个供应商不接受的取值。
		message.AddImageData(data, "", strings.TrimPrefix(mime, "image/"))
	}
	return message, nil
}
