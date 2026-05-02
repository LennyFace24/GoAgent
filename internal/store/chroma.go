package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ChromaStore struct {
	baseURL    string
	httpClient *http.Client
}

func NewChromaStore(baseURL string) *ChromaStore {
	return &ChromaStore{
		baseURL:    baseURL,
		httpClient: &http.Client{},
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
	CollectionName string      `json:"collection_name"`
	IDs            []string    `json:"ids"`
	Embeddings     [][]float64 `json:"embeddings"`
	Documents      []string    `json:"documents,omitempty"`
}

func (s *ChromaStore) Insert(collectionName string, ids []string, embeddings [][]float64, documents []string) error {
	body := InsertRequest{
		CollectionName: collectionName,
		IDs:            ids,
		Embeddings:     embeddings,
		Documents:      documents,
	}
	_, err := s.doRequest("POST", "/insert", body)
	return err
}

type SearchResult struct {
	IDs       [][]string    `json:"ids"`
	Distances [][]float64   `json:"distances"`
	Documents [][]string    `json:"documents"`
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
