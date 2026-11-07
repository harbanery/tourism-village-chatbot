# Support Agent Golang — AI Chatbot Desa Wisata (RAG + Vertex AI)

Backend **AI support agent untuk desa wisata** (mendukung website [DesakuWisataku / tourism-village](https://github.com/)) yang dibangun dengan **Golang**, **Fiber**, **GORM**, dan **PostgreSQL**, dengan pipeline **RAG (Retrieval-Augmented Generation)** di atas **Google Vertex AI**:

- **Embedding** dokumen & gambar via model embedding Vertex AI (default `gemini-embedding-001`, mendukung **EmbeddingGemma** — cukup ganti `vertex.embedding_model`).
- **Generation** jawaban via **Gemini** di Vertex AI (default `gemini-2.5-flash`), multimodal untuk deskripsi gambar.
- **Knowledge base** PostgreSQL: impor teks, file dokumen (`.txt`, `.md`, `.csv`, `.json`, `.html`, `.pdf`), dan gambar (`.jpg`, `.png`, `.webp`).
- Struktur project mengikuti pola **c-access** (`config/`, `app/<module>/`, `entity/`, `helper/`, `middleware/`, `route/`).

## Fitur

| Fitur | Keterangan |
| --- | --- |
| RAG chat | Pertanyaan di-embed → retrieval chunk ter relevan (cosine similarity) → jawaban digenerate Gemini dengan konteks + riwayat chat |
| Impor dokumen | Upload file teks/PDF, otomatis di-chunk dan di-embed ke PostgreSQL |
| Impor gambar | Gambar dideskripsikan Gemini (multimodal), lalu deskripsinya yang di-embed sehingga gambar bisa dicari |
| Kutipan sumber | Setiap jawaban menyimpan `sources` (dokumen + skor similarity) |
| Session & riwayat | Sesi percakapan tersimpan di PostgreSQL dengan histori |
| API key & locale | Proteksi `X-API-Key` (opsional) dan `X-Lang` (ID/EN) untuk pesan respons |
| Auto-migrate | Skema tabel dibuat otomatis saat startup (GORM AutoMigrate) |

## Pipeline RAG

```
                ┌──────────────────────────── INGESTION ────────────────────────────┐
teks / file  ──►│ chunking (paragraf→kalimat, ukuran + overlap)                     │
dokumen/pdf     │ EmbeddingGemma / gemini-embedding (Vertex AI :predict)            │──► documents
                │                                                                    │    document_chunks (jsonb)
gambar       ──►│ Gemini vision → deskripsi teks → embedding deskripsi              │        │
                └────────────────────────────────────────────────────────────────────┘        │
                                                                                            │
                ┌──────────────────────────── CHAT ────────────────────────────┐           │
pertanyaan   ──►│ embedding pertanyaan (task_type RETRIEVAL_QUERY)             │           │
                │ cosine similarity vs semua chunk → top-K (chat.top_k)        │◄──────────┘
                │ prompt: system prompt + riwayat + KONTEKS + pertanyaan       │
                │ Gemini generateContent (Vertex AI) → jawaban + sources       │──► respons
                └───────────────────────────────────────────────────────────────┘
```

Embedding disimpan sebagai `jsonb` dan similarity dihitung di aplikasi, sehingga **tidak butuh extension pgvector** (bisa di-upgrade ke pgvector jika korpus membesar).

## Teknologi

- [Go](https://go.dev/) 1.26+
- [Fiber v2](https://gofiber.io/) — HTTP framework
- [GORM](https://gorm.io/) + [PostgreSQL](https://www.postgresql.org/)
- [Viper](https://github.com/spf13/viper) — konfigurasi `config.json`
- [logrus](https://github.com/sirupsen/logrus) + rotate file hook — logging
- [validator](https://github.com/go-playground/validator) — validasi request
- [Vertex AI](https://cloud.google.com/vertex-ai) — embedding & generation (`golang.org/x/oauth2` untuk ADC)
- [ledongthuc/pdf](https://github.com/ledongthuc/pdf) — ekstraksi teks PDF

## Struktur Project

```
support-agent-golang/
├── main.go                     # entrypoint & bootstrap (pola c-access)
├── config.json.example         # contoh konfigurasi
├── response_message.json       # master pesan respons (ID/EN)
├── config/
│   ├── app.go                  # Bootstrap + wiring DI (repo → usecase → handler → route)
│   ├── fiber.go                # HTTP server + error handler
│   ├── gorm.go                 # koneksi PostgreSQL
│   ├── logrus.go               # logger + rotasi file logs/
│   ├── validator.go
│   └── viper.go                # pembacaan config.json
├── app/
│   ├── chat/                   # modul chat (session, RAG, history)
│   │   ├── chat.handler.go
│   │   ├── chat.usecase.go     # pipeline RAG: embed → retrieve → generate
│   │   ├── chat.repository.go
│   │   └── chat.model.go
│   └── knowledge/              # modul knowledge base (ingest, search)
│       ├── knowledge.handler.go
│       ├── knowledge.usecase.go# chunking, embedding, ekstraksi PDF, deskripsi gambar
│       ├── knowledge.repository.go
│       └── knowledge.model.go
├── entity/                     # entitas GORM (documents, chunks, sessions, messages)
├── helper/
│   ├── vertexai.go             # client REST Vertex AI (predict & generateContent)
│   ├── chunker.go              # text splitter + cosine similarity
│   ├── response.go             # WebResponse standar (pola c-access)
│   ├── response_message.go
│   ├── validate_request.go
│   └── helper.go
├── middleware/
│   └── auth.middleware.go      # X-API-Key + X-Lang
├── route/
│   └── route.go
├── Dockerfile
└── docker-compose.yaml         # PostgreSQL 16
```

## Prasyarat

1. **Go 1.26+** (toolchain lama akan otomatis ter-download via `GOTOOLCHAIN=auto`).
2. **PostgreSQL** — jalankan cepat: `docker compose up -d` (membuat DB `support_agent`).
3. **Google Cloud**:
   - Buat project & aktifkan **Vertex AI API** (`aiplatform.googleapis.com`).
   - Login lokal: `gcloud auth application-default login`, **atau** siapkan **service account JSON** (peran `Vertex AI User`) lalu isi `vertex.credentials_file`.

## Setup

```bash
# 1. Database
docker compose up -d

# 2. Konfigurasi
copy config.json.example config.json
#    - isi vertex.project_id (WAJIB)
#    - sesuaikan database.* bila perlu
#    - opsional: isi api.key untuk memproteksi endpoint

# 3. Autentikasi GCP (pilih salah satu)
gcloud auth application-default login
# atau isi "credentials_file": "C:/path/service-account.json" di config.json

# 4. Jalankan
go mod download
go run .
```

Server berjalan di `http://localhost:8080` (port dari `web.port`). Tabel dibuat otomatis saat startup.

> Tanpa kredensial GCP server tetap bisa start; endpoint yang butuh Vertex AI akan mengembalikan `502` dengan petunjuk autentikasi di log.

## API

Semua endpoint di bawah grup `/api` dengan middleware `X-API-Key` (jika `api.key` diisi) dan `X-Lang: ID|EN` (opsional, default `ID`).

### Chat

| Method | Path | Keterangan |
| --- | --- | --- |
| POST | `/api/chat/sessions` | Buat sesi percakapan |
| POST | `/api/chat/messages` | Kirim pertanyaan (RAG) |
| GET | `/api/chat/sessions/:id/messages` | Riwayat percakapan |

```bash
# Buat sesi
curl -X POST http://localhost:8080/api/chat/sessions \
  -H "Content-Type: application/json" \
  -d '{"user_id":"budi","title":"Tanya paket"}'

# Kirim pertanyaan
curl -X POST http://localhost:8080/api/chat/messages \
  -H "Content-Type: application/json" \
  -d '{"session_id":"<session_id>","message":"Paket wisata apa saja yang tersedia dan berapa harganya?"}'
```

Contoh respons:

```json
{
  "code": 0,
  "message": "Sukses",
  "data": {
    "session_id": "...",
    "answer": "Berdasarkan informasi desa, tersedia paket ... [1]",
    "sources": [
      { "chunk_id": "...", "document_id": "...", "title": "Daftar Paket Wisata", "score": 0.68 }
    ],
    "latency_ms": 1420
  }
}
```

### Knowledge Base

| Method | Path | Keterangan |
| --- | --- | --- |
| POST | `/api/knowledge/text` | Impor teks langsung |
| POST | `/api/knowledge/documents` | Upload file (`file`) / gambar — multipart |
| GET | `/api/knowledge/documents` | Daftar dokumen |
| DELETE | `/api/knowledge/documents/:id` | Hapus dokumen + chunk-nya |
| POST | `/api/knowledge/search` | Uji retrieval (debug kualitas RAG) |

```bash
# Impor teks (mis. data paket dari tourism-village)
curl -X POST http://localhost:8080/api/knowledge/text \
  -H "Content-Type: application/json" \
  -d '{"title":"Paket Wisata","content":"Paket Family Outbound Rp250.000 ..."}'

# Upload dokumen (pdf/txt/md/csv/json/html)
curl -X POST http://localhost:8080/api/knowledge/documents \
  -F "file=@panduan-desa.pdf" -F "title=Panduan Desa Wisata"

# Upload gambar (dideskripsikan Gemini, lalu di-embed)
curl -X POST http://localhost:8080/api/knowledge/documents \
  -F "file=@homestay.jpg"

# Uji retrieval
curl -X POST http://localhost:8080/api/knowledge/search \
  -H "Content-Type: application/json" \
  -d '{"query":"harga homestay","top_k":3}'
```

### Lainnya

| Method | Path | Keterangan |
| --- | --- | --- |
| GET | `/health` | Health check |
| GET | `/version/development` | Info versi |

## Konfigurasi (`config.json`)

| Key | Default | Keterangan |
| --- | --- | --- |
| `web.port` | `8080` | Port HTTP |
| `web.body_limit_mb` | `50` | Batas ukuran body upload |
| `database.*` | localhost | Koneksi PostgreSQL + pool |
| `api.key` | `""` | Jika diisi, wajib header `X-API-Key` |
| `vertex.project_id` | — | Project ID GCP (**wajib diisi**) |
| `vertex.region` | `asia-southeast2` | Region Vertex AI (Jakarta) |
| `vertex.credentials_file` | `""` | Path service account JSON; kosong = ADC |
| `vertex.embedding_model` | `gemini-embedding-001` | Model embedding; ganti ke `embeddinggemma-001` (EmbeddingGemma) atau `text-embedding-005` bila tersedia di project |
| `vertex.embedding_dimension` | `768` | Dimensi output embedding (MRL) |
| `vertex.embedding_batch_size` | `5` | Jumlah teks per request predict |
| `vertex.chat_model` | `gemini-2.5-flash` | Model Gemini untuk jawaban & deskripsi gambar |
| `knowledge.chunk_size` | `1200` | Ukuran chunk (karakter) |
| `knowledge.chunk_overlap` | `150` | Overlap antar chunk |
| `knowledge.max_file_size_mb` | `20` | Batas ukuran file upload |
| `chat.top_k` | `5` | Jumlah chunk yang dipakai sebagai konteks |
| `chat.min_score` | `0.3` | Ambang cosine similarity minimum |
| `chat.max_history` | `10` | Jumlah pesan riwayat yang dikirim ke Gemini |
| `chat.temperature` | `0.2` | Temperature generation |
| `chat.system_prompt` | `""` | Override system prompt default (bahasa Indonesia, khusus desa wisata) |

## Catatan Teknis

- **Model embedding seragam**: dokumen dan query harus di-embed dengan model + dimensi yang sama. Jika `vertex.embedding_model` diganti, hapus dan impor ulang semua dokumen agar vectornya konsisten.
- **Task type**: ingestion memakai `RETRIEVAL_DOCUMENT`, pertanyaan memakai `RETRIEVAL_QUERY` sesuai rekomendasi Google untuk RAG.
- **Skala**: cosine similarity dihitung di aplikasi (korpus ribuan chunk masih cepat). Untuk korpus besar, migrasi kolom `embedding` ke `vector` pgvector + indeks HNSW.
- **Struktur respons** (`code`/`message`/`data`) dan file `response_message.json` mengikuti pola c-access agar konsisten dengan service lain.

## Validasi

```bash
go build ./...   # ✅ compile
go vet ./...     # ✅ static analysis
go test ./...    # ✅ unit test (chunker & similarity)
```

## Lisensi

MIT — sama seperti proyek tourism-village.
