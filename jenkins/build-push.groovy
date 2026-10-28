// Ortak build & push akışı. Jenkinsfile'lar `load` ile çağırır:
//   load('jenkins/build-push.groovy').run('engine5 aes')
// Sürüm release.env -> IMAGE_TAG'den okunur; IMAGE_TAG parametresi doluysa override eder.

def run(String defaultTargets) {
    properties([
        disableConcurrentBuilds(),
        parameters([
            string(name: 'IMAGE_TAG', defaultValue: '', description: 'Boş bırakılırsa release.env içindeki IMAGE_TAG kullanılır'),
            string(name: 'TARGETS', defaultValue: defaultTargets, description: 'Derlenecek image\'lar (engine5 aes)'),
            string(name: 'PLATFORMS', defaultValue: '', description: 'Boş bırakılırsa release.env içindeki PLATFORMS kullanılır'),
            booleanParam(name: 'PUSH', defaultValue: true, description: 'false ise sadece derler, registry\'ye göndermez'),
            booleanParam(name: 'SETUP_QEMU', defaultValue: true, description: 'Multi-arch (arm) build için binfmt/QEMU kurulumu'),
            string(name: 'DOCKER_CREDENTIALS_ID', defaultValue: 'dockerhub', description: 'Jenkins username/password credential id'),
        ]),
    ])

    def releaseTag = sh(returnStdout: true, script: '. ./release.env && printf "%s" "$IMAGE_TAG"').trim()
    def tag = params.IMAGE_TAG?.trim() ? params.IMAGE_TAG.trim() : releaseTag
    currentBuild.displayName = "#${env.BUILD_NUMBER} ${tag} [${params.TARGETS}]"

    def buildEnv = ["IMAGE_TAG=${tag}", "TARGETS=${params.TARGETS}", "PUSH=${params.PUSH}"]
    if (params.PLATFORMS?.trim()) {
        buildEnv << "PLATFORMS=${params.PLATFORMS.trim()}"
    }

    if (params.SETUP_QEMU) {
        stage('Setup QEMU') {
            sh 'docker run --privileged --rm tonistiigi/binfmt --install arm64,arm'
        }
    }

    stage('Build & Push') {
        withEnv(buildEnv) {
            if (params.PUSH) {
                withCredentials([usernamePassword(credentialsId: params.DOCKER_CREDENTIALS_ID,
                        usernameVariable: 'DOCKER_USER', passwordVariable: 'DOCKER_PASS')]) {
                    sh '''
                        set -e
                        echo "$DOCKER_PASS" | docker login -u "$DOCKER_USER" --password-stdin
                        trap 'docker logout >/dev/null 2>&1 || true' EXIT
                        ./build-push-docker.sh
                    '''
                }
            } else {
                sh './build-push-docker.sh'
            }
        }
    }
}

return this
