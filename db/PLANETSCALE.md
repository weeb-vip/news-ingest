# Migrations on PlanetScale (production)

Production runs on PlanetScale (Vitess). The migration job works there like anywhere else,
but it needs a database user with **DDL rights** — PlanetScale's `planetscale-writer` role is
DML-only and refuses to create tables:

```
DDL command denied to user '...', in groups [planetscale-writer],
for table '__migrations_news-ingest' (ACL check error)
```

If you see that, the credentials in `weeb-argocd/event-chart/values/news-ingest-api/values.yaml`
have been rotated back to a writer-only password. The fix is the credential, not the code.

## What the first run does

It **adopts** rather than re-runs. `anime_news` and `anime_fanart` already exist — they were
created by anime-api's migrations 37, 38 and 39, which are copied here verbatim as 1, 2 and 3.
So the job records 1-3 as applied without executing them and continues from 4, which adds the
index behind the site-wide feed.

That decision is made by `migrate` itself, not a separate command, because ordering was a
trap: the deploy runs `migrate`, which would otherwise fail on `CREATE TABLE` against live
tables and leave the migration table dirty.

## If you ever need to do it by hand

```sql
CREATE TABLE `__migrations_news-ingest` (
    version BIGINT NOT NULL PRIMARY KEY,
    dirty   BOOLEAN NOT NULL
);
INSERT INTO `__migrations_news-ingest` (version, dirty) VALUES (3, 0);
ALTER TABLE anime_news ADD INDEX idx_news_latest (published_date DESC, id);
UPDATE `__migrations_news-ingest` SET version = 4, dirty = 0;
```

Verify with `SELECT * FROM \`__migrations_news-ingest\`;` — it should read `4 | 0`.

## Note on anime-api

anime-api runs a migration job against this same database, and it succeeds only because it
has nothing to do: `schema_migrations` already records the latest version, so golang-migrate
issues no DDL. Whether its credentials could actually apply a new migration is untested —
worth knowing before relying on it.
