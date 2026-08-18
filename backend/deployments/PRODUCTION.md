# 24-calculate Production Deployment

This project deploys only the Go backend. The WeChat mini game frontend is not deployed by this server workflow.

## Current Server Layout

```text
/data/website/24-calculate/server       # Git checkout
/data/website/24-calculate/avatars      # Avatar volume
/data/backups/24-calculate/mysql        # Migration backups
/data/docker-container/services/nginx/sites/calc-api.pdurl.cn.conf
```

Runtime containers:

```text
twenty_four_calculate_api
twenty_four_calculate_worker
```

Shared services are not managed by this compose project:

```text
MySQL: 172.17.0.1:3306
Redis: docker-container-redis-1:6379, logical DB 3
Docker network: docker-container_backend
```

## Deployment Model

GitHub Actions builds and publishes the production image to GHCR:

```text
ghcr.io/chenbb0128/24-calculate-backend:<commit-sha>
```

The production server no longer compiles Go during normal deployment. It only:

1. Verifies the requested SHA is the current `origin/master`.
2. Pulls the immutable GHCR image for that SHA.
3. Fast-forwards the server checkout.
4. Backs up MySQL.
5. Runs Goose migrations.
6. Recreates only the API and worker containers.
7. Checks `/ready` locally and through `https://calc-api.pdurl.cn/ready`.

Do not run `docker compose down -v` on the production server.

## GitHub Secrets

The production workflow needs these repository secrets:

```text
PROD_HOST=116.62.159.237
PROD_PORT=22
PROD_USER=calculate-deploy
PROD_SSH_KEY=<private key for github-actions-24-calculate>
PROD_KNOWN_HOSTS=<ssh-keyscan output for the server>
```

`GITHUB_TOKEN` is used automatically for GHCR push/pull during the workflow.

## Server Install

Install the restricted deploy user and commands from the server as root:

```bash
cd /data/website/24-calculate/server
bash backend/deployments/server/install-deploy-components \
  backend/deployments/server/24-calculate-deploy-entrypoint \
  backend/deployments/server/deploy-24-calculate \
  /tmp/github-actions-24-calculate.pub
```

The installer creates `calculate-deploy`, appends one forced-command SSH key, and allows that user to run only `/usr/local/sbin/deploy-24-calculate` through sudo.

## Manual Deploy

A manual server-side deploy can still be run by root when needed:

```bash
cd /data/website/24-calculate/server
git fetch --prune origin master
sha=$(git rev-parse origin/master)
printf '%s\n%s\n' '<ghcr-user>' '<ghcr-token>' | /usr/local/sbin/deploy-24-calculate "$sha"
```

For normal releases, use the GitHub Actions workflow instead.

## Health Checks

```bash
curl --fail http://127.0.0.1:18082/health
curl --fail http://127.0.0.1:18082/ready
curl --fail https://calc-api.pdurl.cn/health
curl --fail https://calc-api.pdurl.cn/ready
```
