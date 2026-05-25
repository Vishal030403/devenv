#!/usr/bin/env groovy

/**
 * Pipeline Helper Library
 * Team Gamma - Industrial CI/CD Pipeline
 * 
 * Reusable functions for Jenkins pipelines
 */

/**
 * Retry with exponential backoff
 * @param maxAttempts Maximum number of retry attempts
 * @param initialDelay Initial delay in seconds
 * @param closure The code block to execute
 */
def retryWithBackoff(int maxAttempts = 3, int initialDelay = 5, Closure closure) {
    def attempt = 1
    def delay = initialDelay
    
    while (attempt <= maxAttempts) {
        try {
            closure()
            return
        } catch (Exception e) {
            if (attempt == maxAttempts) {
                throw e
            }
            
            echo "Attempt ${attempt} failed: ${e.message}"
            echo "Retrying in ${delay} seconds..."
            sleep(delay)
            
            delay = delay * 2  // Exponential backoff
            attempt++
        }
    }
}

/**
 * Execute command with timeout and retry
 * @param command Shell command to execute
 * @param timeoutMinutes Timeout in minutes
 * @param retries Number of retries
 * @return Command output
 */
def executeWithRetry(String command, int timeoutMinutes = 5, int retries = 3) {
    return timeout(time: timeoutMinutes, unit: 'MINUTES') {
        retry(retries) {
            sh(script: command, returnStdout: true).trim()
        }
    }
}

/**
 * Check if file exists in workspace
 * @param filePath Path to file
 * @return Boolean
 */
def fileExists(String filePath) {
    return fileExists(filePath)
}

/**
 * Read YAML file
 * @param filePath Path to YAML file
 * @return Parsed YAML object
 */
def readYamlFile(String filePath) {
    if (!fileExists(filePath)) {
        error "File not found: ${filePath}"
    }
    return readYaml(file: filePath)
}

/**
 * Write YAML file
 * @param filePath Path to write file
 * @param data Data to write
 */
def writeYamlFile(String filePath, Object data) {
    writeYaml(file: filePath, data: data)
}

/**
 * Read JSON file
 * @param filePath Path to JSON file
 * @return Parsed JSON object
 */
def readJsonFile(String filePath) {
    if (!fileExists(filePath)) {
        error "File not found: ${filePath}"
    }
    return readJSON(file: filePath)
}

/**
 * Write JSON file
 * @param filePath Path to write file
 * @param data Data to write
 */
def writeJsonFile(String filePath, Object data) {
    writeJSON(file: filePath, json: data)
}

/**
 * Send Slack notification
 * @param channel Slack channel
 * @param message Message to send
 * @param color Message color (good, warning, danger)
 */
def notifySlack(String channel, String message, String color = 'good') {
    try {
        slackSend(
            channel: channel,
            color: color,
            message: message
        )
    } catch (Exception e) {
        echo "Failed to send Slack notification: ${e.message}"
    }
}

/**
 * Send email notification
 * @param recipients Email recipients
 * @param subject Email subject
 * @param body Email body
 */
def notifyEmail(String recipients, String subject, String body) {
    try {
        emailext(
            to: recipients,
            subject: subject,
            body: body,
            mimeType: 'text/html'
        )
    } catch (Exception e) {
        echo "Failed to send email notification: ${e.message}"
    }
}

/**
 * Get Git information
 * @return Map containing Git information
 */
def getGitInfo() {
    return [
        commit: sh(script: 'git rev-parse HEAD', returnStdout: true).trim(),
        shortCommit: sh(script: 'git rev-parse --short HEAD', returnStdout: true).trim(),
        branch: sh(script: 'git rev-parse --abbrev-ref HEAD', returnStdout: true).trim(),
        author: sh(script: 'git log -1 --pretty=format:"%an"', returnStdout: true).trim(),
        email: sh(script: 'git log -1 --pretty=format:"%ae"', returnStdout: true).trim(),
        message: sh(script: 'git log -1 --pretty=format:"%s"', returnStdout: true).trim(),
        timestamp: sh(script: 'git log -1 --pretty=format:"%ci"', returnStdout: true).trim()
    ]
}

/**
 * Calculate build duration
 * @return Duration string
 */
