CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE sessions (
	id text NOT NULL,
	user_id int4 NOT NULL,
	messages _text NOT NULL,
	model text NOT NULL,
	CONSTRAINT sessions_pkey PRIMARY KEY (id)
);

-- Keep the datasets table as is
CREATE TABLE datasets (
    id serial4 NOT NULL,
    user_id int4 NOT NULL,
    "name" text NOT NULL,
    CONSTRAINT datasets_pkey PRIMARY KEY (id),
    CONSTRAINT datasets_unique UNIQUE (name, user_id)
);

-- Modified documents table - holds metadata, not the full body or vector
CREATE TABLE documents (
    id serial4 NOT NULL,
    dataset_id int4 NOT NULL,
    title text NOT NULL,
    url text NULL,
    -- Removed body and vector columns
    created_at timestamptz DEFAULT now() NOT NULL, -- Optional: track creation time
    updated_at timestamptz DEFAULT now() NOT NULL, -- Optional: track update time
    CONSTRAINT documents_pkey PRIMARY KEY (id),
    CONSTRAINT documents_datasets_fk 
        FOREIGN KEY (dataset_id) 
        REFERENCES datasets(id) 
        ON DELETE CASCADE ON UPDATE CASCADE -- Keep cascade for referential integrity
);

-- New table to store ordered document chunks
CREATE TABLE document_chunks (
    id bigserial NOT NULL, -- Use bigserial if you expect many chunks
    document_id int4 NOT NULL,
    sequence_number int4 NOT NULL, -- To order chunks for reconstruction
    chunk_text text NOT NULL,      -- The actual text content of the chunk
    -- Optional: Add metadata about the chunking process if needed
    -- chunk_metadata jsonb NULL, 
    CONSTRAINT document_chunks_pkey PRIMARY KEY (id),
    CONSTRAINT document_chunks_documents_fk 
        FOREIGN KEY (document_id) 
        REFERENCES documents(id) 
        ON DELETE CASCADE ON UPDATE CASCADE, -- Deleting document deletes its chunks
    CONSTRAINT document_chunks_unique_sequence 
        UNIQUE (document_id, sequence_number) -- Ensure unique sequence per document
);

-- Index for efficient retrieval of chunks in order
CREATE INDEX idx_document_chunks_ordering 
ON document_chunks (document_id, sequence_number);

-- New table for core concepts and their vectors
CREATE TABLE document_concepts (
    id bigserial NOT NULL,          -- Use bigserial if you expect many concepts
    document_chunk_id int8 NOT NULL, -- Link concept to the specific chunk it came from
    concept_text text NOT NULL,     -- The extracted core concept text
    vector public.vector(768) NOT NULL, -- The embedding vector for THIS concept
                                    -- *** ADJUST 768 to your vector dimension ***
    -- Optional: Add metadata about the concept extraction
    -- concept_metadata jsonb NULL, 
    CONSTRAINT document_concepts_pkey PRIMARY KEY (id),
    CONSTRAINT document_concepts_chunks_fk 
        FOREIGN KEY (document_chunk_id) 
        REFERENCES document_chunks(id) 
        ON DELETE CASCADE ON UPDATE CASCADE -- Deleting chunk deletes its concepts
);

-- Index for efficient vector similarity search (using HNSW is recommended)
-- The exact syntax might vary slightly based on pgvector version
-- Replace 'lists = 100, ef_construction = 64' with appropriate parameters
CREATE INDEX idx_document_concepts_vector 
ON document_concepts 
USING hnsw (vector vector_l2_ops); -- Or use vector_ip_ops / vector_cosine_ops
                                 -- based on your distance metric preference

-- Optional: Index for looking up concepts by text (if needed)
CREATE INDEX idx_document_concepts_text ON document_concepts (concept_text); 
-- Consider using pg_trgm for fuzzy text searching on concepts if required
-- CREATE INDEX idx_document_concepts_text_trgm ON document_concepts USING gin (concept_text gin_trgm_ops);

CREATE TABLE users (
	id serial4 NOT NULL,
	username text NOT NULL,
	active bool DEFAULT false NOT NULL,
	CONSTRAINT users_pkey PRIMARY KEY (id),
	CONSTRAINT users_username_key UNIQUE (username)
);

CREATE TABLE access_tokens (
	id serial4 NOT NULL,
	user_id int4 NOT NULL,
	"token" varchar NOT NULL,
	expiration timestamp DEFAULT (now() + '1 year'::interval) NOT NULL,
	CONSTRAINT access_tokens_pkey PRIMARY KEY (id),
	CONSTRAINT access_tokens_token_key UNIQUE (token),
	CONSTRAINT access_tokens_users_fk 
		FOREIGN KEY (user_id) 
		REFERENCES users(id)
);