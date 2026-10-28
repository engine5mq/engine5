// Sadece aes image'ını derleyip pushlar. Sürüm: release.env -> IMAGE_TAG
node {
    stage('Checkout') {
        checkout scm
    }
    load('jenkins/build-push.groovy').run('aes')
}
