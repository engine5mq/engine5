export IMAGE_NAME='hcangunduz/engine5'
export IMAGE_TAG='0.0.7-alpha'
export DOCKER_FILE="./dockerfile"
docker build --file ${DOCKER_FILE} -t ${IMAGE_NAME}:${IMAGE_TAG} .
docker push ${IMAGE_NAME}:${IMAGE_TAG}
