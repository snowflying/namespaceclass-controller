pipeline {
    agent {
       docker {
           image 'alizeedocker/tool:v1'
           label 'vm03-dockeragent'
           args '-v /var/run/docker.sock:/var/run/docker.sock'
       }
    }

    environment {
        // GIT_SSH_COMMAND = "ssh -o StrictHostKeyChecking=no" # disable 'Host key verification' whiling using SSH to clone code from github.
        GO111MODULE = "on"
        GOPROXY = "https://proxy.golang.org"
        DOCKER_REGISTRY = 'alizeedocker'
        IMAGE_NAME = 'nsc-controller:v1'
    }

    stages {
        stage('Checkout') {
            steps {
                git branch: 'main', url: 'https://github.com/snowflying/namespaceclass-controller.git', credentialsId: 'pullcodefromgithub'
            }
        }

        stage('Build') {
            steps {
                sh 'make'
            }
        }

        stage('Unit Test') {
            steps {
                sh 'make test'
            }
        }
        
        stage('Lint') {
            steps {
                sh '''
                    # go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
                    # export PATH=$PATH:$(go env GOPATH)/bin
                    make lint
                '''
            }
        }

        stage('Docker Build & Push') {
            steps {
                // build the image
                sh '''
                    # install docker cli
                    # apt-get update -y && apt-get install docker.io -y

                    # build the docker image
                    # docker build -t ${DOCKER_REGISTRY}/${IMAGE_NAME} .
                '''

                // login the Docker Hub
                withCredentials([string(credentialsId: 'dockerhub-PAT-token', variable: 'DOCKER_TOKEN')]) {
                    sh """
                        echo "$DOCKER_TOKEN" | docker login -u "alizeedocker" --password-stdin
                    """
                }

                // push the image into docker hub
                sh '''
                    docker push ${DOCKER_REGISTRY}/${IMAGE_NAME}
                '''
            }
        }

        stage('Deploy to Dev K8S cluster') {
            steps {
                sh '''
                    # install kubectl cli
                    # curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
                    # chmod +x kubectl
                    # mv kubectl /usr/local/bin/
                '''
                withCredentials([file(credentialsId: 'dev-kubeconfig', variable: 'KUBECONFIG')]) {
                    sh 'kubectl apply -f deployment/nsc-controller.yaml'
                }
            }
        }

        stage('Clean') {
            steps {
                sh 'make clean'
            }
        }
    }
}
