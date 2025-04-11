# SProto - Simple Protobuf Registry

SProto is a lightweight, self-hostable registry for managing Protobuf (`.proto`) file artifacts, inspired by the Buf Schema Registry but with a simpler feature set focused on versioning and artifact storage. It consists of a Go-based server backend and a command-line interface (CLI) client (`protoreg-cli`).

## Features

*   **Module Versioning:** Store immutable, versioned snapshots of Protobuf modules.
*   **Artifact Storage:** Stores `.proto` files as zip archives in an S3-compatible object store (MinIO by default).
*   **Simple API:** RESTful API for publishing, fetching, and listing modules and versions.
*   **CLI Client:** `protoreg-cli` for easy interaction with the registry from the command line.
*   **Dockerized:** Easily deployable using Docker and Docker Compose.

## Architecture

The system comprises the following components:

1.  **Registry Server:** A Go application providing the API. It interacts with PostgreSQL for metadata storage and MinIO for artifact (zip file) storage.
2.  **Registry Client (`protoreg-cli`):** A Go CLI tool used by developers to publish proto directories and fetch specific module versions.
3.  **PostgreSQL:** Stores metadata about modules (namespace, name) and their versions (version string, artifact digest, storage key).
4.  **MinIO:** An S3-compatible object storage server used to store the zipped Protobuf artifacts.

```
+-------------------+       +------------------------+       +---------------------+
| Developer Machine | ----> | Registry Client        | ----> | Registry Server     |
| (Proto Files)     |       | (protoreg-cli)         |       | (Go App in Docker)  |
+-------------------+       +------------------------+       +----------+----------+
                                                                        |
                                                               +--------+--------+
                                                               |                 |
                                                        +------v-------+   +-----v------+
                                                        | PostgreSQL   |   | MinIO      |
                                                        | (Metadata)   |   | (Artifacts)|
                                                        +--------------+   +------------+
```

## Getting Started (Local Development)

The easiest way to run the complete system locally is using Docker Compose.

**Prerequisites:**

*   Docker ([Install Docker](https://docs.docker.com/get-docker/))
*   Docker Compose ([Usually included with Docker Desktop](https://docs.docker.com/compose/install/))
*   Go (1.24 or later, for building the CLI locally if desired)

**Steps:**

1.  **Clone the repository (if you haven't already):**
    ```bash
    git clone <repository-url>
    cd SProto
    ```
2.  **Start the services:**
    This command will build the registry server image and start the server, PostgreSQL, and MinIO containers in detached mode.
    ```bash
    docker-compose up --build -d
    ```
3.  **Verify Services:**
    *   **Registry Server API:** Should be accessible at `http://localhost:8080`. Try `curl http://localhost:8080/health` - it should return `OK`.
    *   **MinIO Console:** Access the MinIO web UI at `http://localhost:9090`. Login with the default credentials `minioadmin` / `minioadmin`. You should see the `sproto-artifacts` bucket created automatically.
    *   **PostgreSQL:** The database is accessible internally to the server container. You can connect using tools like `psql` or a GUI client if needed (map port 5432 if accessing from host).

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

## CLI Usage (`protoreg-cli`)

The CLI tool provides commands to interact with the registry.

**Building the CLI:**

```bash
go build -o protoreg-cli ./cmd/cli
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

*   `GET /api/v1/modules`: List all modules.
*   `GET /api/v1/modules/{namespace}/{module_name}`: List versions for a specific module.
*   `GET /api/v1/modules/{namespace}/{module_name}/{version}/artifact`: Download the artifact zip file for a specific version.
*   `POST /api/v1/modules/{namespace}/{module_name}/{version}`: Publish a new module version (requires `Authorization: Bearer <token>` header and multipart/form-data with `artifact` file field).
*   `GET /health`: Simple health check endpoint (returns `OK`).

Refer to the source code (`internal/api/routes.go` and `internal/api/handlers.go`) for detailed request/response structures and status codes.

## Development

*   **Running Tests:** (TODO: Implement comprehensive tests)
*   **Building Server Binary:** `go build -o sproto-server ./cmd/server`
*   **Building CLI Binary:** `go build -o protoreg-cli ./cmd/cli`

## License

This project is licensed under the terms of the [LICENSE](./LICENSE) file.
