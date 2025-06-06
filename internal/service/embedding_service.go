package service

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// OpenAIEmbeddingService implements the EmbeddingService interface using OpenAI
type OpenAIEmbeddingService struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIEmbeddingService creates a new OpenAI embedding service
func NewOpenAIEmbeddingService(apiKey string) *OpenAIEmbeddingService {
	return &OpenAIEmbeddingService{
		apiKey: apiKey,
		model:  "text-embedding-3-small",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// OpenAI API request/response structures
type openAIEmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *openAIError `json:"error,omitempty"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// GenerateEmbedding generates an embedding for the given text using OpenAI's API
func (s *OpenAIEmbeddingService) GenerateEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Prepare request
	reqBody := openAIEmbeddingRequest{
		Input: text,
		Model: s.model,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	// Make request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var embeddingResp openAIEmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if embeddingResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s (%s)", embeddingResp.Error.Message, embeddingResp.Error.Type)
	}

	// Check response data
	if len(embeddingResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	return embeddingResp.Data[0].Embedding, nil
}

// MockEmbeddingService provides a mock implementation for testing
type MockEmbeddingService struct{}

// NewMockEmbeddingService creates a new mock embedding service
func NewMockEmbeddingService() *MockEmbeddingService {
	return &MockEmbeddingService{}
}

// GenerateEmbedding generates a deterministic fake embedding for testing
func (m *MockEmbeddingService) GenerateEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text provided")
	}

	// Generate a deterministic embedding based on text hash
	// This ensures same input produces same output
	hash := md5.Sum([]byte(text))

	// Create a 1536-dimensional embedding (standard OpenAI size)
	embedding := make([]float64, 1536)

	// Use hash bytes to seed the embedding values
	for i := range embedding {
		// Use hash bytes cyclically and add some variation
		byteIndex := i % len(hash)
		hashValue := float64(hash[byteIndex])

		// Normalize to reasonable embedding values (-1 to 1)
		embedding[i] = (hashValue - 127.5) / 127.5

		// Add some position-based variation
		embedding[i] += float64(i%100) / 10000.0
	}

	// Normalize the vector to unit length (common for embeddings)
	var norm float64
	for _, val := range embedding {
		norm += val * val
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

// KeywordEmbeddingService provides keyword-based similarity without external APIs
type KeywordEmbeddingService struct{}

// NewKeywordEmbeddingService creates a new keyword-based embedding service
func NewKeywordEmbeddingService() *KeywordEmbeddingService {
	return &KeywordEmbeddingService{}
}

// Common network keywords for better semantic matching
// To extend: add synonyms, acronyms, and domain-specific terms relevant to your environment.
var networkKeywords = map[string]float64{
	// Device types
	"device": 1.0, "devices": 1.0, "router": 0.9, "routers": 0.9, "switch": 0.9, "switches": 0.9,
	"firewall": 0.9, "firewalls": 0.9, "host": 0.8, "hosts": 0.8, "node": 0.8, "nodes": 0.8,

	// Network concepts
	"network": 1.0, "networks": 1.0, "interface": 0.9, "interfaces": 0.9, "port": 0.8, "ports": 0.8,
	"link": 0.7, "links": 0.7, "connection": 0.7, "connections": 0.7,

	// Protocols and routing
	"bgp": 0.95, "ospf": 0.9, "eigrp": 0.9, "isis": 0.9, "rip": 0.8, "static": 0.8,
	"tcp": 0.8, "udp": 0.8, "icmp": 0.8, "ip": 0.9, "ipv4": 0.9, "ipv6": 0.9,
	"route": 0.9, "routes": 0.9, "routing": 0.9, "prefix": 0.8, "prefixes": 0.8,
	"neighbor": 0.9, "neighbors": 0.9, "peer": 0.9, "peers": 0.9, "adjacency": 0.8, "adjacencies": 0.8,
	"asn": 0.8, "autonomous": 0.8, "system": 0.8, "as": 0.8,
	"convergence": 0.8, "converge": 0.8, "flap": 0.7, "flapping": 0.7,

	// VLANs and switching
	"vlan": 0.8, "vlans": 0.8, "trunk": 0.7, "access": 0.7, "tagged": 0.7, "untagged": 0.7,

	// Security and compliance
	"acl": 0.8, "acls": 0.8, "security": 0.9, "cve": 0.9, "cves": 0.9, "compliance": 0.8, "audit": 0.8, "check": 0.7,
	"policy": 0.8, "policies": 0.8, "rule": 0.8, "rules": 0.8, "permit": 0.7, "deny": 0.7,

	// Hardware
	"hardware": 1.0, "cpu": 0.8, "memory": 0.8, "ram": 0.8, "storage": 0.8,
	"model": 0.9, "serial": 0.8, "version": 0.8, "firmware": 0.8,

	// Operations
	"show": 0.8, "list": 0.9, "get": 0.8, "find": 0.8, "search": 0.9, "display": 0.8,
	"status": 0.9, "state": 0.9, "config": 0.9, "configuration": 0.9, "settings": 0.8,
	"summary": 0.7, "aggregate": 0.7, "detail": 0.7, "details": 0.7,

	// Common actions
	"select": 0.7, "where": 0.6, "foreach": 0.7, "from": 0.6, "in": 0.5,

	// Attributes
	"name": 0.8, "names": 0.8, "address": 0.8, "addresses": 0.8, "location": 0.8,
	"platform": 0.9, "vendor": 0.8, "type": 0.8, "admin": 0.7, "operational": 0.8,
	"down": 0.7, "up": 0.7, "active": 0.7, "inactive": 0.7, "reachable": 0.8, "unreachable": 0.8,

	// Miscellaneous
	"log": 0.7, "logs": 0.7, "event": 0.7, "events": 0.7, "history": 0.7, "change": 0.7, "changes": 0.7,
}

// GenerateEmbedding creates embeddings based on keyword analysis
func (k *KeywordEmbeddingService) GenerateEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text provided")
	}

	// Create a 384-dimensional embedding (smaller but still effective)
	embedding := make([]float64, 384)

	// Convert to lowercase for matching
	lowerText := strings.ToLower(text)
	words := strings.Fields(lowerText)

	// Initialize with base hash for uniqueness
	hash := md5.Sum([]byte(text))
	for i := range embedding {
		byteIndex := i % len(hash)
		embedding[i] = float64(hash[byteIndex]) / 1000.0
	}

	// Add keyword-based features
	keywordCount := 0
	for _, word := range words {
		// Remove punctuation
		cleanWord := strings.Trim(word, ".,;:!?()[]{}")

		if weight, exists := networkKeywords[cleanWord]; exists {
			keywordCount++
			// Distribute keyword influence across embedding dimensions
			for i := 0; i < len(embedding); i += 10 {
				if i < len(embedding) {
					embedding[i] += weight
				}
			}

			// Add word-specific patterns
			wordHash := md5.Sum([]byte(cleanWord))
			for i := 0; i < len(embedding) && i < len(wordHash)*10; i++ {
				embedding[i] += float64(wordHash[i/10]) * weight / 500.0
			}
		}
	}

	// Add length-based features
	textLength := float64(len(text))
	wordCount := float64(len(words))

	// Encode text statistics in specific dimensions
	if len(embedding) > 10 {
		embedding[0] += textLength / 1000.0
		embedding[1] += wordCount / 100.0
		embedding[2] += float64(keywordCount) / 10.0
	}

	// Add bigram features for better context
	for i := 0; i < len(words)-1; i++ {
		bigram := words[i] + " " + words[i+1]
		bigramHash := md5.Sum([]byte(bigram))

		// Distribute bigram influence
		for j := 0; j < len(embedding) && j < len(bigramHash)*5; j++ {
			embedding[j] += float64(bigramHash[j/5]) / 2000.0
		}
	}

	// Normalize the vector
	var norm float64
	for _, val := range embedding {
		norm += val * val
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}
