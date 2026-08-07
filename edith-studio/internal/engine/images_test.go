package engine

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

func imageDataURL(mime string, data []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func TestDecodeImageDataURL(t *testing.T) {
	mime, data, err := decodeImageDataURL(imageDataURL("image/png", []byte("fake-png")))
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" || string(data) != "fake-png" {
		t.Fatalf("mime = %q, data = %q", mime, data)
	}

	for _, invalid := range []string{"", "image/png", "data:text/plain;base64,AAAA", "data:image/png;base64,!!!not-base64!!!"} {
		if _, _, err := decodeImageDataURL(invalid); !errors.Is(err, ErrInvalidImage) {
			t.Fatalf("decodeImageDataURL(%q) error = %v, want ErrInvalidImage", invalid, err)
		}
	}
}

func TestValidateImages(t *testing.T) {
	ok := ImageInput{Name: "a.png", DataURL: imageDataURL("image/png", []byte("png"))}
	if err := validateImages([]ImageInput{ok}); err != nil {
		t.Fatal(err)
	}

	if err := validateImages([]ImageInput{{Name: "b.txt", DataURL: imageDataURL("text/plain", []byte("x"))}}); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("unsupported mime error = %v", err)
	}

	tooLarge := ImageInput{Name: "big.png", DataURL: imageDataURL("image/png", make([]byte, maxImageBytes+1))}
	if err := validateImages([]ImageInput{tooLarge}); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("oversize error = %v", err)
	}

	tooMany := make([]ImageInput, maxImagesPerRun+1)
	for i := range tooMany {
		tooMany[i] = ok
	}
	if err := validateImages(tooMany); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("too many images error = %v", err)
	}
}

func TestBuildUserMessage(t *testing.T) {
	message, err := buildUserMessage(RunInput{
		Message: "看看这张图",
		Images: []ImageInput{
			{Name: "a.png", DataURL: imageDataURL("image/png", []byte("png-bytes"))},
			{Name: "b.jpeg", DataURL: imageDataURL("image/jpeg", []byte("jpeg-bytes"))},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if message.Content != "看看这张图" {
		t.Fatalf("content = %q", message.Content)
	}
	if len(message.ContentParts) != 2 {
		t.Fatalf("content parts = %d, want 2", len(message.ContentParts))
	}
	for _, part := range message.ContentParts {
		if part.Type != model.ContentTypeImage || part.Image == nil {
			t.Fatalf("part = %#v, want image", part)
		}
		if len(part.Image.Data) == 0 {
			t.Fatalf("image data is empty")
		}
	}
	if !strings.Contains(message.ContentParts[0].Image.Format, "png") {
		t.Fatalf("format = %q", message.ContentParts[0].Image.Format)
	}
}
