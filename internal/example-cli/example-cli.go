// minimal example CLI used for binary size checking

package main

import (
	"github.com/zachmann/cli/v2"
)

func main() {
	(&cli.App{}).Run([]string{})
}
