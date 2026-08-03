# Applying the news schema on PlanetScale (production)

Production runs on PlanetScale (Vitess), which **denies DDL to the application user**:

```
DDL command denied to user 'vd9692bh2vd43jvj7u75', in groups [planetscale-writer],
for table '__migrations_news-ingest' (ACL check error)
```

So the migration job cannot create tables there, and it is disabled in the production values
(`weeb-argocd/event-chart/values/news-ingest-api/values.yaml`). Staging is self-hosted MySQL
and migrates normally.

This is not specific to news-ingest. anime-api *looks* like it migrates in production, but
its job only ever succeeds because it has nothing to do: `schema_migrations` already exists
and already records the latest version, so golang-migrate issues no DDL. Every table that
service owns was created out of band. news-ingest is simply the first service to notice,
because its first run has to create its own migrations table.

## One-off setup

Run through PlanetScale's deploy-request workflow — not from a pod, and not with the
application user.

```sql
-- 1. The migration bookkeeping table golang-migrate would normally create itself.
CREATE TABLE `__migrations_news-ingest` (
    version BIGINT NOT NULL PRIMARY KEY,
    dirty   BOOLEAN NOT NULL
);

-- 2. Record migrations 1-3 as applied. They ARE applied: they are verbatim copies of
--    anime-api's 37, 38 and 39, which created these tables long ago. This is the same
--    adoption the code performs automatically on a database it is allowed to write to.
INSERT INTO `__migrations_news-ingest` (version, dirty) VALUES (3, 0);

-- 3. Migration 4: the index behind the site-wide feed. published_date DESC with id as the
--    tiebreaker, because published_date is only a DATE — several items share a day, and
--    without a stable second key pagination repeats or skips rows between requests.
ALTER TABLE anime_news ADD INDEX idx_news_latest (published_date DESC, id);

-- 4. Record it.
UPDATE `__migrations_news-ingest` SET version = 4, dirty = 0;
```

Verify:

```sql
SELECT * FROM `__migrations_news-ingest`;                 -- 4 | 0
SHOW INDEX FROM anime_news WHERE Key_name = 'idx_news_latest';
```

## Adding a migration later

1. Add the `.sql` files here as usual — staging applies them automatically.
2. Apply the same SQL to production through a PlanetScale deploy request.
3. Bump the recorded version: `UPDATE \`__migrations_news-ingest\` SET version = N;`

Step 3 is easy to forget and costs nothing until someone re-enables the job, at which point
it would try to replay everything from the stale version. If that trade becomes annoying,
the alternative is giving the migration job a PlanetScale user with DDL rights — at which
point production migrates like everywhere else, and this file can go.
