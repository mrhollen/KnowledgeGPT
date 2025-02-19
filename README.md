# KnowledgeGPT

**THIS IS A WORK IN PROGRESS AND IS NOT READY FOR USE**

**KnowledgeGPT** is a lightweight, dependency-minimal Go application designed to ingest documents or text snippets and provide intelligent responses to queries by retrieving relevant information and interfacing with an OpenAI-compatible Language Model (LLM) server. Leveraging Go's powerful standard library, KnowledgeGPT ensures high performance, maintainability, and ease of deployment.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Running the Server](#running-the-server)
  - [API Endpoints](#api-endpoints)
    - [Add Document](#add-document)
    - [Query](#query)
- [Project Structure](#project-structure)
- [Contributing](#contributing)
- [License](#license)

## Features

- **HTTP Server**: Handles incoming requests to add documents, perform queries, and manage chat sessions.
- **Document Management**: Accepts documents with a title, optional URL, and body text, storing them in a Postgres Vector Database.
- **Flexible Database Layer**: Interfaces with the database layer, allowing easy swapping of implementations.
- **LLM Integration**: Communicates with OpenAI-compatible servers to generate responses based on user queries and retrieved documents.
- **Chat Session Management**: Maintains persistent chat sessions across multiple requests and sessions using unique identifiers.
- **Minimal Dependencies**: Built primarily with Go's standard library to ensure lightweight and easy maintenance.

## Architecture

KnowledgeGPT is structured into several key components, each encapsulated within its own package:

- **cmd/server**: Entry point of the application.
- **internal/db**: Database interfaces and Postgres implementation.
- **internal/llm**: LLM client interfaces and OpenAI-compatible implementation.
- **internal/handlers**: HTTP handlers for managing documents, queries, and chat sessions.
- **internal/models**: Data models used across the application.
- **internal/session**: Manages chat session persistence.
- **pkg/utils**: Utility functions, including UUID generation.

## Prerequisites

- **Go**: Version 1.23 or higher is recommended. [Download Go](https://golang.org/dl/)
- **pgvector**: PostgreSQL with the pgvector extension installed is required. [Github](https://github.com/pgvector/pgvector)

## Installation

1. **Clone the Repository**

   ```bash
   git clone https://github.com/mrhollen/KnowledgeGPT.git
   cd KnowledgeGPT
   ```

2. **Initialize Go Modules**

   Ensure you are within the project directory and initialize the Go module:

   ```bash
   go mod tidy
   ```

   This will download the necessary dependencies..

3. **Create the Postgres Database**
   
   Included in the root of this project is a file called `create_postgres_database.sql` that contains all the SQL statements needed to setup your database. Feel free to modify it to suit your specific needs.

4. **Make sure Authentication is Setup**

   Please see the [Authentication](#authentication) section to learn more about how to insert access tokens into the database. Without this you will get a _401 Unauthorized_ for every request.

## Configuration

KnowledgeGPT requires configuration for the Postgres database and the LLM server.

### Environment Variables

It's recommended to use environment variables for sensitive information and configuration settings.

- **LLM_ENDPOINT**: The URL of the OpenAI-compatible LLM server.
- **LLM_EMBEDDING_ENDPOINT**: The URL of the OpenAI-compatible LLM server used specifically for creating embedding vectors.
- **LLM_API_KEY**: API key for authenticating with the LLM server.
- **LLM_DEFAULT_MODEL**: The name of the default model to use when not specified in the user's request.
- **DB_CONNECTION_STRING**: Your postgres connection string.
- **IP_ADDRESS**: The IP Address the server should bind to.
- **PORT**: The port the server should listen on.

You can set these variables in a `.env` file which will be used by [`dotenv`](https://github.com/joho/godotenv).

### Example `.env` File

```env
LLM_ENDPOINT=https://api.openai.com/v1/engines/davinci/completions
LLM_EMBEDDING_ENDPOINT=https://api.openai.com/v1/embeddings
LLM_API_KEY=your_openai_api_key
LLM_DEFAULT_MODEL=LLAMA-3.1-8B

DB_CONNECTION_STRING=postgresql://postgres:password@localhost

IP_ADDRESS=127.0.0.1
PORT=8080
```

*Ensure that `.env` files are excluded from version control to protect sensitive information.*

### System Prompt

To update the system prompt you can update `system_prompt.txt` to suit your needs. At the moment, this file is loaded on startup and used for *all* requests.

## Usage

### Running the Server

```bash
go build -o knowledgegpt ./cmd/server
./knowledgegpt
```

The server will start and listen on the IP Address and Port configured in the `.env` file.

### Authentication

#### Access Tokens

Currently the only authentication supported is via simple access tokens. There is no set format for these tokens so they can be whatever moves your spirit. To create an access token, you'll need to first insert a new user into the users database table. Then create a new access token in the access_tokens table using your previously inserted user_id. Users aren't currently implemented, but there is a foreign key relation between the two so inserting an access token with a user id that doesn't exist is not possible.

After this is done make sure you include your access token in the Authorization header of each request like this:

`Authorization: Bearer your_token_value`

### API Endpoints

KnowledgeGPT exposes the following HTTP endpoints:

#### Add Document

**Endpoint**: `/documents`

**Method**: `POST`

**Description**: Adds a new document to the database.

**Request Body**:

```json
{
  "title": "Document Title",
  "url": "https://example.com", // Optional
  "body": "The content of the document.",
   "dataset": "my_dataset_name"
}
```

**Response**:

- `201 Created` on success.
- `400 Bad Request` if the payload is invalid.
- `500 Internal Server Error` on server-side issues.

**Example**:

```bash
curl -X POST http://localhost:8080/documents \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your_access_token" \
     -d '{
           "title": "Go Programming",
           "url": "https://golang.org",
           "body": "Go is an open-source programming language...",
           "dataset": "my_dataset_name"
         }'
```

#### Query

**Endpoint**: `/query`

**Method**: `POST`

**Description**: Performs a keyword search on the stored documents and retrieves a response from the LLM server based on the query and the retrieved documents.

**Request Body**:

```json
{
  "query": "What is Go?",
  "session_id": "optional-session-id",
  "limit": 512, // Optional; defaults to 512
  "dataset": "my_dataset_name"
}
```

**Response**:

```json
{
  "response": "Go is an open-source programming language developed by Google..."
}
```

**Example**:

```bash
curl -X POST http://localhost:8080/query \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your_access_token" \
     -d '{
           "query": "Explain the Go programming language.",
           "dataset": "my_dataset_name"
         }'
```

## Project Structure

```
KnowledgeGPT/
    |-- cmd/
    |   +-- server/
    |       +-- main.go
    |-- internal/
    |   |-- api/
    |   |   |-- documents/
    |   |   |   +-- add_document_request.go
    |   |   +-- query/
    |   |       |-- query_request.go
    |   |       |-- query_response.go
    |   |       |-- simple_query_request.go
    |   |       +-- simple_query_response.go
    |   |-- auth/
    |   |   +-- access_token_authorizer.go
    |   |-- db/
    |   |   +-- postgres.go
    |   |-- handlers/
    |   |   |-- document.go
    |   |   |-- query.go
    |   |   +-- upload.go
    |   |-- llm/
    |   |   |-- client.go
    |   |   +-- openai.go
    |   |-- models/
    |   |   +-- models.go
    |   +-- parsing/
    |       +-- pdf.go
    +-- pkg/
        +-- utils/
            |-- dotenv.go
            +-- utils.go
```

## Contributing

Contributions are welcome! To contribute to KnowledgeGPT, please follow these steps:

1. **Fork the Repository**

   Click the "Fork" button at the top right of the repository page to create your own copy.

2. **Clone Your Fork**

   ```bash
   git clone https://github.com/mrhollen/KnowledgeGPT.git
   cd KnowledgeGPT
   ```

3. **Create a New Branch**

   ```bash
   git checkout -b feature/your-feature-name
   ```

4. **Make Your Changes**

   Implement your feature or fix bugs. Ensure your code adheres to Go best practices and is well-documented.

5. **Run Tests**

   ```bash
   go test ./...
   ```

6. **Commit Your Changes**

   ```bash
   git commit -m "Add feature: your feature description"
   ```

7. **Push to Your Fork**

   ```bash
   git push origin feature/your-feature-name
   ```

8. **Create a Pull Request**

   Navigate to the original repository and create a pull request from your fork's branch.

---

## License

[Creative Commons Attribution-NonCommercial (CC BY-NC) license](./LICENSE)

*Feel free to reach out via [issues](https://github.com/mrhollen/KnowledgeGPT/issues) for any questions, suggestions, or contributions!*