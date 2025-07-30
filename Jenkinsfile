pipeline {
    agent { label 'pr' }

       options {
        timestamps()
        timeout(time: 1, unit: 'HOURS')
        disableConcurrentBuilds(abortPrevious: true)
    }

    stages {
         stage('Validate commit') {
            steps {
                script {
                    def CHANGE_REPO = sh(script: 'basename -s .git `git config --get remote.origin.url`', returnStdout: true).trim()
                    build job: '/Utils/Validate-Git-Commit', parameters: [
                        string(name: 'Repo', value: "${CHANGE_REPO}"),
                        string(name: 'Branch', value: "${env.CHANGE_BRANCH}"),
                        string(name: 'Commit', value: "${GIT_COMMIT}")
                    ]
                }
            }
        }

        stage('Static analysis') {
            steps {
                sh '''
                go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6 && \
                golangci-lint run ./...
                '''
            }
        }

        stage('Run Tests') {
            steps {
                sh 'go test ./...'
            }
        }

        // TODO: Enable once code coverage is configured
        // stage('Upload test coverage') {
        //     environment {
        //         CODECOV_TOKEN = credentials('codecov-uploader-0xsoniclabs-global')
        //     }
        //     steps {
        //         sh("codecov upload-process -r 0xsoniclabs/sonic -f ./build/coverage.cov -t $CODECOV_TOKEN")
        //     }
        // }

    }

}