def getBuildDuration() {
    def duration = currentBuild.duration
    def seconds = duration / 1000
    def minutes = seconds / 60
    
    if (minutes < 1) {
        return "${seconds.intValue()}s"
    } else {
        def remainingSeconds = seconds % 60
        return "${minutes.intValue()}m ${remainingSeconds.intValue()}s"
    }
}

/**
 * Get current timestamp
 * @param format Date format (default: yyyy-MM-dd HH:mm:ss)
 * @return Formatted timestamp
 */
def getTimestamp(String format = 'yyyy-MM-dd HH:mm:ss') {
    return sh(script: "date +'${format}'", returnStdout: true).trim()
}

/**
 * Check if Docker image exists in registry
 * @param registry Registry host
 * @param image Image name
 * @param tag Image tag
 * @return Boolean
 */
def dockerImageExists(String registry, String image, String tag) {
    def exitCode = sh(
        script: "docker manifest inspect ${registry}/${image}:${tag} > /dev/null 2>&1",
        returnStatus: true
    )
    return exitCode == 0
}

/**
 * Get Docker image digest
 * @param image Full image name with tag
 * @return Image digest
 */
def getDockerImageDigest(String image) {
    return sh(
        script: "docker inspect --format='{{index .RepoDigests 0}}' ${image}",
        returnStdout: true
    ).trim()
}

/**
 * Clean Docker resources
 * @param all Clean all resources including images
 */
def cleanDocker(boolean all = false) {
    sh 'docker system prune -f'
    if (all) {
        sh 'docker image prune -a -f'
    }
}

/**
 * Check if Kubernetes resource exists
 * @param resourceType Resource type (deployment, service, etc.)
 * @param resourceName Resource name
 * @param namespace Namespace
 * @return Boolean
 */
def k8sResourceExists(String resourceType, String resourceName, String namespace) {
    def exitCode = sh(
        script: "kubectl get ${resourceType} ${resourceName} -n ${namespace} > /dev/null 2>&1",
        returnStatus: true
    )
    return exitCode == 0
}

/**
 * Get Kubernetes pod status
 * @param podName Pod name
 * @param namespace Namespace
 * @return Pod status
 */
def getK8sPodStatus(String podName, String namespace) {
    return sh(
        script: "kubectl get pod ${podName} -n ${namespace} -o jsonpath='{.status.phase}'",
        returnStdout: true
    ).trim()
}

/**
 * Wait for Kubernetes pods to be ready
 * @param selector Label selector
 * @param namespace Namespace
 * @param timeout Timeout in minutes
 */
def waitForK8sPodsReady(String selector, String namespace, int timeout = 5) {
    timeout(time: timeout, unit: 'MINUTES') {
        sh """
            kubectl wait --for=condition=ready pod \
                -l ${selector} \
                -n ${namespace} \
                --timeout=${timeout}m
        """
    }
}

/**
 * Get Kubernetes deployment replicas
 * @param deploymentName Deployment name
 * @param namespace Namespace
 * @return Map with desired, current, and available replicas
 */
def getK8sDeploymentReplicas(String deploymentName, String namespace) {
    def desired = sh(
        script: "kubectl get deployment ${deploymentName} -n ${namespace} -o jsonpath='{.spec.replicas}'",
        returnStdout: true
    ).trim().toInteger()
    
    def current = sh(
        script: "kubectl get deployment ${deploymentName} -n ${namespace} -o jsonpath='{.status.replicas}'",
        returnStdout: true
    ).trim().toInteger()
    
    def available = sh(
        script: "kubectl get deployment ${deploymentName} -n ${namespace} -o jsonpath='{.status.availableReplicas}'",
        returnStdout: true
    ).trim().toInteger()
    
    return [
        desired: desired,
        current: current,
        available: available
    ]
}

/**
 * Scale Kubernetes deployment
 * @param deploymentName Deployment name
 * @param namespace Namespace
 * @param replicas Number of replicas
 */
def scaleK8sDeployment(String deploymentName, String namespace, int replicas) {
    sh "kubectl scale deployment ${deploymentName} -n ${namespace} --replicas=${replicas}"
}

