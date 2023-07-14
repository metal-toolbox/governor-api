//go:generate sqlboiler crdb --add-soft-deletes
package main

import "github.com/metal-toolbox/governor-api/cmd"

func main() {
	cmd.Execute()
}
