# 24-calculate Production Deployment

This project deploys only the Go backend. The WeChat mini game frontend is not
deployed by the production server workflow.

## Current Server Layout

```text
/data/website/24-calculate/server       # Git checkout
/data/website/24-calculate/avatars      # Avatar volume
/data/backups/24-calculate/mysql        # MySQL backups
/data/backups/24-calculate/env          # protected .env backups
/data/docker-container/services/nginx/sites/calc-api.pdurl.cn.conf
/etc/24-calculate/docker-compose.production.yml
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

Production follows the same ACR/Jenkins pattern used by the other image-based
projects on this server.

GitHub Actions is responsible for CI and mirroring `master` to NAS. It does not
deploy production directly. The old GHCR-based GitHub Actions deployment is
disabled because the production server now requires a managed config payload.

Jenkins builds and publishes the backend image:

```text
registry.cn-hangzhou.aliyuncs.com/zdzq/24-calculate-backend:<commit-sha>
```

Then Jenkins SSHes to the restricted production entrypoint with:

```text
<commit-sha> --image-input=acr --config-sha=<64-char sha> --env-prod-b64=<base64 managed env>
```

The production server then:

1. Verifies the requested SHA is the current `origin/master`.
2. Pulls the immutable ACR image for that SHA.
3. Fast-forwards the server checkout.
4. Installs the managed production environment while preserving protected
   secret values from the server-side root-owned `.env`.
5. Backs up MySQL.
6. Runs Goose migrations.
7. Recreates only the API and worker containers.
8. Checks `/ready` locally and through `https://calc-api.pdurl.cn/ready`.

Do not run `docker compose down -v` on the production server.

## Jenkins Job

Use [jenkins/24-calculate-backend.groovy](jenkins/24-calculate-backend.groovy)
as the production Jenkins pipeline definition.

Expected Jenkins credentials:

```text
aliyun-acr-zdzq              # username/password for registry.cn-hangzhou.aliyuncs.com
jenkins-24-calculate-prod    # SSH key authorized for /usr/local/bin/24-calculate-deploy-local-image-entrypoint
```

The Jenkins job reads the non-secret managed environment template from:

```text
backend/deployments/production.env.managed
```

Sensitive values in that template must remain `${PROD_SECRET:...}` markers.
The production server resolves those markers from its existing protected
`backend/deployments/.env`.

## Server Entrypoints

The Jenkins deployment key is restricted to:

```text
/usr/local/bin/24-calculate-deploy-local-image-entrypoint
```

That entrypoint validates the commit SHA, `--image-input`, `--config-sha`, and
`--env-prod-b64`, then calls:

```text
/usr/local/sbin/deploy-24-calculate
```

The repository keeps install scripts for initial setup and recovery, but the
current production host already has the restricted deploy user and root-owned
deployment scripts installed.

For recovery installs, run the installer as root with the Jenkins deploy public
key:

```bash
cd /data/website/24-calculate/server
bash backend/deployments/server/install-deploy-components \
  backend/deployments/server/24-calculate-deploy-local-image-entrypoint \
  backend/deployments/server/deploy-24-calculate \
  backend/deployments/server/24-calculate-managed-env.sh \
  /tmp/jenkins-24-calculate-prod.pub
```

## Manual Server-Side Deploy

A root operator can still deploy manually from the production server if needed:

```bash
cd /data/website/24-calculate/server
git fetch --prune origin master
sha="$(git rev-parse origin/master)"
env_template="$(mktemp /tmp/24-calculate-env-template.XXXXXX)"
git show "$sha:backend/deployments/production.env.managed" > "$env_template"
config_sha="$(grep -m1 '^ZDZQ_CONFIG_SHA=' "$env_template" | cut -d= -f2-)"
env_prod_b64="$(base64 -w0 "$env_template")"
rm -f "$env_template"
printf '%s\n%s\n' '<acr-user>' '<acr-password>' \
  | /usr/local/sbin/deploy-24-calculate \
      "$sha" \
      --image-input=acr \
      --config-sha="$config_sha" \
      --env-prod-b64="$env_prod_b64"
```

For normal releases, use Jenkins instead.

## Health Checks

```bash
curl --fail http://127.0.0.1:18082/health
curl --fail http://127.0.0.1:18082/ready
curl --fail https://calc-api.pdurl.cn/health
curl --fail https://calc-api.pdurl.cn/ready
```
