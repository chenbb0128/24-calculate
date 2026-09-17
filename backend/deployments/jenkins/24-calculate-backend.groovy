pipeline {
  agent any

  options {
    timestamps()
    disableConcurrentBuilds()
    buildDiscarder(logRotator(numToKeepStr: '20'))
  }

  parameters {
    string(name: 'BRANCH', defaultValue: 'master', description: '24-calculate 的生产分支；生产发布只允许 master。')
    string(name: 'APP_SHA', defaultValue: '', description: '可选：NAS hook 传入的精确 commit；手动发布留空则使用 BRANCH 最新提交。')
    booleanParam(name: 'DEPLOY', defaultValue: false, description: '勾选后构建并推送固定版本后端镜像到阿里云 ACR，然后部署 API/worker。')
    booleanParam(name: 'PUSH_ACR', defaultValue: false, description: '不勾选 DEPLOY 时，可选择只把固定版本后端镜像推送到阿里云 ACR zdzq。')
    string(name: 'TRIGGER_REPO', defaultValue: 'manual', description: '触发来源记录：NAS hook、同步任务或手动触发；只用于审计。')
  }

  environment {
    IMAGE_NAME = 'registry.cn-hangzhou.aliyuncs.com/zdzq/24-calculate-backend'
    NAS_REPO = 'ssh://chenhua@192.168.31.240/volume1/docker/24-calculate-git/24-calculate.git'
    NAS_GIT_SSH_COMMAND = 'ssh -i /var/jenkins_home/.ssh/nas_mirror_24_calc_ed25519 -o UserKnownHostsFile=/var/jenkins_home/.ssh/known_hosts -o StrictHostKeyChecking=yes -o IdentitiesOnly=yes'
    COMPONENT_PATH = 'backend'
    PROD_HOST = '116.62.159.237'
    PROD_PORT = '22'
    PROD_USER = 'calculate-deploy'
    PROD_SSH_CREDENTIALS_ID = 'twenty-four-calculate-prod-ssh'
    NO_PROXY = '127.0.0.1,localhost,192.168.31.240,116.62.159.237,calc-api.pdurl.cn'
    no_proxy = '127.0.0.1,localhost,192.168.31.240,116.62.159.237,calc-api.pdurl.cn'
  }

  stages {
    stage('从 NAS 代码镜像检出后端代码') {
      steps {
        deleteDir()
        sh '''
          set -Eeuo pipefail
          if [ "${DEPLOY:-false}" = "true" ] && [ "${BRANCH:-master}" != "master" ]; then
            echo '后端生产发布只允许 master 分支。' >&2
            exit 64
          fi
          if [ -n "${APP_SHA:-}" ] && ! printf '%s' "$APP_SHA" | grep -Eq '^[0-9a-f]{40}$'; then
            echo 'APP_SHA 必须是 40 位小写 Git commit SHA。' >&2
            exit 64
          fi
          export GIT_SSH_COMMAND="$NAS_GIT_SSH_COMMAND"
          git clone --depth=100 --branch "${BRANCH:-master}" "$NAS_REPO" source
          git -C source rev-parse "origin/${BRANCH:-master}" > BRANCH_SHA
          if [ -n "${APP_SHA:-}" ]; then
            git -C source merge-base --is-ancestor "$APP_SHA" "origin/${BRANCH:-master}" || {
              echo "APP_SHA 不属于 ${BRANCH:-master} 分支：$APP_SHA" >&2
              exit 65
            }
            git -C source checkout --detach "$APP_SHA"
          fi
          git -C source rev-parse HEAD > APP_SHA_RESOLVED
          test -d "source/$COMPONENT_PATH"
          test -f "source/$COMPONENT_PATH/deployments/Dockerfile.production"
          test -f "source/$COMPONENT_PATH/deployments/production.env.managed"
          git -C source log -1 --oneline
        '''
        script {
          env.APP_SHA = readFile('APP_SHA_RESOLVED').trim()
          if (!(env.APP_SHA ==~ /[0-9a-f]{40}/)) { error('Invalid APP_SHA: ' + env.APP_SHA) }
          env.CONFIG_SHA = sh(returnStdout: true, script: '''
            set -Eeuo pipefail
            grep -m1 '^ZDZQ_CONFIG_SHA=' source/backend/deployments/production.env.managed | cut -d= -f2-
          ''').trim()
          if (!(env.CONFIG_SHA ==~ /[0-9a-f]{64}/)) { error('Invalid CONFIG_SHA: ' + env.CONFIG_SHA) }
          env.SERVER_ENV_PROD_B64 = sh(returnStdout: true, script: '''
            set -Eeuo pipefail
            base64 -w0 source/backend/deployments/production.env.managed
          ''').trim()
          if (!(env.SERVER_ENV_PROD_B64 ==~ /[A-Za-z0-9+\\/]+={0,2}/)) { error('Invalid managed production environment payload') }
          env.IMAGE_REF = env.IMAGE_NAME + ':' + env.APP_SHA
          currentBuild.displayName = "#${env.BUILD_NUMBER} 后端 ${env.APP_SHA.take(8)}"
          currentBuild.description = "APP_SHA=${env.APP_SHA} CONFIG_SHA=${env.CONFIG_SHA} trigger=${params.TRIGGER_REPO}"
          echo '后端镜像=' + env.IMAGE_REF
        }
      }
    }

    stage('构建后端镜像') {
      steps {
        sh '''
          set -Eeuo pipefail
          docker build \
            -f "source/$COMPONENT_PATH/deployments/Dockerfile.production" \
            --build-arg GO_BUILD_PARALLELISM=2 \
            -t "$IMAGE_REF" \
            "source/$COMPONENT_PATH"
          docker image inspect "$IMAGE_REF" >/dev/null
        '''
      }
    }

    stage('推送镜像到阿里云 ACR') {
      when {
        expression { return params.PUSH_ACR == true || params.DEPLOY == true }
      }
      steps {
        withCredentials([usernamePassword(credentialsId: 'aliyun-acr-zdzq', usernameVariable: 'ACR_USER', passwordVariable: 'ACR_PASSWORD')]) {
          sh(label: '推送后端镜像到阿里云 ACR', script: '''
set -Eeuo pipefail
ACR_REGISTRY="registry.cn-hangzhou.aliyuncs.com"
ACR_IMAGE_REF="registry.cn-hangzhou.aliyuncs.com/zdzq/24-calculate-backend:$APP_SHA"
SOURCE_IMAGE_REF="$IMAGE_REF"

test -n "$SOURCE_IMAGE_REF"
test -n "$ACR_IMAGE_REF"
docker image inspect "$SOURCE_IMAGE_REF" >/dev/null
printf '%s' "$ACR_PASSWORD" | docker login --username "$ACR_USER" --password-stdin "$ACR_REGISTRY" >/dev/null
if [ "$SOURCE_IMAGE_REF" != "$ACR_IMAGE_REF" ]; then
  docker tag "$SOURCE_IMAGE_REF" "$ACR_IMAGE_REF"
fi
push_attempt=1
until docker push "$ACR_IMAGE_REF"; do
  if [ "$push_attempt" -ge 3 ]; then
    echo "docker push failed after 3 attempts: $ACR_IMAGE_REF" >&2
    exit 1
  fi
  sleep_seconds=$((push_attempt * 10))
  echo "docker push failed; retrying in $sleep_seconds s (attempt $push_attempt/3): $ACR_IMAGE_REF" >&2
  sleep "$sleep_seconds"
  push_attempt=$((push_attempt + 1))
done
docker logout "$ACR_REGISTRY" >/dev/null 2>&1 || true
echo "Pushed $ACR_IMAGE_REF"
''')
        }
      }
    }

    stage('部署后端到生产') {
      when { expression { return params.DEPLOY == true } }
      steps {
        withCredentials([usernamePassword(credentialsId: 'aliyun-acr-zdzq', usernameVariable: 'ACR_USER', passwordVariable: 'ACR_PASSWORD')]) {
          sshagent(credentials: [env.PROD_SSH_CREDENTIALS_ID]) {
            sh '''
              set -Eeuo pipefail
              set +x
              printf '%s\n%s\n' "$ACR_USER" "$ACR_PASSWORD" | ssh \
                -o BatchMode=yes \
                -o StrictHostKeyChecking=accept-new \
                -o ServerAliveInterval=30 \
                -o ServerAliveCountMax=30 \
                -p "$PROD_PORT" \
                "$PROD_USER@$PROD_HOST" \
                "$APP_SHA --image-input=acr --config-sha=$CONFIG_SHA --env-prod-b64=$SERVER_ENV_PROD_B64"
            '''
          }
        }
      }
    }

    stage('验证后端生产健康') {
      when { expression { return params.DEPLOY == true } }
      steps {
        sh 'curl --noproxy "*" --fail --show-error --silent --connect-timeout 5 --max-time 15 --retry 5 --retry-all-errors --retry-delay 3 --resolve calc-api.pdurl.cn:443:116.62.159.237 https://calc-api.pdurl.cn/ready >/dev/null'
      }
    }
  }

  post {
    always {
      sh 'docker logout registry.cn-hangzhou.aliyuncs.com >/dev/null 2>&1 || true'
    }
  }
}
