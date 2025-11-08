package main

import (
	_ "embed"

	"github.com/mbreese/mcpfurl/cmd"
)

//go:embed LICENSE
var licenseText string

func main() {
	cmd.SetLicenseText(licenseText)
	cmd.Execute()
}
