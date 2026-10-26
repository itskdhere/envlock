package exec

import (
	"os"
	"os/exec"
	"os/signal"
	"unsafe"

	"golang.org/x/sys/windows"
)

func run(cmd *exec.Cmd) error {
	disarm := killTreeIfKilled()

	// Ctrl+C reaches every process on the console; swallow it and let the child decide when to exit.
	signal.Notify(make(chan os.Signal, 1), os.Interrupt)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if cmd.ProcessState == nil {
		return err
	}
	disarm()
	os.Exit(cmd.ProcessState.ExitCode())
	return nil
}

// killTreeIfKilled makes Windows kill the command if envlock dies abruptly.
// disarm before a normal exit so the command's background processes survive.
func killTreeIfKilled() (disarm func()) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return func() {}
	}
	setLimits := func(flags uint32) error {
		var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		info.BasicLimitInformation.LimitFlags = flags
		_, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
		return err
	}
	if setLimits(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE) != nil ||
		windows.AssignProcessToJobObject(job, windows.CurrentProcess()) != nil {
		windows.CloseHandle(job)
		return func() {}
	}
	return func() { _ = setLimits(0) }
}