/**
 * Rollback Kubernetes deployment
 * @param deploymentName Deployment name
 * @param namespace Namespace
 */
def rollbackK8sDeployment(String deploymentName, String namespace) {
    sh "kubectl rollout undo deployment/${deploymentName} -n ${namespace}"
}

/**
 * Parse memory string to bytes
 * @param memory Memory string (e.g., "1Gi", "512Mi")
 * @return Memory in bytes
 */
def parseMemory(String memory) {
    if (memory.endsWith('Gi')) {
        return memory.replace('Gi', '').toFloat() * 1024 * 1024 * 1024
    } else if (memory.endsWith('Mi')) {
        return memory.replace('Mi', '').toFloat() * 1024 * 1024
    } else if (memory.endsWith('Ki')) {
        return memory.replace('Ki', '').toFloat() * 1024
    } else {
        return memory.toFloat()
    }
}

/**
 * Parse CPU string to millicores
 * @param cpu CPU string (e.g., "1", "500m")
 * @return CPU in millicores
 */
def parseCPU(String cpu) {
    if (cpu.endsWith('m')) {
        return cpu.replace('m', '').toFloat()
    } else {
        return cpu.toFloat() * 1000
    }
}

/**
 * Format bytes to human-readable string
 * @param bytes Number of bytes
 * @return Formatted string
 */
def formatBytes(long bytes) {
    def units = ['B', 'KB', 'MB', 'GB', 'TB']
    def unitIndex = 0
    def size = bytes.toFloat()
    
    while (size >= 1024 && unitIndex < units.size() - 1) {
        size /= 1024
        unitIndex++
    }
    
    return String.format("%.2f %s", size, units[unitIndex])
}

/**
 * Generate unique build ID
 * @return Unique build ID
 */
def generateBuildId() {
    def timestamp = System.currentTimeMillis()
    def random = new Random().nextInt(10000)
    return "${timestamp}-${random}"
}

/**
 * Validate semver version string
 * @param version Version string
 * @return Boolean
 */
def isValidSemver(String version) {
    return version ==~ /^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$/
}

/**
 * Compare semver versions
 * @param version1 First version
 * @param version2 Second version
 * @return -1 if version1 < version2, 0 if equal, 1 if version1 > version2
 */
def compareSemver(String version1, String version2) {
    def v1Parts = version1.tokenize('.')
    def v2Parts = version2.tokenize('.')
    
    for (int i = 0; i < Math.min(v1Parts.size(), v2Parts.size()); i++) {
        def v1Part = v1Parts[i].toInteger()
        def v2Part = v2Parts[i].toInteger()
        
        if (v1Part < v2Part) return -1
        if (v1Part > v2Part) return 1
    }
    
    return 0
}

/**
 * Archive artifacts with compression
 * @param artifacts Artifact pattern
 * @param archiveName Archive name
 */
def archiveCompressed(String artifacts, String archiveName) {
    sh "tar -czf ${archiveName}.tar.gz ${artifacts}"
    archiveArtifacts artifacts: "${archiveName}.tar.gz"
}

/**
 * Generate HTML report
 * @param title Report title
 * @param data Report data
 * @param outputFile Output file path
 */
def generateHtmlReport(String title, Map data, String outputFile) {
    def html = """
<!DOCTYPE html>
<html>
<head>
    <title>${title}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        table { border-collapse: collapse; width: 100%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #4CAF50; color: white; }
        tr:nth-child(even) { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>${title}</h1>
    <table>
        <thead>
            <tr>
                <th>Property</th>
                <th>Value</th>
            </tr>
        </thead>
        <tbody>
"""
    
    data.each { key, value ->
        html += """
            <tr>
                <td>${key}</td>
                <td>${value}</td>
            </tr>
"""
    }
    
    html += """
        </tbody>
    </table>
</body>
</html>
"""
    
    writeFile file: outputFile, text: html
}

/**
 * Mask sensitive data in string
 * @param text Text to mask
 * @param patterns List of regex patterns to mask
 * @return Masked text
 */
def maskSensitiveData(String text, List<String> patterns) {
    def masked = text
    
    patterns.each { pattern ->
        masked = masked.replaceAll(pattern, '***MASKED***')
    }
    
    return masked
}

// Export functions
return this
