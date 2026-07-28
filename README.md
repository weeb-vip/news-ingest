# news-ingest

The bridge that puts **AI-researched anime news + fanart** onto weeb.vip. The
`anime-research` tool POSTs here; this service normalizes + dedupes each item, produces it
to Kafka, and a consumer upserts it into the weeb MySQL DB, where **anime-api** exposes it as
`Anime.news` / `Anime.fanart`.

```
anime-research  ──HTTP──►  news-ingest (serve-api)  ──►  Kafka (anime.news.v1 / anime.fanart.v1)
                                                             │
                          news-ingest (serve-consumer)  ◄────┘  ──►  MySQL (anime_news / anime_fanart)  ──►  anime-api GraphQL
```

Two run modes (one binary, pick via the first arg — mirrors the other weeb-vip services' Helm `args`):

```bash
news-ingest serve-api        # HTTP ingest → Kafka
news-ingest serve-consumer   # Kafka → MySQL
```

## HTTP API

`POST /v1/news` (optional `Authorization: Bearer $INGEST_TOKEN`)

```jsonc
{
  "anime_id": "c7e45e8d-…",        // weeb.vip id (required — the dedupe/GraphQL key)
  "mal_id": 60543,
  "title": "…", "status": "…",
  "news": [
    { "title": "…", "summary": "…", "category": "renewal",
      "date": "2026-07-28", "url": "https://…", "source": "animenewsnetwork.com", "episode": null }
  ],
  "fanart": ["https://…/img.jpg"],
  "researched_at": "2026-07-28T12:00:00Z"
}
```
Undated news items are dropped. Returns `202 { ok, news, fanart }`. `GET /health` → `ok`.

## Deduplication

Each item gets a stable id so re-runs never duplicate (the id is both the Kafka key and the
MySQL primary key; the consumer upserts):

- **news:** `sha1(anime_id + normalized_url)` — one article = one row. Falls back to
  `sha1(anime_id + iso_date + title_slug)` when there's no URL.
- **fanart:** `sha1(anime_id + image_url)`.

URLs are normalized (lowercase host, drop `www`/query/fragment/trailing slash) and dates to
`YYYY-MM-DD` before hashing. anime-api's `anime_news` table also carries a
`UNIQUE(anime_id, published_date, title_slug)` fallback index.

## Config (env vars)

| var | default | notes |
|---|---|---|
| `PORT` | `3000` | api http port |
| `KAFKA_BOOTSTRAP_SERVERS` | `localhost:9092` | prod: `kafka.confluent.svc.cluster.local:9092` |
| `KAFKA_CONSUMER_GROUP_NAME` | `news-ingest` | consumer group |
| `KAFKA_NEWS_TOPIC` | `anime.news.v1` | |
| `KAFKA_FANART_TOPIC` | `anime.fanart.v1` | |
| `KAFKA_OFFSET` | `earliest` | first-run offset |
| `INGEST_TOKEN` | _(unset)_ | if set, required as a bearer token |
| `DBHOST`/`DBNAME`/`DBUSERNAME`/`DBPASSWORD`/`DBPORT`/`DBSSL` | weeb defaults | consumer only |

## Notes

- Uses **segmentio/kafka-go** (pure Go, no CGO/librdkafka) so it builds and runs anywhere,
  including CI, without system deps. The rest of the platform uses confluent-kafka-go via the
  `ep` framework — this service doesn't consume Debezium CDC, so it doesn't need that
  machinery. Straightforward to switch to confluent if uniformity is preferred.
- The `anime_news` / `anime_fanart` tables are **owned and migrated by anime-api**; this
  service only writes rows.
- The consumer commits offsets even when a handler errors, so a poison message can't
  crash-loop it (upserts are idempotent).
