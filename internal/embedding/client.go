package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

const embedBatchSize = 16

type Client struct {
	BaseURL          string
	APIKey           string
	Model            string
	DocInstruction   string
	QueryInstruction string
	RerankURL        string
	RerankModel      string
	EmbedHTTP        *http.Client
	RerankHTTP       *http.Client
}

func NewClient(baseURL, apiKey, model, docInstruction, queryInstruction, rerankURL, rerankModel string, embedTimeout, rerankTimeout time.Duration) *Client {
	return &Client{
		BaseURL:          baseURL,
		APIKey:           apiKey,
		Model:            model,
		DocInstruction:   docInstruction,
		QueryInstruction: queryInstruction,
		RerankURL:        rerankURL,
		RerankModel:      rerankModel,
		EmbedHTTP:        &http.Client{Timeout: embedTimeout},
		RerankHTTP:       &http.Client{Timeout: rerankTimeout},
	}
}

func (c *Client) EmbeddingEnabled() bool {
	return c != nil && c.BaseURL != "" && c.Model != ""
}

func (c *Client) RerankEnabled() bool {
	return c != nil && c.RerankURL != "" && c.RerankModel != ""
}

func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.embed(ctx, []string{ChatTemplate(c.QueryInstruction, text)})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += embedBatchSize {
		end := min(start+embedBatchSize, len(texts))
		batch := make([]string, 0, end-start)
		for _, t := range texts[start:end] {
			batch = append(batch, ChatTemplate(c.DocInstruction, t))
		}
		vecs, err := c.embed(ctx, batch)
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func (c *Client) embed(ctx context.Context, inputs []string) ([][]float32, error) {
	var resp struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	body := map[string]any{"model": c.Model, "input": inputs}
	if err := c.post(ctx, c.EmbedHTTP, c.BaseURL+"/embeddings", body, &resp); err != nil {
		return nil, fmt.Errorf("embeddings: %w", err)
	}
	if len(resp.Data) != len(inputs) {
		return nil, fmt.Errorf("embeddings: got %d vectors for %d inputs", len(resp.Data), len(inputs))
	}
	sort.Slice(resp.Data, func(i, j int) bool { return resp.Data[i].Index < resp.Data[j].Index })
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

func (c *Client) Rerank(ctx context.Context, query string, docs []string) ([]float64, error) {
	var resp struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}
	body := map[string]any{"model": c.RerankModel, "query": query, "documents": docs}
	if err := c.post(ctx, c.RerankHTTP, c.RerankURL, body, &resp); err != nil {
		return nil, fmt.Errorf("rerank: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("rerank: empty response")
	}
	scores := make([]float64, len(docs))
	for i := range scores {
		scores[i] = -1
	}
	for _, r := range resp.Results {
		if r.Index >= 0 && r.Index < len(docs) {
			scores[r.Index] = r.RelevanceScore
		}
	}
	return scores, nil
}

func (c *Client) post(ctx context.Context, hc *http.Client, url string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %.200s", resp.StatusCode, raw)
	}
	return json.Unmarshal(raw, out)
}
