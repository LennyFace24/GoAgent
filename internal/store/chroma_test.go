package store

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListDocumentsParsesChromaResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/documents" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("collection_name"); got != "documents" {
			t.Errorf("collection_name = %q", got)
		}
		_, _ = w.Write([]byte(`{"status":"ok","documents":[{"id":"1","document":"a","metadata":{"source":"a.md"}},{"id":"2","document":"b","metadata":{"source":"a.md"}}]}`))
	}))
	defer server.Close()

	docs, err := NewChromaStore(server.URL).ListDocuments("documents")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || docs[0].Metadata["source"] != "a.md" || docs[1].Document != "b" {
		t.Fatalf("unexpected documents: %+v", docs)
	}
}

func TestDeleteBySourceSendsEncodedQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/documents" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("collection_name"); got != "documents" {
			t.Errorf("collection_name = %q", got)
		}
		if got := r.URL.Query().Get("source"); got != "运维 手册.md" {
			t.Errorf("source = %q", got)
		}
		_, _ = w.Write([]byte(`{"status":"deleted"}`))
	}))
	defer server.Close()

	if err := NewChromaStore(server.URL).DeleteBySource("documents", "运维 手册.md"); err != nil {
		t.Fatal(err)
	}
}

func TestInsertSendsMetadatas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body InsertRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Metadatas) != 1 || body.Metadatas[0]["source"] != "手册.md" {
			t.Errorf("metadatas not forwarded: %+v", body.Metadatas)
		}
		_, _ = w.Write([]byte(`{"status":"inserted"}`))
	}))
	defer server.Close()

	err := NewChromaStore(server.URL).Insert(
		"documents",
		[]string{"1"},
		[][]float64{{0.1, 0.2}},
		[]string{"content"},
		[]map[string]any{{"source": "手册.md"}},
	)
	if err != nil {
		t.Fatal(err)
	}
}
