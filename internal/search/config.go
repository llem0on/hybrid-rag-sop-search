package search

type Config struct {
	MinSemantic        float64
	BM25Min            float64
	BM25MinCoverage    float64
	WSemantic          float64
	WKeyword           float64
	RRFK               float64
	LaneTop            int
	Candidates         int
	RerankMin          float64
	RerankTopMin       float64
	RerankTopCos       float64
	MaxResults         int
	RerankContentChars int
}

func DefaultConfig() Config {
	return Config{
		MinSemantic:        0.60,
		BM25Min:            12,
		BM25MinCoverage:    0.5,
		WSemantic:          0.8,
		WKeyword:           0.2,
		RRFK:               60,
		LaneTop:            30,
		Candidates:         10,
		RerankMin:          0.5,
		RerankTopMin:       0.15,
		RerankTopCos:       0.65,
		MaxResults:         4,
		RerankContentChars: 800,
	}
}
