package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/LennyFace24/MiniAgent/internal/store"
)

func TestListDocumentsDeduplicatesAndSortsSources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","documents":[
			{"id":"1","document":"a","metadata":{"source":"运维.md"}},
			{"id":"2","document":"b","metadata":{"source":"api.md"}},
			{"id":"3","document":"c","metadata":{"source":"运维.md"}},
			{"id":"4","document":"d","metadata":null}
		]}`))
	}))
	defer server.Close()

	svc := &FileService{vectorStore: store.NewChromaStore(server.URL)}
	docs, err := svc.ListDocuments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"api.md", "运维.md"}; !reflect.DeepEqual(docs, want) {
		t.Fatalf("documents = %v, want %v", docs, want)
	}
}

func TestDeleteDocumentRejectsEmptySource(t *testing.T) {
	svc := &FileService{vectorStore: store.NewChromaStore("http://127.0.0.1:1")}
	if err := svc.DeleteDocument(context.Background(), "   "); err == nil {
		t.Fatal("empty source should be rejected before calling Chroma")
	}
}
