package user

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gen2brain/webp"
	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
)

const avatarOutputSize = 256

type StoredAvatar struct {
	Key    string
	URL    string
	Width  int
	Height int
	Format string
}

// AvatarStorage hides the persistence mechanism from the user service. The
// first implementation stores files on the local machine; an object-storage
// adapter can implement the same interface later without changing the API.
type AvatarStorage interface {
	Save(context.Context, uint64, []byte) (StoredAvatar, error)
	Delete(context.Context, string) error
}

type FileAvatarStorage struct {
	root    string
	baseURL string
}

func NewFileAvatarStorage(root, baseURL string) *FileAvatarStorage {
	return &FileAvatarStorage{root: root, baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/")}
}

func (s *FileAvatarStorage) Save(ctx context.Context, userID uint64, data []byte) (StoredAvatar, error) {
	if err := ctx.Err(); err != nil {
		return StoredAvatar{}, err
	}
	if s == nil || strings.TrimSpace(s.root) == "" {
		return StoredAvatar{}, fmt.Errorf("avatar file storage is not configured")
	}
	name := uuid.NewString() + ".webp"
	relative := filepath.ToSlash(filepath.Join("avatars", strconv.FormatUint(userID, 10), name))
	directory := filepath.Join(s.root, filepath.FromSlash(filepath.Dir(relative)))
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return StoredAvatar{}, fmt.Errorf("create avatar directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".avatar-*.tmp")
	if err != nil {
		return StoredAvatar{}, fmt.Errorf("create avatar temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = os.Remove(temporaryName)
	}()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return StoredAvatar{}, fmt.Errorf("write avatar: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return StoredAvatar{}, fmt.Errorf("close avatar: %w", err)
	}
	path := filepath.Join(s.root, filepath.FromSlash(relative))
	if err := os.Rename(temporaryName, path); err != nil {
		return StoredAvatar{}, fmt.Errorf("publish avatar: %w", err)
	}
	publicURL := "/" + relative
	if s.baseURL != "" {
		publicURL = s.baseURL + "/" + relative
	}
	return StoredAvatar{Key: relative, URL: publicURL, Width: avatarOutputSize, Height: avatarOutputSize, Format: "webp"}, nil
}

func (s *FileAvatarStorage) Delete(ctx context.Context, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || strings.TrimSpace(s.root) == "" {
		return nil
	}
	relative, ok := avatarStorageKey(value)
	if !ok {
		return nil
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, cleanPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("avatar path escapes storage directory")
	}
	if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func avatarStorageKey(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" {
		value = parsed.Path
	}
	value = strings.TrimLeft(filepath.ToSlash(value), "/")
	if !strings.HasPrefix(value, "avatars/") || !strings.HasSuffix(value, ".webp") || strings.Contains(value, "..") {
		return "", false
	}
	return value, true
}

func processAvatarImage(data []byte, maxDimension int) ([]byte, int, int, error) {
	format := detectAvatarFormat(data)
	if format == "" {
		return nil, 0, 0, fmt.Errorf("仅支持 JPG、PNG 和 WEBP 图片")
	}
	config, err := decodeAvatarConfig(bytes.NewReader(data), format)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("图片文件损坏或格式无效")
	}
	if config.Width < 1 || config.Height < 1 || config.Width > maxDimension || config.Height > maxDimension {
		return nil, 0, 0, fmt.Errorf("图片尺寸不能超过 %d×%d", maxDimension, maxDimension)
	}
	img, err := decodeAvatar(bytes.NewReader(data), format)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("图片解码失败")
	}
	if img.Bounds().Dx() < 1 || img.Bounds().Dy() < 1 {
		return nil, 0, 0, fmt.Errorf("图片尺寸无效")
	}

	source := img.Bounds()
	size := source.Dx()
	if source.Dy() < size {
		size = source.Dy()
	}
	crop := image.Rect(
		source.Min.X+(source.Dx()-size)/2,
		source.Min.Y+(source.Dy()-size)/2,
		source.Min.X+(source.Dx()-size)/2+size,
		source.Min.Y+(source.Dy()-size)/2+size,
	)
	destination := image.NewNRGBA(image.Rect(0, 0, avatarOutputSize, avatarOutputSize))
	xdraw.CatmullRom.Scale(destination, destination.Bounds(), img, crop, xdraw.Over, nil)
	var encoded bytes.Buffer
	if err := webp.Encode(&encoded, destination, webp.Options{Quality: 82, Method: 4}); err != nil {
		return nil, 0, 0, fmt.Errorf("图片编码失败")
	}
	return encoded.Bytes(), avatarOutputSize, avatarOutputSize, nil
}

func detectAvatarFormat(data []byte) string {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "jpeg"
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "png"
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "webp"
	}
	return ""
}

func decodeAvatarConfig(reader io.Reader, format string) (image.Config, error) {
	switch format {
	case "jpeg":
		return jpeg.DecodeConfig(reader)
	case "png":
		return png.DecodeConfig(reader)
	case "webp":
		return webp.DecodeConfig(reader)
	default:
		return image.Config{}, fmt.Errorf("unsupported avatar format")
	}
}

func decodeAvatar(reader io.Reader, format string) (image.Image, error) {
	switch format {
	case "jpeg":
		return jpeg.Decode(reader)
	case "png":
		return png.Decode(reader)
	case "webp":
		animation, err := webp.DecodeAll(reader)
		if err != nil {
			return nil, err
		}
		if len(animation.Image) != 1 {
			return nil, fmt.Errorf("animated webp is not allowed")
		}
		return animation.Image[0], nil
	default:
		return nil, fmt.Errorf("unsupported avatar format")
	}
}
