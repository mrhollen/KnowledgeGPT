CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE sessions (
	id text NOT NULL,
	user_id int4 NOT NULL,
	messages _text NOT NULL,
	model text NOT NULL,
	CONSTRAINT sessions_pkey PRIMARY KEY (id)
);

CREATE TABLE datasets (
	id serial4 NOT NULL,
	user_id int4 NOT NULL,
	"name" text NOT NULL,
	CONSTRAINT datasets_pkey PRIMARY KEY (id),
	CONSTRAINT datasets_unique UNIQUE (name, user_id)
);

CREATE TABLE documents (
	id serial4 NOT NULL,
	dataset_id int4 NOT NULL,
	title text NOT NULL,
	url text NULL,
	body text NOT NULL,
	vector public.vector NOT NULL,
	CONSTRAINT documents_pkey PRIMARY KEY (id),
	CONSTRAINT documents_datasets_fk 
		FOREIGN KEY (dataset_id) 
		REFERENCES datasets(id) 
		ON DELETE CASCADE ON UPDATE CASCADE
);

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