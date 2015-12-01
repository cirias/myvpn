package common

import (
	"github.com/golang/glog"
	"github.com/kardianos/osext"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	SCRIPTDIR        = "script"
	IFUPSCRIPTNAME   = "if-up.sh"
	IFUPCONFD        = "if-up.d"
	IFDOWNSCRIPTNAME = "if-down.sh"
	IFDOWNCONFD      = "if-down.d"
)

func getScriptDir() (scriptDir string, err error) {
	dir, err := osext.ExecutableFolder()
	scriptDir = filepath.Join(dir, SCRIPTDIR)
	return
}

func execScript(path string, args ...string) (err error) {
	cmd := exec.Command(path, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	return
}

func getWalker(args ...string) func(path string, info os.FileInfo, err error) error {
	walker := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		return execScript(path, args...)
	}

	return walker
}

func IfUp(args ...string) (err error) {
	scriptDir, err := getScriptDir()
	if err != nil {
		return
	}

	err = execScript(filepath.Join(scriptDir, IFUPSCRIPTNAME), args...)
	if err != nil {
		return
	}

	err = filepath.Walk(filepath.Join(scriptDir, IFUPCONFD), getWalker(args...))
	return
}

func IfDown(args ...string) (err error) {
	glog.Infoln("getScriptDir")
	scriptDir, err := getScriptDir()
	if err != nil {
		return
	}

	glog.Infoln("execScript")
	err = execScript(filepath.Join(scriptDir, IFDOWNSCRIPTNAME), args...)
	if err != nil {
		return
	}

	glog.Infoln("filepath.Walk")
	err = filepath.Walk(filepath.Join(scriptDir, IFDOWNCONFD), getWalker(args...))
	return
}
