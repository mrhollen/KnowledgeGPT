CREATE TABLE sessions (
	id text NOT NULL,
	messages _text NOT NULL,
	model text NOT NULL,
	CONSTRAINT sessions_pkey PRIMARY KEY (id)
);

CREATE TABLE datasets (
	id serial4 NOT NULL,
	"name" text NOT NULL,
	CONSTRAINT datasets_pkey PRIMARY KEY (id),
	CONSTRAINT datasets_unique UNIQUE (name)
);

CREATE TABLE documents (
	id serial4 NOT NULL,
	dataset_id int4 NOT NULL,
	title text NOT NULL,
	url text NULL,
	body text NOT NULL,
	vector public.vector NOT NULL, -- public.vector requires the pgvector extension
	CONSTRAINT documents_pkey PRIMARY KEY (id)
);