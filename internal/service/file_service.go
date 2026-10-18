package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LennyFace24/MiniAgent/internal/store"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/document"
)

// DocumentCollection 是所有知识库文档共用的 Chroma collection。
const DocumentCollection = "documents"

type FileService struct {
	loader      *file.FileLoader
	splitter    document.Transformer
	embedder    *openai.Embedder
	vectorStore *store.ChromaStore
}

func NewFileService() (*FileService, error) {
	ctx := context.Background()
	cfg := config.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("配置未加载")
	}

	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		UseNameAsID: true,
	})
	if err != nil {
		return nil, fmt.Errorf("创建文件加载器失败: %w", err)
	}

	splitter, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"#":   "title",
			"##":  "section",
			"###": "subsection",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("创建文档分片器失败: %w", err)
	}

	embedder, err := openai.NewEmbedder(ctx,
		&openai.EmbeddingConfig{
			Model:   cfg.Embedding.Model,
			APIKey:  cfg.Embedding.ApiKey,
			BaseURL: cfg.Embedding.BaseUrl,
		})
	if err != nil {
		return nil, fmt.Errorf("创建 Embedding 客户端失败: %w", err)
	}

	chromaURL := cfg.ChromaDB.URL
	if chromaURL == "" {
		chromaURL = "http://localhost:8088"
	}
	chromaStore := store.NewChromaStore(chromaURL)

	return &FileService{
		loader:      loader,
		splitter:    splitter,
		embedder:    embedder,
		vectorStore: chromaStore,
	}, nil
}

func (s *FileService) ProcessFile(ctx context.Context, file multipart.File, filename string) error {
	// 1. 保存到临时文件
	tmpFile, err := os.CreateTemp("", "upload-*"+filepath.Ext(filename))
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		os.Remove(tmpFile.Name())
	}()

	// copy file content to temp file
	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		return fmt.Errorf("save uploaded file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// 2. 加载文档
	docs, err := s.loader.Load(ctx, document.Source{URI: tmpFile.Name()})
	if err != nil {
		return fmt.Errorf("load document: %w", err)
	}

	// 3. 分片
	chunks, err := s.splitter.Transform(ctx, docs)
	if err != nil {
		return fmt.Errorf("split document: %w", err)
	}

	// 4. 准备数据（生成唯一 ID，并写入来源文件名）
	ids := make([]string, len(chunks))
	contents := make([]string, len(chunks))
	metadatas := make([]map[string]any, len(chunks))
	timestamp := time.Now().UnixMilli()
	for i, chunk := range chunks {
		ids[i] = fmt.Sprintf("%s_%d_%d", chunk.ID, timestamp, i)
		contents[i] = chunk.Content
		metadatas[i] = map[string]any{
			"source": filename,
			"chunk":  i,
		}
	}

	// 5. 向量化
	embeddings, err := s.embedder.EmbedStrings(ctx, contents)
	if err != nil {
		return fmt.Errorf("embed documents: %w", err)
	}

	// 6. 存储到 ChromaDB。先清理同名文件的旧向量，保证一个文件只保留一组数据。
	if err := s.vectorStore.CreateCollection(DocumentCollection); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	if err := s.vectorStore.DeleteBySource(DocumentCollection, filename); err != nil {
		return fmt.Errorf("delete previous document vectors: %w", err)
	}

	if err := s.vectorStore.Insert(DocumentCollection, ids, embeddings, contents, metadatas); err != nil {
		return fmt.Errorf("insert vectors: %w", err)
	}

	return nil
}

func (s *FileService) Search(ctx context.Context, query string, topK int) ([]string, error) {
	// 1. 向量化查询
	queryEmbeddings, err := s.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// 2. 搜索
	results, err := s.vectorStore.Search(DocumentCollection, queryEmbeddings[0], topK)
	if err != nil {
		return nil, fmt.Errorf("search vectors: %w", err)
	}

	// 3. 返回文档内容
	if len(results.Documents) == 0 {
		return nil, nil
	}

	return results.Documents[0], nil
}

// ListDocuments 返回知识库中已入库的文件名（去重、排序）。
func (s *FileService) ListDocuments(ctx context.Context) ([]string, error) {
	documents, err := s.vectorStore.ListDocuments(DocumentCollection)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, doc := range documents {
		source, _ := doc.Metadata["source"].(string)
		source = strings.TrimSpace(source)
		if source == "" || seen[source] {
			continue
		}
		seen[source] = true
		names = append(names, source)
	}
	sort.Strings(names)
	return names, nil
}

// DeleteDocument 删除指定来源文件的所有向量。
func (s *FileService) DeleteDocument(ctx context.Context, source string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return fmt.Errorf("文件名不能为空")
	}
	if err := s.vectorStore.DeleteBySource(DocumentCollection, source); err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}
