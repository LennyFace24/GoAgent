package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/LennyFace24/MiniAgent/internal/store"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/document"
)

type FileService struct {
	loader      *file.FileLoader
	splitter    document.Transformer
	embedder    *openai.Embedder
	vectorStore *store.ChromaStore
}

func NewFileService() *FileService {
	ctx := context.Background()
	cfg := config.GetConfig()

	// 创建 file loader
	loader, _ := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		UseNameAsID: true,
	})

	// 创建 markdown splitter
	splitter, _ := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"#":   "title",
			"##":  "section",
			"###": "subsection",
		},
	})

	// 创建 Embedding
	embedder, _ := openai.NewEmbedder(ctx,
		&openai.EmbeddingConfig{
			Model:   cfg.Embedding.Model,
			APIKey:  cfg.Embedding.ApiKey,
			BaseURL: cfg.Embedding.BaseUrl,
		})

	// 创建 ChromaDB 存储
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
	}
}

func (s *FileService) ProcessFile(ctx context.Context, file multipart.File, filename string) error {
	// 1. 保存到临时文件
	tmpFile,err := os.CreateTemp("", "upload-*"+filepath.Ext(filename))
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		os.Remove(tmpFile.Name())
	}()

	// copy file content to temp file
	if _,err := io.Copy(tmpFile,file);err != nil {
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

	// 4. 准备数据（生成唯一 ID）
	ids := make([]string, len(chunks))
	contents := make([]string, len(chunks))
	timestamp := time.Now().UnixMilli()
	for i, chunk := range chunks {
		ids[i] = fmt.Sprintf("%s_%d_%d", chunk.ID, timestamp, i)
		contents[i] = chunk.Content
	}

	// 5. 向量化
	embeddings, err := s.embedder.EmbedStrings(ctx, contents)
	if err != nil {
		return fmt.Errorf("embed documents: %w", err)
	}

	// 6. 存储到 ChromaDB
	collectionName := "documents"
	if err := s.vectorStore.CreateCollection(collectionName); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}

	if err := s.vectorStore.Insert(collectionName, ids, embeddings, contents); err != nil {
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
	collectionName := "documents"
	results, err := s.vectorStore.Search(collectionName, queryEmbeddings[0], topK)
	if err != nil {
		return nil, fmt.Errorf("search vectors: %w", err)
	}

	// 3. 返回文档内容
	if len(results.Documents) == 0 {
		return nil, nil
	}

	return results.Documents[0], nil
}
