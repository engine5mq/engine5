// engine5 ve aes image'larını aynı sürümle derleyip pushlar. Sürüm: release.env -> IMAGE_TAG
node {
    stage('Checkout') {
        checkout scm
    }
    load('jenkins/build-push.groovy').run('engine5 aes')
}
