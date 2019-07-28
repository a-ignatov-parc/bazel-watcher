package local_repository

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	bazel "github.com/bazelbuild/bazel-integration-testing/go"
	"github.com/bazelbuild/bazel-watcher/e2e"
)

func must(t *testing.T, e error) {
	if e != nil {
		t.Fatalf("Error: %s", e)
		t.Logf("Stack trace:\n%s", string(debug.Stack()))
	}
}

func TestLocalRepositoryRunWithModifiedFile(t *testing.T) {
	secondary, err := bazel.New()
	if err != nil {
		t.Fatal(err)
	}

	must(t, secondary.ScratchFile("WORKSPACE", `workspace(name = "secondary")`))
	must(t, secondary.ScratchFile("BUILD", `
sh_library(
	name = "lib",
	data = ["lib.sh"],
	visibility = ["//visibility:public"],
)

sh_binary(
	name = "workspace",
	srcs = ["workspace.sh"],
)
`))
	must(t, secondary.ScratchFileWithMode("lib.sh", `
function say_hello {
	printf "hello!"
}
`, 0777))
	must(t, secondary.ScratchFileWithMode("workspace.sh", `
echo $BUILD_WORKSPACE_DIRECTORY
`, 0777))

	_, stdout, _ := secondary.RunBazel([]string{"run", "//:workspace"})
	secondaryWorkspacePath := strings.TrimSpace(stdout)

	main, err := bazel.New()
	if err != nil {
		t.Fatal(err)
	}
	must(t, main.ScratchFile("WORKSPACE", fmt.Sprintf(`
workspace(name = "main")

local_repository(
    name = "secondary",
    path = "%s",
)
`, secondaryWorkspacePath)))
	must(t, main.ScratchFile("BUILD", `
sh_binary(
	name = "test",
	srcs = ["test.sh"],
	deps = [
		"@secondary//:lib",
	],
)
`))
	must(t, main.ScratchFileWithMode("test.sh", `
#!/bin/bash
source ../secondary/lib.sh
say_hello
`, 0777))

	ibazel := e2e.NewIBazelTester(t, main)
	ibazel.Run([]string{}, "//:test")
	defer ibazel.Kill()

	ibazel.ExpectOutput("hello!")

	// File operations in `TestingBazel` doesn't respect their own `tmpDir` all instances
	// will work in the same directory so in order to update files in `secondary` workspace
	// we need to change directory manualy.
	os.Chdir(secondaryWorkspacePath)

	must(t, secondary.ScratchFileWithMode("lib.sh", `
function say_hello {
	printf "hello2!"
}
`, 0777))
	ibazel.ExpectOutput("hello2!")
}
