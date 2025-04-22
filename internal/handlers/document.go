package handlers

import (
	"context" // Added
	"encoding/json"
	"fmt"
	"log" // Added for logging errors
	"net/http"
	"strings" // Added for placeholder chunking
	"time"

	api "github.com/mrhollen/KnowledgeGPT/internal/api/documents"
	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
	"github.com/mrhollen/KnowledgeGPT/internal/models"
)

type DocumentHandler struct {
	Client llm.Client
	DB     *db.PostgresDB
}

// --- Placeholder Functions ---
// Replace these with your actual implementation

// chunkText splits the document body into manageable chunks.
// Implement your desired chunking strategy (e.g., by paragraph, sentence, fixed size).
func chunkText(text string, chunkSize int) []string {
	// Simple placeholder: split by double newline, then approximate size
	// Replace with a more robust method.
	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	currentChunk := ""
	for _, p := range paragraphs {
		trimmedP := strings.TrimSpace(p)
		if trimmedP == "" {
			continue
		}
		if len(currentChunk)+len(trimmedP)+1 > chunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = trimmedP
		} else {
			if len(currentChunk) > 0 {
				currentChunk += "\n\n" + trimmedP
			} else {
				currentChunk = trimmedP
			}
		}
	}
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}
	// If no double newlines, split by single? Or by sentence? Needs refinement.
	if len(chunks) == 0 && len(text) > 0 {
		// Fallback: very basic split - consider sentence splitting or token counting
		if len(text) > chunkSize {
			// Very crude split - replace!
			chunks = append(chunks, text[:chunkSize])
			// Add more chunks if needed... this is just illustrative
		} else {
			chunks = append(chunks, text)
		}
	}
	log.Printf("Chunked text into %d chunks", len(chunks))
	return chunks
}

// extractConcepts identifies core concepts within a text chunk.
// This might involve NLP techniques or calling an LLM.
func (h *DocumentHandler) extractConcepts(ctx context.Context, chunkText string, model string) ([]string, error) {
	// Placeholder: Use LLM to extract keywords/concepts
	// Adjust the prompt and logic as needed.
	// This could be a separate LLM call per chunk, potentially slow/expensive.
	// Consider batching or alternative NLP methods.
	prompt := fmt.Sprintf("Identify the main core concepts from the following text. The reader will have no context into the larger document so make sure the concept is complate. List each concept in a complete sencent on a new line, without any preamble or explanation:\n\n```\n%s\n```", chunkText)

	// Use a context with a shorter timeout for this potentially faster call
	conceptCtx, cancel := context.WithTimeout(ctx, 1000*time.Second) // Adjust timeout
	defer cancel()

	response, err := h.Client.SendPrompt(conceptCtx, prompt, "", model)
	if err != nil {
		return nil, fmt.Errorf("failed to extract concepts using LLM: %w", err)
	}

	concepts := strings.Split(strings.TrimSpace(response), "\n")
	// Basic cleanup
	var cleanedConcepts []string
	for _, c := range concepts {
		trimmed := strings.TrimSpace(c)
		if trimmed != "" {
			// Further cleanup (e.g., remove leading hyphens/bullets) can be added
			trimmed = strings.TrimPrefix(trimmed, "- ")
			trimmed = strings.TrimPrefix(trimmed, "* ")
			cleanedConcepts = append(cleanedConcepts, trimmed)
		}
	}
	if len(cleanedConcepts) == 0 {
		log.Printf("Warning: No concepts extracted for chunk starting with: %s...", chunkText[:min(50, len(chunkText))])
		// Decide if returning an error or an empty slice is better
		// return nil, fmt.Errorf("no concepts extracted")
	}
	log.Printf("Extracted %d concepts for chunk", len(cleanedConcepts))
	return cleanedConcepts, nil
}

// --- HTTP Handlers ---

func (h *DocumentHandler) AddDocument(userId int64, w http.ResponseWriter, r *http.Request) {
	var req api.AddDocumentRequest
	// NOTE: api.AddDocumentRequest likely still contains 'Body'. This is okay for receiving the initial data.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close() // Ensure body is closed

	if err := h.createDocumentAndContent(r.Context(), userId, req); err != nil {
		// Error handling improved in createDocumentAndContent to send HTTP errors
		log.Printf("Error processing document '%s': %v", req.Title, err)
		// Avoid writing header twice if createDocumentAndContent already did
		// http.Error(w, fmt.Sprintf("Failed to process document: %v", err), http.StatusInternalServerError)
		return // Error response already sent by createDocumentAndContent
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, `{"message": "Document processing started successfully"}`) // Provide feedback
}

func (h *DocumentHandler) AddDocuments(userId int64, w http.ResponseWriter, r *http.Request) {
	var reqs []api.AddDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close() // Ensure body is closed

	// Consider running these concurrently with worker pools for large batches
	// For now, sequential processing:
	errorOccurred := false
	results := make(map[string]string) // Track status per document title

	for _, req := range reqs {
		err := h.createDocumentAndContent(r.Context(), userId, req)
		if err != nil {
			log.Printf("Error processing document '%s' in batch: %v", req.Title, err)
			errorOccurred = true
			results[req.Title] = fmt.Sprintf("Failed: %v", err)
			// Decide if you want to stop the whole batch on first error, or continue
			// continue
		} else {
			results[req.Title] = "Success"
		}
	}

	// Respond based on outcome
	w.Header().Set("Content-Type", "application/json")
	if errorOccurred {
		// Respond with partial success/failure details if needed
		w.WriteHeader(http.StatusMultiStatus) // Indicate partial success/failure
		json.NewEncoder(w).Encode(results)
	} else {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, `{"message": "All documents processed successfully"}`)
	}
}

