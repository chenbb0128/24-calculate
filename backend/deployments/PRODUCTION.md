# 24-calculate Production Deployment

This deployment is server-only. The WeChat mini game frontend is not deployed by these files.

The production server already runs Docker, Nginx, Redis, MySQL, ClassMate, and YouQuanGou services. This project must stay isolated:

- Do not create, restart, remove, or reconfigure the shared Redis service.
- Do not create a MySQL container here. Use the external MySQL settings in `.env`.
- Do not run `docker compose down -v`.
- Use the dedicated directory `/data/website/24-calculate/server`.
- Use the dedicated containers `twenty_four_calculate_api` and `twenty_four_calculate_worker`.

## Files

```text
/data/website/24-calculate/server
`-- deployments
    |-- docker-compose.production.yml
    |-- Dockerfile.production
    |-- production.env.example
    `-- .env
```

Create `.env` on the server:

```bash
cd /data/website/24-calculate/server/deployments
cp production.env.example .env
chmod 600 .env
```

Fill all `replace-with-` values. For the migration command, prefer an alphanumeric MySQL password unless you verify that special characters are properly escaped in the MySQL DSN.

## External Services

Set `SHARED_DOCKER_NETWORK` to the existing Docker network that can reach Nginx, Redis, and MySQL. On the current server this is expected to be `docker-container_backend`, but verify before startup:

```bash
docker network ls
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Networks}}\t{{.Ports}}'
```

If Redis or MySQL is exposed only on the host, keep the network as `bridge` or the existing shared network and use this inside `GO_SERVICE_REDIS_ADDR` or `GO_SERVICE_DATABASE_HOST`:

```text
host.docker.internal:<port>
```

For shared Redis, prefer an unused logical DB, for example:

```text
GO_SERVICE_REDIS_DB=2
```

This matters because the app uses Redis keys and Asynq queue metadata.

## First Startup

Do this only after MySQL credentials are ready.

```bash
cd /data/website/24-calculate/server/deployments
docker compose -f docker-compose.production.yml --env-file .env config
docker compose -f docker-compose.production.yml --env-file .env build
docker compose -f docker-compose.production.yml --env-file .env --profile migrate run --rm migrate
docker compose -f docker-compose.production.yml --env-file .env up -d api worker
docker compose -f docker-compose.production.yml --env-file .env ps
```

## Checks

```bash
curl --fail http://127.0.0.1:18082/health
curl --fail http://127.0.0.1:18082/ready
docker compose -f docker-compose.production.yml --env-file .env logs --tail=100 api worker
```

`/health` only checks that the API process is alive. `/ready` checks MySQL and Redis too.

## Nginx

If Nginx runs in Docker on the same `SHARED_DOCKER_NETWORK`, proxy to:

```nginx
proxy_pass http://twenty_four_calculate_api:8080;
```

If Nginx runs directly on the host, proxy to:

```nginx
proxy_pass http://127.0.0.1:18082;
```

Use `nginx-api.conf.example` as a minimal reference, then run `nginx -t` before reloading the existing Nginx container.

## Upgrade

```bash
cd /data/website/24-calculate/server/deployments
docker compose -f docker-compose.production.yml --env-file .env build
docker compose -f docker-compose.production.yml --env-file .env --profile migrate run --rm migrate
docker compose -f docker-compose.production.yml --env-file .env up -d api worker
```

Back up MySQL before migrations. Redis data for rooms, matchmaking, run state, and queue metadata is temporary but still shared, so do not flush it.