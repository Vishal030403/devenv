package docker

# OPA Policies for Dockerfile Validation
# Team Gamma - Industrial CI/CD Pipeline

##############################################################################
# DENY RULES - Dockerfile best practices
##############################################################################

# Deny use of 'latest' tag in FROM instructions
deny[msg] {
    input[i].Cmd == "from"
    val := input[i].Value
    contains(val[i], ":latest")
    msg := sprintf("Line %d: Base image uses 'latest' tag which is not allowed", [i])
}

deny[msg] {
    input[i].Cmd == "from"
    val := input[i].Value
    image := val[0]
    not contains(image, ":")
    not contains(image, "@sha256:")
    msg := sprintf("Line %d: Base image '%s' has no tag specified", [i, image])
}

# Deny running as root
deny[msg] {
    input[i].Cmd == "user"
    val := input[i].Value
    val[0] == "root"
    msg := sprintf("Line %d: Container runs as root user", [i])
}

# Deny use of ADD when COPY should be used
deny[msg] {
    input[i].Cmd == "add"
    val := input[i].Value
    not is_url(val[0])
    not is_tar(val[0])
    msg := sprintf("Line %d: Use COPY instead of ADD for local files", [i])
}

# Deny missing HEALTHCHECK
deny[msg] {
    count([x | input[x].Cmd == "healthcheck"]) == 0
    msg := "Dockerfile is missing HEALTHCHECK instruction"
}

# Deny apt-get without cleanup
deny[msg] {
    input[i].Cmd == "run"
    val := input[i].Value[0]
    contains(val, "apt-get install")
    not contains(val, "rm -rf /var/lib/apt/lists/*")
    msg := sprintf("Line %d: apt-get install should include cleanup", [i])
}

# Deny curl/wget without cleanup
deny[msg] {
    input[i].Cmd == "run"
    val := input[i].Value[0]
    contains(val, "curl")
    not contains(val, "&&")
    msg := sprintf("Line %d: Temporary tools should be installed and removed in same layer", [i])
}

##############################################################################
# WARN RULES - Best practices
##############################################################################

warn[msg] {
    count([x | input[x].Cmd == "label"]) == 0
    msg := "Dockerfile has no LABEL instructions for metadata"
}

warn[msg] {
    count([x | input[x].Cmd == "expose"]) == 0
    msg := "Dockerfile does not EXPOSE any ports"
}

warn[msg] {
    input[i].Cmd == "run"
    val := input[i].Value[0]
    contains(val, "&&")
    count(split(val, "&&")) > 5
    msg := sprintf("Line %d: RUN command chains too many operations (>5)", [i])
}

warn[msg] {
    input[i].Cmd == "from"
    val := input[i].Value[0]
    not contains(val, "alpine")
    not contains(val, "slim")
    not contains(val, "scratch")
    msg := sprintf("Line %d: Consider using Alpine, slim, or distroless base images", [i])
}

warn[msg] {
    count([x | input[x].Cmd == "workdir"]) == 0
    msg := "Dockerfile does not set WORKDIR"
}

warn[msg] {
    input[i].Cmd == "copy"
    val := input[i].Value
    val[_] == "."
    msg := sprintf("Line %d: Copying entire context (.) may include unwanted files", [i])
}

##############################################################################
# HELPER FUNCTIONS
##############################################################################

is_url(str) {
    startswith(str, "http://")
}

is_url(str) {
    startswith(str, "https://")
}

is_tar(str) {
    endswith(str, ".tar")
}

is_tar(str) {
    endswith(str, ".tar.gz")
}

is_tar(str) {
    endswith(str, ".tgz")
}

contains(str, substr) {
    contains(str, substr)
}

startswith(str, prefix) {
    startswith(str, prefix)
}

endswith(str, suffix) {
    endswith(str, suffix)
}

split(str, delim) = output {
    output := split(str, delim)
}

count(arr) = output {
    output := count(arr)
}
