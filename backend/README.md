# Backend System Documentation

This document provides a technical overview of the backend system, which is a file storage service utilizing **Block-Level Deduplication** to optimize storage efficiency and network performance.

---

## 1. Technical Stack

* **Language**: Go (Golang)
* **Web Framework**: Fiber (v2)
* **Database**: PostgreSQL (using `pgx/v5` for connection pooling)
* **Object Storage**: S3-Compatible Storage (e.g., QNAP, MinIO) via AWS SDK v2
* **Authentication**: JWT (JSON Web Tokens)
* **Documentation**: Swagger/OpenAPI

---

## 2. Core Architecture: Block-Level Deduplication

The system does not store files as monolithic objects. Instead, it employs a **Content-Addressable Storage (CAS)** model:

1. **Splitting**: Files are streamed and split into fixed-size chunks (e.g., 4MB or 8MB blocks).
2. **Hashing**: Every block is hashed using SHA-256 to create a unique "fingerprint."
3. **Deduplication**: Before uploading to S3, the system checks the PostgreSQL `blocks` table for the hash:
* **Hit**: If the hash exists, the upload is skipped, and a reference count is incremented.
* **Miss**: The block is uploaded to S3 (using the hash as the key) and registered in the database.


4. **Reconstruction**: A file is stored as a "recipe" in the `file_blocks` table, which maps a `file_id` to a list of `block_id`s in a specific `block_index` order.

---

## 3. Key Components

### Block Processor (`internal/block`)

* Manages the lifecycle of an upload by coordinating splitting, hashing, and concurrent worker threads.
* Uses a **worker pool pattern** (default 4 workers) to process and upload blocks in parallel.

### Repository Layer (`internal/repository`)

* **BlockRepository**: Handles CRUD operations for data blocks, including reference counting (`ref_count`) for garbage collection readiness.
* **FileRepository**: Manages file-level metadata and the mapping between files and their constituent blocks.
* **UserRepository**: Manages user accounts and authentication data.

### Storage Layer (`internal/storage`)

* Wraps the S3 client to provide simplified `PutObject`, `GetObject`, and `ObjectExists` methods tailored for the QNAP/S3 environment.

---

## 4. API Endpoints

### Authentication

* `POST /auth/register`: Create a new user account.
* `POST /auth/login`: Authenticate and receive a JWT.

### File Management (Requires Auth)

* `POST /files`: Upload a file using `multipart/form-data`.
* `GET /files`: List all files belonging to the authenticated user.
* `GET /files/{id}/info`: Retrieve metadata for a specific file.
* `GET /files/{id}/download`: Reconstruct and stream the file from S3 blocks to the client.

---

## 5. Data Model

* **Users**: Stores user credentials and profile info.
* **Blocks**: Stores the SHA-256 hash, S3 key, size, and reference count of unique data chunks.
* **Files**: Stores high-level metadata (filename, size, MIME type).
* **File_Blocks**: A join table that maintains the ordered sequence of blocks for every file version.

---

## 6. Security

* **JWT Middleware**: Protects all file-related routes, ensuring users can only access or modify files they own.
* **Context Management**: Uses `context.Context` throughout the processing pipeline to handle timeouts and cancellations gracefully.
