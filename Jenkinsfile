// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

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