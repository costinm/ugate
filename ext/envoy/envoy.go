package envoy

import (
	"os"
	"os/exec"
)

// Exec envoy with a custom bootstrap.
//

func Start(wd, bin string, args []string) {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = wd

}
