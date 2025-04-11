# SProto - Simple Protobuf Registry

[![CI & Deploy](https://github.com/Suhaibinator/SProto/actions/workflows/ci.yaml/badge.svg)](https://github.com/Suhaibinator/SProto/actions/workflows/ci.yaml)

SProto is a lightweight, self-hostable registry for managing Protobuf (`.proto`) file artifacts, inspired by the Buf Schema Registry but with a simpler feature set focused on versioning and artifact storage. It consists of a Go-based server backend and a command-line interface (CLI) client (`protoreg-cli`).

## Features

*   **Module Versioning:** Store immutable, versioned snapshots of Protobuf modules.
*   **Artifact Storage:** Stores `.proto` files as zip archives in an S3-compatible object store (MinIO by default).
*   **Simple API:** RESTful API for publishing, fetching, and listing modules and versions.
*   **CLI Client:** `protoreg-cli` for easy interaction with the registry from the command line.
*   **Dockerized:** Easily deployable using Docker and Docker Compose.

## Architecture

The system comprises the following components:

1.  **Registry Server:** A Go application providing the API. It interacts with PostgreSQL for metadata storage and MinIO (or another S3-compatible service) for artifact (zip file) storage.
2.  **Registry Client (`protoreg-cli`):** A Go CLI tool used by developers to publish proto directories and fetch specific module versions.
3.  **PostgreSQL:** Stores metadata about modules (namespace, name) and their versions (version string, artifact digest, storage key).
4.  **MinIO (or S3):** An S3-compatible object storage server used to store the zipped Protobuf artifacts.

```mermaid
graph LR
    Dev[Developer Machine<br/>(Proto Files)] --> CLI(Registry Client<br/>protoreg-cli);
    CLI --> Server(Registry Server<br/>Go App in Docker);
    Server --> DB[(PostgreSQL<br/>Metadata)];
    Server --> S3[(MinIO / S3<br/>Artifacts)];
```

## Getting Started (Local Development)

The easiest way to run the complete system locally is using Docker Compose.

**Prerequisites:**

*   Docker ([Install Docker](https://docs.docker.com/get-docker/))
*   Docker Compose ([Usually included with Docker Desktop](https://docs.docker.com/compose/install/))
*   Go (1.24 or later, required for building the server/CLI locally)

**Steps:**

1.  **Clone the repository (if you haven't already):**
    ```bash
    git clone <repository-url>
    cd SProto
    ```
2.  **Start the services:**
    This command builds the registry server image (if needed) and starts the server, PostgreSQL, and MinIO containers in detached mode.
    ```bash
    docker-compose up --build -d
    ```
3.  **Verify Services:**
    *   **Registry Server API:** Should be accessible at `http://localhost:8080`. Try `curl http://localhost:8080/health` - it should return `OK`.
    *   **MinIO Console:** Access the MinIO web UI at `http://localhost:9090`. Login with the default credentials `minioadmin` / `minioadmin`. You should see the `sproto-artifacts` bucket created automatically.
    *   **PostgreSQL:** The database is accessible internally to the server container. You can connect using tools like `psql` or a GUI client if needed (uncomment the port mapping in `docker-compose.yaml` to access from the host).
4.  **Check Logs (Optional):**
    ```bash
    docker-compose logs -f
    ```
5.  **Stop Services:**
    ```bash
    docker-compose down
    ```

## Server Configuration

The registry server (`registry-server` service in `docker-compose.yaml`) is configured primarily through environment variables. These can be set directly in the `docker-compose.yaml` file or passed through other means if deploying differently.

| Environment Variable        | Default Value                                                            | Description                                                                 |
| :-------------------------- | :----------------------------------------------------------------------- | :-------------------------------------------------------------------------- |
| `PROTOREG_SERVER_PORT`      | `8080`                                                                   | Port the registry server listens on.                                        |
| `PROTOREG_DB_DSN`           | `host=postgres user=postgres password=postgres dbname=sproto port=5432 sslmode=disable` | PostgreSQL Data Source Name.                                                |
| `PROTOREG_MINIO_ENDPOINT`   | `minio:9000`                                                             | MinIO server endpoint (use service name within Docker Compose).             |
| `PROTOREG_MINIO_ACCESS_KEY` | `minioadmin`                                                             | MinIO access key. **Change for production!**                                |
| `PROTOREG_MINIO_SECRET_KEY` | `minioadmin`                                                             | MinIO secret key. **Change for production!**                                |
| `PROTOREG_MINIO_BUCKET`     | `sproto-artifacts`                                                       | Name of the MinIO bucket to store artifacts. Will be created if it doesn't exist. |
| `PROTOREG_MINIO_USE_SSL`    | `false`                                                                  | Whether to use SSL/TLS when connecting to MinIO.                            |
| `PROTOREG_AUTH_TOKEN`       | `supersecrettoken`                                                       | Static bearer token required for publishing. **Change for production!**     |

## Security Considerations

*   **Default Credentials:** The default `docker-compose.yaml` uses insecure default credentials (`minioadmin`/`minioadmin` for MinIO, `postgres`/`postgres` for PostgreSQL) and a default auth token (`supersecrettoken`). **These MUST be changed for any production or shared deployment.** Update the environment variables in `docker-compose.yaml` or your deployment configuration.
*   **Authentication:** Publishing requires a static bearer token (`PROTOREG_AUTH_TOKEN`). Ensure this token is kept secret and has sufficient entropy. Consider more robust authentication mechanisms (like OIDC, API Keys per user/team) for production environments if needed (this would require code changes).
*   **Network Exposure:** Ensure only necessary ports are exposed to the network. The default `docker-compose.yaml` exposes the server (8080) and MinIO UI (9090). Adjust as needed.
*   **S3 Bucket Permissions:** If using a managed S3 service, configure bucket policies appropriately to restrict access.

## CLI Usage (`protoreg-cli`)

The CLI tool provides commands to interact with the registry.

**Building the CLI:**

```bash
# Build for your current OS/Arch
go build -o protoreg-cli ./cmd/cli

# Optional: Move the binary to a directory in your PATH
# sudo mv protoreg-cli /usr/local/bin/
```

**Configuration:**

The CLI loads its configuration (Registry URL and API Token) with the following precedence:

1.  Command-line flags (`--registry-url`, `--api-token`)
2.  Environment variables (`PROTOREG_REGISTRY_URL`, `PROTOREG_API_TOKEN`)
3.  Configuration file (`~/.config/protoreg/config.yaml` by default)
4.  Default values (`registry_url` defaults to `http://localhost:8080`)

**Global Flags:**

*   `--registry-url <url>`: Overrides the registry URL.
*   `--api-token <token>`: Overrides the API token.
*   `--config <path>`: Specifies a custom config file path.
*   `--log-level <level>`: Sets the logging level (`debug`, `info`, `warn`, `error`). Default is `info`.

**Commands:**

1.  **`configure`**: Saves configuration settings to the config file.
    ```bash
    # Save registry URL
    ./protoreg-cli configure --registry-url http://your-registry.example.com:8080

    # Save API token
    ./protoreg-cli configure --api-token YOUR_SECURE_TOKEN

    # Save both
    ./protoreg-cli configure --registry-url http://localhost:8080 --api-token supersecrettoken
    ```

2.  **`publish`**: Zips and uploads a directory as a new module version.
    *   Requires `--module` and `--version` flags.
    *   Requires authentication (API token).
    ```bash
    # Usage: ./protoreg-cli publish <directory> --module <namespace/name> --version <semver>
    ./protoreg-cli publish ./path/to/protos --module mycompany/user --version v1.0.0
    ```

3.  **`fetch`**: Downloads and extracts a specific module version.
    *   Requires the `--output` flag.
    ```bash
    # Usage: ./protoreg-cli fetch <namespace/module_name> <version> --output <dir>
    ./protoreg-cli fetch mycompany/user v1.0.0 --output ./downloaded-protos
    # Files will be extracted to ./downloaded-protos/mycompany/user/v1.0.0/
    ```

4.  **`list`**: Lists modules or versions.
    ```bash
    # List all modules
    ./protoreg-cli list

    # List versions for a specific module
    ./protoreg-cli list mycompany/user
    ```

## API Specification

The server exposes a simple REST API under the `/api/v1` base path.

**Health Check:**

*   `GET /health`
    *   **Success Response (200 OK):** `OK` (plain text)

**Modules:**

*   `GET /api/v1/modules`
    *   **Description:** Lists all registered modules with their latest version.
    *   **Success Response (200 OK):**
        ```json
        {
          "modules": [
            {
              "namespace": "mycompany",
              "name": "billing",
              "latest_version": "v1.2.0"
            },
            {
              "namespace": "mycompany",
              "name": "user",
              "latest_version": "v0.1.5"
            },
            {
              "namespace": "another-org",
              "name": "common",
              "latest_version": "" // If no versions published yet
            }
          ]
        }
        ```
    *   **Error Response (500 Internal Server Error):** `{"error": "Failed to retrieve modules"}`

*   `GET /api/v1/modules/{namespace}/{module_name}`
    *   **Description:** Lists all available versions for a specific module, sorted semantically descending.
    *   **URL Parameters:**
        *   `namespace`: The module's namespace (e.g., `mycompany`).
        *   `module_name`: The module's name (e.g., `user`).
    *   **Success Response (200 OK):**
        ```json
        {
          "namespace": "mycompany",
          "module_name": "user",
          "versions": [
            "v1.0.0",
            "v0.9.1",
            "v0.9.0"
          ]
        }
        ```
    *   **Error Response (404 Not Found):** `{"error": "Module not found"}`
    *   **Error Response (500 Internal Server Error):** `{"error": "Failed to retrieve module"}` or `{"error": "Failed to retrieve module versions"}`

**Artifacts:**

*   `GET /api/v1/modules/{namespace}/{module_name}/{version}/artifact`
    *   **Description:** Downloads the zipped artifact for a specific module version.
    *   **URL Parameters:**
        *   `namespace`, `module_name`, `version` (e.g., `v1.0.0`).
    *   **Success Response (200 OK):**
        *   `Content-Type: application/zip`
        *   `Content-Disposition: attachment; filename="{namespace}_{module_name}_{version}.zip"`
        *   Body: The raw zip file content.
    *   **Error Response (404 Not Found):** `{"error": "Module version not found"}`
    *   **Error Response (500 Internal Server Error):** `{"error": "Failed to retrieve module version"}` or `{"error": "Failed to retrieve artifact"}`

*   `POST /api/v1/modules/{namespace}/{module_name}/{version}`
    *   **Description:** Publishes a new module version artifact.
    *   **URL Parameters:**
        *   `namespace`, `module_name`, `version` (e.g., `v1.0.0`).
    *   **Headers:**
        *   `Authorization: Bearer <your-auth-token>` (Required)
        *   `Content-Type: multipart/form-data; boundary=...` (Required)
    *   **Form Data:**
        *   `artifact`: The zip file containing the `.proto` files for this version.
    *   **Success Response (201 Created):**
        ```json
        {
          "namespace": "mycompany",
          "module_name": "user",
          "version": "v1.0.0",
          "artifact_digest": "sha256:abcdef123...", // SHA256 hash of the uploaded zip
          "created_at": "2023-10-27T10:00:00Z"
        }
        ```
    *   **Error Response (400 Bad Request):** `{"error": "Invalid version format"}` or `{"error": "Missing artifact file"}` or `{"error": "Failed to process artifact"}`
    *   **Error Response (401 Unauthorized):** `{"error": "Unauthorized"}` (If token is missing or invalid)
    *   **Error Response (409 Conflict):** `{"error": "Module version already exists"}`
    *   **Error Response (500 Internal Server Error):** `{"error": "Failed to save module metadata"}` or `{"error": "Failed to upload artifact"}`

## Development

*   **Running Tests:** Unit tests for the API handlers use `sqlmock` for database interactions. Run them using the standard Go test command:
    ```bash
    go test ./internal/api/...
    ```
    *(Note: More comprehensive integration tests involving actual DB/MinIO interactions could be added.)*
*   **Building Server Binary:** `go build -o sproto-server ./cmd/server`
*   **Building CLI Binary:** `go build -o protoreg-cli ./cmd/cli`

## License

This project is licensed under the terms of the [LICENSE](./LICENSE) file.
