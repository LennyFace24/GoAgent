package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const chromaRequestTimeout = 30 * time.Second

type ChromaStore struct {
	baseURL    string
	httpClient *http.Client
}

func NewChromaStore(baseURL string) *ChromaStore {
	return &ChromaStore{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: chromaRequestTimeout,
		},
	}
}

func (s *ChromaStore) CreateCollection(name string) error {
	body := map[string]string{
		"collection_name": name,
	}
	_, err := s.doRequest("POST", "/collection/create", body)
	return err
}

type InsertRequest struct {
	CollectionName string           `json:"collection_name"`
	IDs            []string         `json:"ids"`
	Embeddings     [][]float64      `json:"embeddings"`
	Documents      []string         `json:"documents,omitempty"`
	Metadatas      []map[string]any `json:"metadatas,omitempty"`
}

func (s *ChromaStore) Insert(collectionName string, ids []string, embeddings [][]float64, documents []string, metadatas []map[string]any) error {
	body := InsertRequest{
		CollectionName: collectionName,
		IDs:            ids,
		Embeddings:     embeddings,
		Documents:      documents,
		Metadatas:      metadatas,
	}
	_, err := s.doRequest("POST", "/insert", body)
	return err
}

type SearchResult struct {
	IDs       [][]string  `json:"ids"`
	Distances [][]float64 `json:"distances"`
	Documents [][]string  `json:"documents"`
}

type SearchResponse struct {
	Status  string       `json:"status"`
	Results SearchResult `json:"results"`
}

func (s *ChromaStore) Search(collectionName string, queryEmbedding []float64, topK int) (*SearchResult, error) {
	body := map[string]interface{}{
		"collection_name":  collectionName,
		"query_embeddings": [][]float64{queryEmbedding},
		"n_results":        topK,
		"include":          []string{"documents", "distances"},
	}

	resp, err := s.doRequest("POST", "/search", body)
	if err != nil {
		return nil, err
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(resp, &searchResp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	return &searchResp.Results, nil
}

func (s *ChromaStore) Delete(collectionName string, ids []string) error {
	body := map[string]interface{}{
		"collection_name": collectionName,
		"ids":             ids,
	}
	_, err := s.doRequest("POST", "/delete", body)
	return err
}

// ChromaDocument 表示 Chroma 中的一条文档记录。
type ChromaDocument struct {
	ID       string         `json:"id"`
	Document string         `json:"document"`
	Metadata map[string]any `json:"metadata"`
}

type documentsResponse struct {
	Status    string           `json:"status"`
	Documents []ChromaDocument `json:"documents"`
}

// ListDocuments 返回 collection 中的全部文档记录。
func (s *ChromaStore) ListDocuments(collectionName string) ([]ChromaDocument, error) {
	path := fmt.Sprintf("/documents?collection_name=%s", url.QueryEscape(collectionName))
	resp, err := s.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result documentsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse documents response: %w", err)
	}
	return result.Documents, nil
}

// DeleteBySource 删除 metadata.source 等于 source 的所有向量。
func (s *ChromaStore) DeleteBySource(collectionName, source string) error {
	path := fmt.Sprintf("/documents?collection_name=%s&source=%s",
		url.QueryEscape(collectionName), url.QueryEscape(source))
	_, err := s.doRequest("DELETE", path, nil)
	return err
}

func (s *ChromaStore) Health() error {
	_, err := s.doRequest("GET", "/health", nil)
	return err
}

func (s *ChromaStore) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, s.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
