#!/usr/bin/env groovy

/**
 * Validation Utilities
 * Team Gamma - Industrial CI/CD Pipeline
 * 
 * Validators for different project types and configurations
 */

/**
 * Validate Node.js project structure
 * @param projectPath Project root path
 * @return Validation result
 */
def validateNodeJsProject(String projectPath = '.') {
    def errors = []
    def warnings = []
    
    // Check package.json
    if (!fileExists("${projectPath}/package.json")) {
        errors << "Missing package.json"
    } else {
        def packageJson = readJSON file: "${projectPath}/package.json"
        
        if (!packageJson.name) {
            errors << "package.json missing 'name' field"
        }
        
        if (!packageJson.version) {
            warnings << "package.json missing 'version' field"
        }
        
        if (!packageJson.scripts) {
            warnings << "package.json has no scripts defined"
        } else {
            if (!packageJson.scripts.start) {
                warnings << "package.json missing 'start' script"
            }
            if (!packageJson.scripts.test) {
                warnings << "package.json missing 'test' script"
            }
        }
        
        // Check for security vulnerabilities
        if (packageJson.dependencies) {
            def hasDeprecated = checkDeprecatedPackages(packageJson.dependencies)
            if (hasDeprecated) {
                warnings << "Found deprecated npm packages"
            }
        }
    }
    
    // Check for .nvmrc or .node-version
    if (!fileExists("${projectPath}/.nvmrc") && !fileExists("${projectPath}/.node-version")) {
        warnings << "Missing .nvmrc or .node-version for Node version pinning"
    }
    
    // Check for package-lock.json or yarn.lock
    if (!fileExists("${projectPath}/package-lock.json") && !fileExists("${projectPath}/yarn.lock")) {
        warnings << "Missing lock file (package-lock.json or yarn.lock)"
    }
    
    // Check for ESLint configuration
    if (!fileExists("${projectPath}/.eslintrc") && 
        !fileExists("${projectPath}/.eslintrc.js") && 
        !fileExists("${projectPath}/.eslintrc.json")) {
        warnings << "Missing ESLint configuration"
    }
    
    // Check for Prettier configuration
    if (!fileExists("${projectPath}/.prettierrc") && 
        !fileExists("${projectPath}/.prettierrc.js") && 
        !fileExists("${projectPath}/.prettierrc.json")) {
        warnings << "Missing Prettier configuration"
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Validate Python project structure
 * @param projectPath Project root path
 * @return Validation result
 */
def validatePythonProject(String projectPath = '.') {
    def errors = []
    def warnings = []
    
    // Check requirements.txt
    if (!fileExists("${projectPath}/requirements.txt")) {
        errors << "Missing requirements.txt"
    } else {
        def requirements = readFile("${projectPath}/requirements.txt")
        
        // Check for pinned versions
        def unpinnedDeps = requirements.readLines().findAll { line ->
            line && !line.startsWith('#') && !line.contains('==') && !line.contains('>=')
        }
        
        if (!unpinnedDeps.isEmpty()) {
            warnings << "Found unpinned dependencies in requirements.txt"
        }
    }
    
    // Check for setup.py or pyproject.toml
    if (!fileExists("${projectPath}/setup.py") && !fileExists("${projectPath}/pyproject.toml")) {
        warnings << "Missing setup.py or pyproject.toml for package metadata"
    }
    
    // Check for .python-version
    if (!fileExists("${projectPath}/.python-version")) {
        warnings << "Missing .python-version for Python version pinning"
    }
    
    // Check for virtual environment configuration
    if (!fileExists("${projectPath}/.venv") && !fileExists("${projectPath}/venv")) {
        warnings << "No virtual environment detected"
    }
    
    // Check for test configuration
    if (!fileExists("${projectPath}/pytest.ini") && 
        !fileExists("${projectPath}/setup.cfg") &&
        !fileExists("${projectPath}/pyproject.toml")) {
        warnings << "Missing pytest configuration"
    }
    
    // Check for linting configuration
    if (!fileExists("${projectPath}/.pylintrc") && 
        !fileExists("${projectPath}/pylintrc")) {
        warnings << "Missing pylint configuration"
    }
    
    // Check for type checking
    if (!fileExists("${projectPath}/mypy.ini") && 
        !fileExists("${projectPath}/.mypy.ini")) {
        warnings << "Missing mypy configuration for type checking"
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Validate Java project structure
 * @param projectPath Project root path
 * @return Validation result
 */
def validateJavaProject(String projectPath = '.') {
    def errors = []
    def warnings = []
    
    // Check for Maven or Gradle
    def hasMaven = fileExists("${projectPath}/pom.xml")
    def hasGradle = fileExists("${projectPath}/build.gradle") || fileExists("${projectPath}/build.gradle.kts")
    
    if (!hasMaven && !hasGradle) {
        errors << "Missing pom.xml or build.gradle"
    }
    
    if (hasMaven) {
        def pom = readFile("${projectPath}/pom.xml")
        
        if (!pom.contains('<groupId>')) {
            errors << "pom.xml missing groupId"
        }
        
        if (!pom.contains('<artifactId>')) {
            errors << "pom.xml missing artifactId"
        }
        
        if (!pom.contains('<version>')) {
            warnings << "pom.xml missing version"
        }
        
        // Check for Maven wrapper
        if (!fileExists("${projectPath}/mvnw")) {
            warnings << "Missing Maven wrapper (mvnw)"
        }
    }
    
    if (hasGradle) {
        // Check for Gradle wrapper
        if (!fileExists("${projectPath}/gradlew")) {
            warnings << "Missing Gradle wrapper (gradlew)"
        }
        
        // Check for gradle.properties
        if (!fileExists("${projectPath}/gradle.properties")) {
            warnings << "Missing gradle.properties"
        }
    }
    
    // Check for source directory structure
    if (!fileExists("${projectPath}/src/main/java")) {
        warnings << "Missing standard src/main/java directory"
    }
    
    if (!fileExists("${projectPath}/src/test/java")) {
        warnings << "Missing src/test/java directory for tests"
    }
    
    // Check for Checkstyle configuration
    if (!fileExists("${projectPath}/checkstyle.xml") && 
        !fileExists("${projectPath}/config/checkstyle/checkstyle.xml")) {
        warnings << "Missing Checkstyle configuration"
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Validate Go project structure
 * @param projectPath Project root path
 * @return Validation result
 */
def validateGoProject(String projectPath = '.') {
    def errors = []
    def warnings = []
    
    // Check for go.mod
    if (!fileExists("${projectPath}/go.mod")) {
        errors << "Missing go.mod"
    } else {
        def goMod = readFile("${projectPath}/go.mod")
        
        if (!goMod.contains('module ')) {
            errors << "go.mod missing module declaration"
        }
        
        if (!goMod.contains('go ')) {
            warnings << "go.mod missing Go version"
        }
    }
    
    // Check for go.sum
    if (!fileExists("${projectPath}/go.sum")) {
        warnings << "Missing go.sum for dependency verification"
    }
    
    // Check for main.go or cmd directory
    if (!fileExists("${projectPath}/main.go") && !fileExists("${projectPath}/cmd")) {
        warnings << "Missing main.go or cmd directory"
    }
    
    // Check for standard Go project layout
    def standardDirs = ['pkg', 'internal', 'api', 'web']
    def hasStandardLayout = standardDirs.any { dir -> fileExists("${projectPath}/${dir}") }
    
    if (!hasStandardLayout) {
        warnings << "Project doesn't follow standard Go project layout"
    }
    
    // Check for tests
    def hasTests = sh(
        script: "find ${projectPath} -name '*_test.go' | head -1",
        returnStatus: true
    ) == 0
    
    if (!hasTests) {
        warnings << "No test files found (*_test.go)"
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Validate Dockerfile
 * @param dockerfilePath Path to Dockerfile
 * @return Validation result
 */
def validateDockerfile(String dockerfilePath = './Dockerfile') {
    def errors = []
    def warnings = []
    
    if (!fileExists(dockerfilePath)) {
        errors << "Dockerfile not found at ${dockerfilePath}"
        return [valid: false, errors: errors, warnings: warnings]
    }
    
    def dockerfile = readFile(dockerfilePath)
    def lines = dockerfile.readLines()
    
    // Check for FROM instruction
    if (!lines.any { it.trim().toUpperCase().startsWith('FROM ') }) {
        errors << "Dockerfile missing FROM instruction"
    }
    
    // Check for USER instruction (should not run as root)
    def hasUser = lines.any { it.trim().toUpperCase().startsWith('USER ') }
    if (!hasUser) {
        warnings << "Dockerfile doesn't specify USER (will run as root)"
    }
    
    // Check for EXPOSE instruction
    def hasExpose = lines.any { it.trim().toUpperCase().startsWith('EXPOSE ') }
    if (!hasExpose) {
        warnings << "Dockerfile doesn't EXPOSE any ports"
    }
    
    // Check for HEALTHCHECK
    def hasHealthcheck = lines.any { it.trim().toUpperCase().startsWith('HEALTHCHECK ') }
    if (!hasHealthcheck) {
        warnings << "Dockerfile missing HEALTHCHECK instruction"
    }
    
    // Check for latest tag in FROM
    def fromLines = lines.findAll { it.trim().toUpperCase().startsWith('FROM ') }
    fromLines.each { line ->
        if (line.contains(':latest') || (!line.contains(':') && !line.contains('@sha256'))) {
            warnings << "FROM instruction uses 'latest' tag or no tag: ${line}"
        }
    }
    
    // Check for WORKDIR
    def hasWorkdir = lines.any { it.trim().toUpperCase().startsWith('WORKDIR ') }
    if (!hasWorkdir) {
        warnings << "Dockerfile doesn't set WORKDIR"
    }
    
    // Check for LABEL instructions
    def hasLabels = lines.any { it.trim().toUpperCase().startsWith('LABEL ') }
    if (!hasLabels) {
        warnings << "Dockerfile has no LABEL instructions for metadata"
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Validate Kubernetes manifests
 * @param manifestsPath Path to K8s manifests directory
 * @return Validation result
 */
def validateKubernetesManifests(String manifestsPath = './k8s') {
    def errors = []
    def warnings = []
    
    if (!fileExists(manifestsPath)) {
        errors << "Kubernetes manifests directory not found: ${manifestsPath}"
        return [valid: false, errors: errors, warnings: warnings]
    }
    
    // Check for required manifests
    def requiredManifests = ['deployment.yml', 'service.yml']
    requiredManifests.each { manifest ->
        if (!fileExists("${manifestsPath}/base/${manifest}") && 
            !fileExists("${manifestsPath}/${manifest}")) {
            warnings << "Missing recommended manifest: ${manifest}"
        }
    }
    
    // Check for kustomization.yaml
    if (!fileExists("${manifestsPath}/base/kustomization.yaml") &&
        !fileExists("${manifestsPath}/base/kustomization.yml") &&
        !fileExists("${manifestsPath}/kustomization.yaml")) {
        warnings << "Missing kustomization.yaml for Kustomize"
    }
    
    // Validate YAML syntax
    def yamlFiles = sh(
        script: "find ${manifestsPath} -name '*.yml' -o -name '*.yaml'",
        returnStdout: true
    ).trim().split('\n')
    
    yamlFiles.each { file ->
        try {
            readYaml file: file
        } catch (Exception e) {
            errors << "Invalid YAML in ${file}: ${e.message}"
        }
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Validate environment configuration
 * @param envVars Required environment variables
 * @return Validation result
 */
def validateEnvironment(List<String> envVars) {
    def errors = []
    def warnings = []
    
    envVars.each { varName ->
        if (!env.getProperty(varName)) {
            errors << "Required environment variable not set: ${varName}"
        }
    }
    
    // Check Docker
    def dockerVersion = sh(script: 'docker --version', returnStatus: true)
    if (dockerVersion != 0) {
        errors << "Docker not available"
    }
    
    // Check kubectl
    def kubectlVersion = sh(script: 'kubectl version --client', returnStatus: true)
    if (kubectlVersion != 0) {
        errors << "kubectl not available"
    }
    
    // Check git
    def gitVersion = sh(script: 'git --version', returnStatus: true)
    if (gitVersion != 0) {
        errors << "Git not available"
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Validate security scan results
 * @param scanResults Scan results object
 * @param criticalThreshold Maximum allowed critical vulnerabilities
 * @return Validation result
 */
def validateSecurityScan(Map scanResults, int criticalThreshold = 0) {
    def errors = []
    def warnings = []
    
    if (scanResults.critical > criticalThreshold) {
        errors << "Found ${scanResults.critical} critical vulnerabilities (threshold: ${criticalThreshold})"
    }
    
    if (scanResults.high > 10) {
        warnings << "Found ${scanResults.high} high severity vulnerabilities"
    }
    
    if (scanResults.secrets && scanResults.secrets > 0) {
        errors << "Found ${scanResults.secrets} potential secrets in code"
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Validate test results
 * @param testResults Test results object
 * @param minCoverage Minimum required code coverage percentage
 * @return Validation result
 */
def validateTestResults(Map testResults, float minCoverage = 80.0) {
    def errors = []
    def warnings = []
    
    if (testResults.failed > 0) {
        errors << "Found ${testResults.failed} failed tests"
    }
    
    if (testResults.coverage < minCoverage) {
        warnings << "Code coverage ${testResults.coverage}% is below minimum ${minCoverage}%"
    }
    
    if (testResults.skipped > 0) {
        warnings << "Found ${testResults.skipped} skipped tests"
    }
    
    return [
        valid: errors.isEmpty(),
        errors: errors,
        warnings: warnings
    ]
}

/**
 * Check for deprecated npm packages
 * @param dependencies Dependencies object from package.json
 * @return Boolean indicating if deprecated packages found
 */
def checkDeprecatedPackages(Map dependencies) {
    // This is a simplified check - in production, you'd query npm registry
    def knownDeprecated = ['request', 'gulp-util', 'node-uuid']
    
    return dependencies.keySet().any { dep ->
        knownDeprecated.contains(dep)
    }
}

/**
 * Print validation results
 * @param results Validation results
 * @param context Context description
 */
def printValidationResults(Map results, String context = 'Validation') {
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║ ${context.center(61)} ║"
    echo "╠═══════════════════════════════════════════════════════════════╣"
    
    if (results.valid) {
        echo "║ ✓ All checks passed ${' ' * 41}║"
    } else {
        echo "║ ✗ Validation failed ${' ' * 41}║"
    }
    
    if (!results.errors.isEmpty()) {
        echo "║                                                               ║"
        echo "║ Errors:                                                       ║"
        results.errors.each { error ->
            def truncated = error.length() > 57 ? error.substring(0, 54) + '...' : error
            echo "║   • ${truncated.padRight(57)}║"
        }
    }
    
    if (!results.warnings.isEmpty()) {
        echo "║                                                               ║"
        echo "║ Warnings:                                                     ║"
        results.warnings.each { warning ->
            def truncated = warning.length() > 57 ? warning.substring(0, 54) + '...' : warning
            echo "║   • ${truncated.padRight(57)}║"
        }
    }
    
    echo "╚═══════════════════════════════════════════════════════════════╝"
}

// Export functions
return this
