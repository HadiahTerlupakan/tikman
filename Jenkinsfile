// Deploys TikMan to the VPS Jenkins itself runs on, so the pipeline builds and
// starts the stack in place — no registry sits in the path. Secrets live in an
// env file on the host and never enter the repository or Jenkins.
pipeline {
    agent any

    options {
        timestamps()
        disableConcurrentBuilds()
        buildDiscarder(logRotator(numToKeepStr: '20'))
    }

    environment {
        ENV_FILE = '/opt/tikman/.env'
        COMPOSE = 'docker compose --env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml'
    }

    stages {
        stage('Preflight') {
            steps {
                sh '''
                    set -eu

                    test -f "$ENV_FILE" || {
                        echo "Missing $ENV_FILE. It holds ENCRYPTION_KEY, the database and"
                        echo "session secrets, and CLOUDFLARE_TUNNEL_TOKEN, and is deliberately"
                        echo "kept out of the repository. See .env.example."
                        exit 1
                    }

                    # The container cannot load the module; only the host can. Without it
                    # the API still starts and the VPN page reports every site as never
                    # connected, which points the operator at the wrong end of the tunnel.
                    lsmod | grep -q '^wireguard' || {
                        echo "The wireguard kernel module is not loaded on this host."
                        echo "Run once:      sudo modprobe wireguard"
                        echo "Make it stick: echo wireguard | sudo tee /etc/modules-load.d/wireguard.conf"
                        exit 1
                    }

                    $COMPOSE config >/dev/null
                '''
            }
        }

        stage('Build') {
            steps {
                sh 'set -eu; $COMPOSE build'
            }
        }

        stage('Deploy') {
            steps {
                sh 'set -eu; $COMPOSE up -d --remove-orphans'
            }
        }

        stage('Verify') {
            steps {
                sh '''
                    set -eu

                    $COMPOSE exec -T api wget -q -O /dev/null http://localhost:8080/health || {
                        echo "The API did not answer /health."
                        exit 1
                    }

                    # 401 proves the VPN routes are registered and merely want a session.
                    # A 404 would mean this image predates the module — the failure that
                    # once left the menu present but every call dead.
                    $COMPOSE exec -T api wget -O /dev/null \
                        http://localhost:8080/api/v1/wireguard/server 2>&1 | grep -q '401' || {
                        echo "The VPN route did not answer 401. This build looks like it"
                        echo "predates the WireGuard module."
                        exit 1
                    }

                    echo "API healthy and the VPN routes are registered."
                '''
            }
        }
    }

    post {
        failure {
            sh 'set +e; $COMPOSE ps; $COMPOSE logs --tail=50 api worker frontend cloudflared'
        }
    }
}
