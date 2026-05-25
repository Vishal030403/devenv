module devenv-gamma

go 1.24

require (
	github.com/spf13/cobra v1.8.0
	pipeline-cli/scaffolding_engine v0.0.0
)

replace pipeline-cli/scaffolding_engine => "../team delta"

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)
