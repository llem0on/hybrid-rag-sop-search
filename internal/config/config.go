package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/llem0on/hybrid-rag-sop-search/internal/embedding"
	"github.com/llem0on/hybrid-rag-sop-search/internal/search"
)

type Config struct {
	Addr             string
	DataDir          string
	LLMBaseURL       string
	LLMAPIKey        string
	EmbedModel       string
	RerankURL        string
	RerankModel      string
	DocInstruction   string
	QueryInstruction string
	EmbedTimeout     time.Duration
	RerankTimeout    time.Duration
	OCREnabled       bool
	OCRLanguage      string
	Search           search.Config
}

func Load() Config {
	base := strings.TrimRight(env("LLM_BASE_URL", ""), "/")
	rerankURL := env("RERANK_URL", "")
	if rerankURL == "" && base != "" {
		rerankURL = strings.TrimSuffix(base, "/v1") + "/rerank"
	}

	s := search.DefaultConfig()
	s.MinSemantic = envFloat("MIN_SEMANTIC", s.MinSemantic)
	s.BM25Min = envFloat("BM25_MIN", s.BM25Min)
	s.BM25MinCoverage = envFloat("BM25_MIN_COVERAGE", s.BM25MinCoverage)
	s.WSemantic = envFloat("W_SEMANTIC", s.WSemantic)
	s.WKeyword = envFloat("W_KEYWORD", s.WKeyword)
	s.RRFK = envFloat("RRF_K", s.RRFK)
	s.RerankMin = envFloat("RERANK_MIN", s.RerankMin)
	s.RerankTopMin = envFloat("RERANK_TOP_MIN", s.RerankTopMin)
	s.RerankTopCos = envFloat("RERANK_TOP_COS", s.RerankTopCos)
	s.MaxResults = envInt("MAX_RESULTS", s.MaxResults)

	return Config{
		Addr:             env("ADDR", ":8080"),
		DataDir:          env("DATA_DIR", "./data"),
		LLMBaseURL:       base,
		LLMAPIKey:        env("LLM_API_KEY", ""),
		EmbedModel:       env("EMBED_MODEL", "qwen3-vl-embedding-8b"),
		RerankURL:        rerankURL,
		RerankModel:      env("RERANK_MODEL", ""),
		DocInstruction:   env("DOC_INSTRUCTION", embedding.DefaultDocInstruction),
		QueryInstruction: env("QUERY_INSTRUCTION", embedding.DefaultQueryInstruction),
		EmbedTimeout:     envDuration("EMBED_TIMEOUT", 60*time.Second),
		RerankTimeout:    envDuration("RERANK_TIMEOUT", 25*time.Second),
		OCREnabled:       envBool("OCR_ENABLED", true),
		OCRLanguage:      env("OCR_LANGUAGE", "eng"),
		Search:           s,
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(env(key, ""), 64); err == nil {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, err := strconv.ParseBool(env(key, "")); err == nil {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(env(key, "")); err == nil {
		return v
	}
	return def
}
