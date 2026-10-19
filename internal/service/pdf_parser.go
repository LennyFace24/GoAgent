package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/ledongthuc/pdf"
)

const (
	pdfChunkRunes   = 3000
	pdfChunkOverlap = 0
)

// pdfTextParser 提取 PDF 文本并按固定长度切块。
// 默认的 TextParser 会把 PDF 二进制当文本，Markdown 分片器通常得到空数组，
// 最终向 embedding 服务发送空 input 并返回 400。
type pdfTextParser struct{}

func (pdfTextParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) (docs []*schema.Document, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			docs = nil
			err = fmt.Errorf("解析 PDF 失败: %v", recovered)
		}
	}()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 PDF 失败: %w", err)
	}

	pdfReader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("打开 PDF 失败: %w", err)
	}
	textReader, err := pdfReader.GetPlainText()
	if err != nil {
		return nil, fmt.Errorf("提取 PDF 文本失败: %w", err)
	}
	text, err := io.ReadAll(textReader)
	if err != nil {
		return nil, fmt.Errorf("读取 PDF 文本失败: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cleaned := sanitizePDFText(string(text))
	if strings.TrimSpace(cleaned) == "" {
		return nil, fmt.Errorf("PDF 中未提取到文本，可能是扫描件或字体不受支持")
	}

	chunks := splitPDFText(cleaned, pdfChunkRunes, pdfChunkOverlap)
	docs = make([]*schema.Document, 0, len(chunks))
	for _, chunk := range chunks {
		docs = append(docs, &schema.Document{Content: chunk})
	}
	return docs, nil
}

func sanitizePDFText(text string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r < 32 || r == 0x7f:
			return -1
		default:
			return r
		}
	}, text)
}

func splitPDFText(text string, maxRunes, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return []string{text}
	}
	if maxRunes <= 0 {
		maxRunes = pdfChunkRunes
	}
	if overlap < 0 || overlap >= maxRunes {
		overlap = 0
	}

	chunks := make([]string, 0, len(runes)/maxRunes+1)
	for start := 0; start < len(runes); {
		end := start + maxRunes
		if end >= len(runes) {
			end = len(runes)
		} else {
			// 尽量在换行处切开，避免把句子截断。
			for i := end - 1; i > start+maxRunes/2; i-- {
				if runes[i] == '\n' {
					end = i
					break
				}
			}
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(runes) {
			break
		}
		start = end
		if overlap > 0 && start > overlap {
			start -= overlap
		}
	}
	return chunks
}