// createDocumentAndContent handles the logic for chunking, concept extraction, embedding, and DB insertion.
// It now returns an error to allow AddDocument/AddDocuments to handle responses appropriately.
func (h *DocumentHandler) createDocumentAndContent(ctx context.Context, userId int64, req api.AddDocumentRequest) error {
	if req.Body == "" {
		// Handle empty body based on requirements (error or ignore?)
		log.Printf("Skipping document '%s': empty body", req.Title)
		return fmt.Errorf("document body is empty") // Return error to signal failure
	}

	// 1. Get Dataset ID
	datasetName := req.Dataset
	if datasetName == "" {
		datasetName = "default"
	}
	// Use context for DB call
	datasetId, err := h.DB.GetOrCreateDataset(ctx, datasetName, userId)
	if err != nil {
		log.Printf("Error getting/creating dataset '%s': %v", datasetName, err)
		// http.Error(w, "Error accessing dataset", http.StatusInternalServerError) // Send error from here
		return fmt.Errorf("error accessing dataset: %w", err) // Return error for caller
	}

	// 2. Prepare Document Metadata (without body/vector)
	docMetadata := models.Document{
		Title:     req.Title,
		URL:       &req.URL, // Assuming req.URL is *string or handle nil appropriately
		DatasetID: datasetId,
	}
	// If req.URL is string, convert to *string if needed for the model
	// if req.URL != "" {
	// 	docMetadata.URL = &req.URL
	// }

	// 3. Chunk the document body
	// TODO: Make chunkSize configurable?
	chunks := chunkText(req.Body, 1000) // Example chunk size
	if len(chunks) == 0 && len(req.Body) > 0 {
		log.Printf("Warning: Failed to chunk non-empty document body for '%s'. Using entire body as one chunk.", req.Title)
		chunks = []string{req.Body} // Fallback? Or return error?
	}
	if len(chunks) == 0 {
		log.Printf("Skipping document '%s': body resulted in zero chunks", req.Title)
		return fmt.Errorf("document body resulted in zero chunks") // Return error
	}

	// 4. Process Chunks: Extract Concepts and Embed Them
	contentToInsert := make([]db.ChunkWithConcepts, 0, len(chunks))

	for i, chunkStr := range chunks {
		log.Printf("Processing chunk %d/%d for document '%s'", i+1, len(chunks), req.Title)
		chunkModel := models.DocumentChunk{
			SequenceNumber: i + 1, // Sequence starts at 1
			ChunkText:      chunkStr,
		}

		// Extract Concepts (using placeholder function)
		// Pass context down
		conceptsText, err := h.extractConcepts(ctx, chunkStr, "") // Use appropriate model if needed
		if err != nil {
			log.Printf("Error extracting concepts for chunk %d of '%s': %v. Skipping concepts for this chunk.", i+1, req.Title, err)
			// Decide: Skip concepts for this chunk, or fail the whole document?
			// Let's skip concepts for this chunk for now.
			conceptsText = []string{} // Ensure it's an empty slice
			// return fmt.Errorf("failed to extract concepts for chunk %d: %w", i+1, err) // Alternative: fail fast
		}

		conceptsModels := make([]models.DocumentConcept, 0, len(conceptsText))
		if len(conceptsText) > 0 {
			log.Printf("Embedding %d concepts for chunk %d...", len(conceptsText), i+1)
		}

		for j, conceptStr := range conceptsText {
			if strings.TrimSpace(conceptStr) == "" {
				continue // Skip empty concepts
			}
			// Get Embedding for the concept
			// Use context with appropriate timeout
			embedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)       // Adjust timeout
			conceptVec, err := h.Client.GetEmbedding(embedCtx, conceptStr, "") // Pass context, Use appropriate model

			cancel() // Release context resources promptly

			if err != nil {
				log.Printf("Error getting embedding for concept '%s' (chunk %d, concept %d): %v. Skipping concept.", conceptStr, i+1, j+1, err)
				// Decide: Skip concept, or fail whole document?
				// Skipping concept for now.
				continue
				// return fmt.Errorf("failed to get embedding for concept '%s': %w", conceptStr, err) // Alternative: fail fast
			}

			conceptModel := models.DocumentConcept{
				ConceptText: conceptStr,
				Vector:      conceptVec,
				// DocumentChunkID is set by the DB layer during insertion
			}
			conceptsModels = append(conceptsModels, conceptModel)
		}

		// Only add chunk if it has text (redundant check maybe, but safe)
		if chunkModel.ChunkText != "" {
			contentToInsert = append(contentToInsert, db.ChunkWithConcepts{
				Chunk:    chunkModel,
				Concepts: conceptsModels, // May be empty if concept extraction/embedding failed
			})
		}
	} // End chunk processing loop

	// 5. Add to Database
	if len(contentToInsert) == 0 {
		log.Printf("Warning: No valid chunks with content generated for document '%s'. Nothing to insert.", req.Title)
		// This might happen if all chunks failed concept extraction/embedding and we chose to skip them.
		// Decide if this is an error condition.
		return fmt.Errorf("no processable content generated for the document")
	}

	log.Printf("Inserting document '%s' with %d processed chunks into dataset '%s'...", req.Title, len(contentToInsert), datasetName)
	// Use context for the DB call
	_, err = h.DB.AddDocumentAndContent(ctx, docMetadata, contentToInsert)
	if err != nil {
		log.Printf("Error inserting document '%s' content: %v", req.Title, err)
		// http.Error(w, "Failed to save document content", http.StatusInternalServerError) // Send error from here
		return fmt.Errorf("failed to save document content: %w", err) // Return error for caller
	}

	log.Printf("Successfully processed and inserted document '%s'", req.Title)
	return nil // Success
}

// Helper for min calculation
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
