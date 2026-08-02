package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	imageModel = "gemini-2.5-flash-image"
	ttsModel   = "gemini-2.5-flash-preview-tts"

	// PCM 16-bit 24kHz mono — định dạng audio Gemini TTS trả về.
	wavSampleRate = 24000
	wavChannels   = 1
	wavBits       = 16
)

// GenerateImage tạo ảnh từ prompt và ghi ra file PNG dstPNG.
func (c *Client) GenerateImage(ctx context.Context, prompt, dstPNG string) error {
	req := genRequest{
		Contents:         []reqContent{{Parts: []reqPart{{Text: prompt}}}},
		GenerationConfig: map[string]any{"responseModalities": []string{"IMAGE", "TEXT"}},
	}
	resp, err := c.generate(ctx, imageModel, req)
	if err != nil {
		return err
	}
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.InlineData == nil || !strings.HasPrefix(p.InlineData.MimeType, "image/") {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
		if err != nil {
			return fmt.Errorf("giải mã ảnh base64 từ Gemini: %w", err)
		}
		if dir := filepath.Dir(dstPNG); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(dstPNG, raw, 0o644); err != nil {
			return fmt.Errorf("ghi file ảnh %s: %w", dstPNG, err)
		}
		return nil
	}
	return errors.New("Gemini không trả về ảnh nào — thử đổi prompt hoặc thử lại sau")
}

// TTS tổng hợp giọng nói (giọng voice) và ghi ra file WAV dstWav.
func (c *Client) TTS(ctx context.Context, text, voice, dstWav string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("nội dung TTS trống")
	}
	req := genRequest{
		Contents: []reqContent{{Parts: []reqPart{{Text: text}}}},
		GenerationConfig: map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]any{
				"voiceConfig": map[string]any{
					"prebuiltVoiceConfig": map[string]any{"voiceName": voice},
				},
			},
		},
	}
	resp, err := c.generate(ctx, ttsModel, req)
	if err != nil {
		return err
	}
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.InlineData == nil || !strings.HasPrefix(p.InlineData.MimeType, "audio/") {
			continue
		}
		pcm, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
		if err != nil {
			return fmt.Errorf("giải mã audio base64 từ Gemini: %w", err)
		}
		return writeWAV(dstWav, pcm)
	}
	return errors.New("Gemini không trả về audio nào — kiểm tra tên giọng đọc (Kore, Puck, Charon, Fenrir, Aoede) hoặc thử lại")
}

// writeWAV bọc dữ liệu PCM 16-bit 24kHz mono bằng header WAV 44 byte (RIFF/fmt/data).
func writeWAV(path string, pcm []byte) error {
	byteRate := wavSampleRate * wavChannels * wavBits / 8
	blockAlign := wavChannels * wavBits / 8

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	le(&buf, uint32(36+len(pcm)))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	le(&buf, uint32(16))              // kích thước chunk fmt
	le(&buf, uint16(1))               // PCM
	le(&buf, uint16(wavChannels))     // mono
	le(&buf, uint32(wavSampleRate))   // 24000 Hz
	le(&buf, uint32(byteRate))        // 48000 B/s
	le(&buf, uint16(blockAlign))      // 2
	le(&buf, uint16(wavBits))         // 16 bit
	buf.WriteString("data")
	le(&buf, uint32(len(pcm)))
	buf.Write(pcm)

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("ghi file WAV %s: %w", path, err)
	}
	return nil
}

func le(buf *bytes.Buffer, v any) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}
