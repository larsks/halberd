package version

import "fmt"

var (
	BuildVersion string = "development"
	BuildRef     string = ""
	BuildDate    string = ""
)

func ShowVersion() {
	fmt.Printf("Version: %s\n", BuildVersion)
	fmt.Printf("Build ref: %s\n", BuildRef)
	fmt.Printf("Build date: %s\n", BuildDate)
}
