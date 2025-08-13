# TinyImage Server

A simple, lightweight image processing and compression server built with Go and Fiber. This server provides both HTTP and WebSocket APIs to queue and process image compression tasks, supporting **WebP**, **PNG**, and **JPEG** formats.

## Features

* **Asynchronous Processing:** Uses a worker pool to handle image compression tasks without blocking the main server thread.
* **Multiple Formats:** Supports converting and compressing images to WebP, PNG, and JPEG.
* **WebSocket API:** Enables real-time, bidirectional communication for image uploads and instant previews of processed images.
* **Efficient Compression:** Leverages `bimg` (a Go wrapper for `libvips`) and `pngquant` for high-quality, efficient compression.
* **Configurable:** Easily customize server port, task concurrency, and maximum upload size via a `config.yaml` file.
* **Automatic Cleanup:** Periodically deletes old processed files to manage disk space.

## Deployment

The easiest way to deploy this server is by using the provided `Dockerfile` and Docker Compose.

### Prerequisites

* **Docker:** [Install Docker](https://docs.docker.com/get-docker/) on your system.
* **Docker Compose:** [Install Docker Compose](https://docs.docker.com/compose/install/) (comes with Docker Desktop).

### Steps

1.  **Clone the Repository**

    ```bash
    git clone https://github.com/your-username/your-repo-name.git
    cd your-repo-name
    ```

2.  **Configure the Server**
    Edit the `config.yaml` file to set your desired options.

    * **`server.port`**: The port the server will listen on.
    * **`server.output_dir`**: The directory to save processed images.
    * **`upload.max_upload_size`**: The maximum file size for uploads (e.g., `10MB`, `500KB`).
    * **`upload.max_concurrent_tasks`**: The number of concurrent image processing workers.

    <!-- end list -->

    ```yaml
    # config.yaml example
    server:
      port: 8080
      output_dir: "output"

    upload:
      max_upload_size: 10MB
      max_concurrent_tasks: 3

    download:
      download_url: "download/"
    ```

3.  **Build and Run with Docker Compose**
    Create a `docker-compose.yaml` file in the project root with the following content:

    ```yaml
    version: '3.8'
    services:
      tinyimage-server:
        build:
          context: .
          dockerfile: Dockerfile
        ports:
          - "8080:8080"
        volumes:
          - ./config.yaml:/config/config.yaml
          - ./output:/app/output
    ```

    Then, run the following command to build and start the server:

    ```bash
    docker-compose up --build -d
    ```

    The server will now be running at `http://localhost:8080`.

## API Documentation

The server provides a RESTful API and a WebSocket API.

### 1\. HTTP API

#### `GET /` - Server Status

**Description:** Get server configuration and version info.
**Example:**

```bash
curl http://localhost:8080/
```

**Response:**

```json
{
  "version": "v0.0.1",
  "download_url": "download/",
  "max_upload_size_bytes": 10485760,
  "max_concurrent_tasks": 3
}
```

#### `POST /upload` - Upload Image

**Description:** Upload an image for processing. The server queues the task and returns a unique MD5 hash.
**Request:** `multipart/form-data`

* `picture`: (file, required) The image file.
* `format`: (string, optional) Output format (`webp`, `png`, `jpg`). Default is `webp`.
* `quality`: (int, optional) For `jpg` format, 1-100. Default is `80`.
  **Example:**

<!-- end list -->

```bash
curl -X POST http://localhost:8080/upload \
  -F "picture=@/path/to/your/image.jpg" \
  -F "format=png"
```

**Response:**

```json
{
  "message": "Your image is queued for processing",
  "md5": "a1b2c3d4e5f6...",
  "format": "png",
  "quality": 0
}
```

-----

#### `GET /status/:md5` - Check Task Status

**Description:** Check the processing status of an image.
**Example:**

```bash
curl http://localhost:8080/status/a1b2c3d4e5f6...
```

**Response:**

```json
[
  {
    "md5": "a1b2c3d4e5f6...",
    "status": "processing",
    "format": "png",
    "quality": 0
  }
]
```

-----

#### `GET /download/:md5` - Download Processed Image

**Description:** Download the processed image file.
**Example:**

```bash
curl -o processed_image.png http://localhost:8080/download/a1b2c3d4e5f6...
```

**Response:** The raw image file.

### 2\. WebSocket API

The WebSocket API allows for real-time communication, providing instant feedback and the processed image file directly over the connection.

#### **Endpoint:** `ws://localhost:8080/ws`

**Client to Server (Request):**
Send a JSON object containing the Base64-encoded image data.

```json
{
  "filename": "my_image.jpeg",
  "format": "jpg",
  "data": "iVBORw0KGgo...",
  "quality": 80
}
```

**Server to Client (Response):**
The server will send messages with status updates. When the image is processed, a `done` message with the Base64-encoded output file is sent.

* **Queued:**
  ```json
  {"status": "queued", "md5": "a1b2c3d4...", "format": "jpg", "quality": 80}
  ```
* **Processing:**
  ```json
  {"status": "processing", "md5": "a1b2c3d4...", "format": "jpg", "quality": 80}
  ```
* **Done:**
  ```json
  {
    "status": "done",
    "md5": "a1b2c3d4...",
    "format": "jpg",
    "quality": 80,
    "file": "iVBORw0KGgo..."
  }
  ```
* **Error:**
  ```json
  {"status": "error", "message": "unsupported_format"}
  ```
    ```json
  {"status": "error", "message": "unsupported_format"}
  ```